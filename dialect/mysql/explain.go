package mysql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	core "github.com/selectDb/dialect/core"
)

func init() {
	core.RegisterExplainParser("mysql", func() core.ExplainParser {
		return &mysqlExplainParser{}
	})
}

type mysqlExplainParser struct{}

var _ core.ExplainParser = (*mysqlExplainParser)(nil)

func (p *mysqlExplainParser) Dialect() string { return "mysql" }

func (p *mysqlExplainParser) BuildExplainQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return fmt.Sprintf("EXPLAIN ANALYZE FORMAT=TREE %s", q)
}

func (p *mysqlExplainParser) GetFormattingConfig() core.FormattingConfig {
	return core.FormattingConfig{
		BlockSizeBytes: 16384,
		MemoryUnit:     "B",
	}
}

// Parse accepts the text output of EXPLAIN ANALYZE FORMAT=TREE and builds a
// nested ExplainNode tree from the indentation. Each line looks like:
//
//	-> Aggregate: sum(t1.c1)  (cost=10..10 rows=1) (actual time=5.0..5.1 rows=1 loops=1)
//	    -> Table scan on t1  (cost=0.35 rows=42) (actual time=0.1..0.5 rows=42 loops=1)
func (p *mysqlExplainParser) Parse(rawExplain interface{}) (*core.ExplainNode, error) {
	text, err := normalizeRawExplainMySQL(rawExplain)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(text, "\n")

	type frame struct {
		indent int
		node   *core.ExplainNode
	}
	var stack []frame
	var root *core.ExplainNode
	counter := 0
	nextID := func() string {
		id := fmt.Sprintf("node_%d", counter)
		counter++
		return id
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		idx := strings.Index(line, "-> ")
		if idx < 0 {
			continue
		}
		indent := idx
		content := strings.TrimSpace(line[idx+3:])
		node := parseMySQLTreeLine(content)
		node.ID = nextID()

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			root = node
		} else {
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, node)
		}
		stack = append(stack, frame{indent: indent, node: node})
	}

	if root == nil {
		return nil, fmt.Errorf("mysql explain: no plan nodes parsed")
	}
	return root, nil
}

// parseMySQLTreeLine parses a single "node line" like:
//
//	Aggregate: sum(c1)  (cost=10..10 rows=1) (actual time=5.0..5.1 rows=1 loops=1)
func parseMySQLTreeLine(s string) *core.ExplainNode {
	node := &core.ExplainNode{
		Type:     core.NodeTypeOther,
		Children: []*core.ExplainNode{},
		Metadata: make(map[string]interface{}),
	}
	// Strip the trailing "(...) (...)" groups and parse each.
	parens := mysqlExplainParenGroups(s)
	head := s
	if len(parens) > 0 {
		head = strings.TrimSpace(s[:parens[0].start])
	}
	node.Operation = head
	node.Type = classifyMySQLOperation(head)
	for _, g := range parens {
		body := s[g.start+1 : g.end]
		parseMySQLParenGroup(body, node)
	}
	if strings.HasPrefix(head, "Table scan on ") || strings.HasPrefix(head, "Index scan on ") || strings.HasPrefix(head, "Index lookup on ") || strings.HasPrefix(head, "Index range scan on ") {
		fields := strings.Fields(head)
		if len(fields) >= 4 {
			name := fields[3]
			node.TargetTable = &name
		}
	}
	return node
}

// parseMySQLParenGroup extracts known metrics (cost, rows, actual time) from a
// "(...)" group and stashes anything else into Metadata.
func parseMySQLParenGroup(body string, node *core.ExplainNode) {
	body = strings.TrimSpace(body)
	// Recognise the two common shapes:
	//   cost=X[..Y] rows=Z
	//   actual time=X..Y rows=Z loops=W
	if strings.Contains(body, "actual time=") {
		if v, ok := findMySQLDuration(body, "actual time="); ok {
			node.ActualTime = &v
		}
		if v, ok := findMySQLFloat(body, "rows="); ok {
			node.ActualRows = &v
		}
		if v, ok := findMySQLFloat(body, "loops="); ok {
			node.Metadata["loops"] = v
		}
		return
	}
	if strings.HasPrefix(body, "cost=") {
		if v, ok := findMySQLDuration(body, "cost="); ok {
			node.EstimatedCost = &v
		}
		if v, ok := findMySQLFloat(body, "rows="); ok {
			node.EstimatedRows = &v
		}
		return
	}
	// Unknown group; keep verbatim.
	node.Metadata[strings.SplitN(body, " ", 2)[0]] = body
}

// findMySQLFloat scans body for `<key><number>` and returns the parsed value.
var floatRe = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:e[+-]?\d+)?`)

func findMySQLFloat(body, key string) (float64, bool) {
	idx := strings.Index(body, key)
	if idx < 0 {
		return 0, false
	}
	rest := body[idx+len(key):]
	m := floatRe.FindString(rest)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// findMySQLDuration parses `<key>start..end` and returns the end value (the
// last/total reading). When only a single value is present it returns that.
func findMySQLDuration(body, key string) (float64, bool) {
	idx := strings.Index(body, key)
	if idx < 0 {
		return 0, false
	}
	rest := body[idx+len(key):]
	// Find the first whitespace or close-paren after the value.
	end := strings.IndexAny(rest, " )")
	if end < 0 {
		end = len(rest)
	}
	chunk := rest[:end]
	if dot := strings.Index(chunk, ".."); dot >= 0 {
		chunk = chunk[dot+2:]
	}
	m := floatRe.FindString(chunk)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func classifyMySQLOperation(s string) core.NodeType {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "table scan"), strings.HasPrefix(lower, "index scan"),
		strings.HasPrefix(lower, "index lookup"), strings.HasPrefix(lower, "index range scan"),
		strings.HasPrefix(lower, "covering index lookup"):
		return core.NodeTypeScan
	case strings.HasPrefix(lower, "nested loop"), strings.HasPrefix(lower, "hash join"),
		strings.HasPrefix(lower, "inner hash join"):
		return core.NodeTypeJoin
	case strings.HasPrefix(lower, "aggregate"), strings.HasPrefix(lower, "group aggregate"):
		return core.NodeTypeAggregate
	case strings.HasPrefix(lower, "sort"):
		return core.NodeTypeSort
	case strings.HasPrefix(lower, "filter"):
		return core.NodeTypeFilter
	case strings.HasPrefix(lower, "materialize"), strings.HasPrefix(lower, "select #"):
		return core.NodeTypeSubquery
	}
	return core.NodeTypeOther
}

type parenRange struct{ start, end int }

// mysqlExplainParenGroups returns the spans of top-level "(...)" groups in s.
func mysqlExplainParenGroups(s string) []parenRange {
	var out []parenRange
	depth := 0
	start := -1
	for i, c := range s {
		switch c {
		case '(':
			if depth == 0 {
				start = i
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, parenRange{start: start, end: i})
				start = -1
			}
		}
	}
	return out
}

func normalizeRawExplainMySQL(raw interface{}) (string, error) {
	switch v := raw.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("mysql explain: unexpected payload type %T", raw)
	}
}
