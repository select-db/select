// Command agentprobe runs SQL through every dialect-level language feature and
// reports what each produced: lint, completion at a caret, the inspected tree,
// and the rights the permission check demands. It is what the dialect agents
// measure through: one case from the flags, or a JSONL batch with -batch.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/tokenanalyzer"
)

func main() {
	dialectName := flag.String("dialect", "postgresql", "postgresql, mysql or sqlite")
	sqlInput := flag.String("sql", "", "SQL to probe, or @file. A | marks the caret")
	metaPath := flag.String("meta", "", "core.Metadata JSON file; defaults to core.GetInspectTestMetadata()")
	showRaw := flag.Bool("raw", false, "also report what the analyzer returned")
	asJSON := flag.Bool("json", false, "print the result as JSON")
	batchPath := flag.String("batch", "", "JSONL file of cases, or - for stdin; writes one JSON result per line")
	completionLimit := flag.Int("completion-limit", 0, "keep only the first N suggestions, in the order offered; 0 keeps all")
	export := flag.Bool("export-cases", false, "print every case of the Go case tables as JSONL, one line per dialect, and stop")
	flag.Parse()

	if *export {
		if err := exportCases(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "agentprobe: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(options{
		dialect: *dialectName,
		sql:     *sqlInput,
		meta:    *metaPath,
		raw:     *showRaw,
		json:    *asJSON,
		batch:   *batchPath,
		limit:   *completionLimit,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "agentprobe: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	dialect, sql, meta, batch string
	raw, json                 bool
	limit                     int
}

func run(opts options) error {
	if opts.batch == "" && strings.TrimSpace(opts.sql) == "" {
		return fmt.Errorf("-sql or -batch is required")
	}
	meta, err := loadMetadata(opts.meta)
	if err != nil {
		return err
	}

	pythonPath, script, ok := tokenanalyzer.FindDevAnalyzer()
	if !ok {
		return fmt.Errorf("python venv not found; run `uv sync` in dialect/core/tokenanalyzer/python")
	}
	analyzer := tokenanalyzer.NewAnalyzer(pythonPath, script)
	defer analyzer.Close()
	p := prober{analyzer: analyzer, meta: meta, raw: opts.raw, completionLimit: opts.limit}

	if opts.batch != "" {
		return p.runBatch(opts.batch, os.Stdout)
	}

	sqlText, err := readArg(opts.sql)
	if err != nil {
		return fmt.Errorf("reading -sql: %w", err)
	}
	result := p.probe(Case{Dialect: opts.dialect, SQL: sqlText})
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	if opts.json {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	printResult(result, meta)
	return nil
}

// runBatch probes one case per input line. A case that cannot be probed is
// reported in its own result rather than ending the batch, so one bad line
// costs one result and not the run.
func (p prober) runBatch(path string, out io.Writer) error {
	input := io.Reader(os.Stdin)
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("reading -batch: %w", err)
		}
		defer func() { _ = file.Close() }()
		input = file
	}

	encoder := json.NewEncoder(out)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var probeCase Case
		var result Result
		if err := json.Unmarshal([]byte(line), &probeCase); err != nil {
			result = Result{Error: fmt.Sprintf("line %d: %v", lineNumber, err)}
		} else {
			result = p.probe(probeCase)
		}
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func loadMetadata(metaPath string) (core.Metadata, error) {
	if metaPath == "" {
		return core.GetInspectTestMetadata(), nil
	}
	rawJSON, err := os.ReadFile(metaPath)
	if err != nil {
		return core.Metadata{}, fmt.Errorf("reading -meta: %w", err)
	}
	var meta core.Metadata
	if err := json.Unmarshal(rawJSON, &meta); err != nil {
		return core.Metadata{}, fmt.Errorf("parsing -meta: %w", err)
	}
	// Every path that builds a catalog from a real database runs this, and
	// without it a column typed as a named enum has no values, so the probe
	// reports no enum completion and no enum diagnostic for a catalog that
	// would produce both.
	core.EnrichEnumValues(&meta)
	return meta, nil
}

// readArg returns the literal value, or the file contents when it starts with @.
func readArg(value string) (string, error) {
	path, isFile := strings.CutPrefix(value, "@")
	if !isFile {
		return value, nil
	}
	fileContents, err := os.ReadFile(path)
	return string(fileContents), err
}

// cutCaret removes the caret marker and reports where it was, as the 1-based
// line and 0-based column the dialect layer expects. Returns line 0 when there
// is no marker.
func cutCaret(sql string) (string, int, int) {
	markerIndex := caretMarkerIndex(sql)
	if markerIndex == -1 {
		return sql, 0, 0
	}
	beforeCaret := sql[:markerIndex]
	line := strings.Count(beforeCaret, "\n") + 1
	lineStart := strings.LastIndex(beforeCaret, "\n") + 1
	// The analyzer counts characters, so a multi-byte identifier earlier on the
	// line would move the caret if this counted bytes.
	col := utf8.RuneCountInString(beforeCaret[lineStart:])
	return beforeCaret + sql[markerIndex+1:], line, col
}

// caretMarkerIndex finds the caret marker, stepping over || so a concatenation
// operator is not mistaken for one.
func caretMarkerIndex(sql string) int {
	for i := 0; i < len(sql); i++ {
		if sql[i] != '|' {
			continue
		}
		if i+1 < len(sql) && sql[i+1] == '|' {
			i++
			continue
		}
		return i
	}
	return -1
}
