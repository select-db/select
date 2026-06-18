package db_client

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	FormatCSVSemicolon = "csv_semicolon"
	FormatCSVComma     = "csv_comma"
	FormatJSON         = "json"
)

type ExportParams struct {
	FileID       string            // temp or disk file identifier
	Statement    string            // SQL content to execute
	DbInstanceID string
	FolderID     string
	Format       string            // csv_semicolon | csv_comma | json
	Filename     string
	RuntimeVars  map[string]string // user-supplied values for unresolved $variables
}

// Export runs the query with full results, formats the output, and saves via OS save dialog.
func (dbc *DbClient) Export(params ExportParams) error {
	dbInstance := dbc.Graph.GetDBInstanceNodeByID(params.DbInstanceID)
	if dbInstance == nil {
		return fmt.Errorf("failed to get DB instance with id: %s", params.DbInstanceID)
	}
	result := dbc.Query(QueryParams{
		FileID:       params.FileID,
		Statement:    params.Statement,
		DbInstanceID: params.DbInstanceID,
		FolderID:     params.FolderID,
		ForExport:    true,
		RuntimeVars:  params.RuntimeVars,
	})

	if len(result.Errors) > 0 {
		return errors.New(result.Errors[0])
	}

	if len(result.Columns) == 0 || result.RowCount == 0 {
		if result.AffectedRows > 0 {
			return errors.New("no rows to export (query affected rows but returned no result set)")
		}
		return errors.New("no data to export")
	}

	data, err := formatExport(result.Columns, result.Rows, params.Format)
	if err != nil {
		return fmt.Errorf("failed to format: %w", err)
	}

	ext := ".csv"
	if params.Format == FormatJSON {
		ext = ".json"
	}
	filename := params.Filename
	if filename == "" {
		filename = "export" + ext
	} else if !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename = filename + ext
	}

	var filters []runtime.FileFilter
	if params.Format == FormatJSON {
		filters = []runtime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
			{DisplayName: "All", Pattern: "*"},
		}
	} else {
		filters = []runtime.FileFilter{
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All", Pattern: "*"},
		}
	}

	path, err := runtime.SaveFileDialog(dbc.ctx, runtime.SaveDialogOptions{
		DefaultFilename: filename,
		Title:           "Export query results",
		Filters:         filters,
	})
	if err != nil {
		return fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return nil // user cancelled
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func formatExport(columns []string, rows [][]interface{}, format string) ([]byte, error) {
	switch format {
	case FormatCSVSemicolon:
		return formatCSV(columns, rows, ';')
	case FormatCSVComma:
		return formatCSV(columns, rows, ',')
	case FormatJSON:
		return formatJSON(columns, rows)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

func formatCSV(columns []string, rows [][]interface{}, delim rune) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Comma = delim

	if err := w.Write(columns); err != nil {
		return nil, err
	}
	for _, row := range rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = valueToString(v)
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return []byte(buf.String()), w.Error()
}

func valueToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatJSON(columns []string, rows [][]interface{}) ([]byte, error) {
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			if i < len(row) {
				obj[col] = row[i]
			}
		}
		items = append(items, obj)
	}
	return json.MarshalIndent(items, "", "  ")
}
