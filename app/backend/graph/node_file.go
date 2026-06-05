package graph

import (
	"selectDb/backend/utils"

	"github.com/selectDb/dialect/core"
)

type FileDTO struct {
	ID  *string `json:"id,omitempty"`
	URI *string `json:"uri,omitempty"`

	Name *string `json:"name,omitempty"`

	FolderID       *string                   `json:"folder_id,omitempty"`
	Databases      *[]DatabaseRef            `json:"databases,omitempty"`
	QueryResults   map[string]*QueryResult   `json:"queryResults,omitempty"`
	PlanResults    map[string]*ExplainResult `json:"planResults,omitempty"`
	ExplainResults map[string]*ExplainResult `json:"explainResults,omitempty"`
}

type QueryResult struct {
	Id            string          `json:"id,omitempty"`
	Columns       []string        `json:"columns,omitempty"`
	Rows          [][]interface{} `json:"rows,omitempty"`
	AffectedRows  int64           `json:"affectedRows,omitempty"`
	RowCount      int             `json:"rowCount"`
	DurationMs    int64           `json:"durationMs,omitempty"`
	Errors        []string        `json:"errors,omitempty"`
	ErrorPosition *int            `json:"errorPosition,omitempty"` // 1-based character offset when the DB reports an error location
	Explain       *ExplainResult  `json:"explain,omitempty"`

	Page     int `json:"page"`
	PageSize int `json:"pageSize"`

	// Streaming watermark (rows safe to read from index 0). Only meaningful on
	// page responses; "ready" / "partial" / "pending" disambiguates RowCount.
	Available int    `json:"available,omitempty"`
	Status    string `json:"status,omitempty"`

	// Per-column editable metadata - populated for each column that can be edited
	ColumnMetadata []ColumnMetadata `json:"columnMetadata,omitempty"`
}

// ColumnMetadata contains editability information for a single column
type ColumnMetadata struct {
	HasAllPrimaryKeys  bool     `json:"hasAllPrimaryKeys"`            // Whether this column's table has all primary keys present in result (enables editing)
	IsPrimaryKey       bool     `json:"isPrimaryKey"`                 // Whether this specific column is a primary key for its table
	IsForeignKey       bool     `json:"isForeignKey,omitempty"`       // Whether this specific column is a foreign key
	DatabaseID         string   `json:"databaseId,omitempty"`         // Database ID (only if hasAllPrimaryKeys)
	Schema             string   `json:"schema,omitempty"`             // Schema name (only if hasAllPrimaryKeys)
	Table              string   `json:"table,omitempty"`              // Table name (only if hasAllPrimaryKeys)
	OriginalColumnName string   `json:"originalColumnName,omitempty"` // Original column name (before alias)
	PrimaryKeys        []string `json:"primaryKeys,omitempty"`        // Primary key column names for this table
	PrimaryKeysIdxs    []int    `json:"primaryKeysIdxs,omitempty"`    // Indices in result where primary key columns appear
	DataType           string   `json:"dataType,omitempty"`           // Native column type (e.g. varchar(255), int4, mood)
	EnumValues         []string `json:"enumValues,omitempty"`         // Allowed values when the column is an enumerated type; drives the cell picker
}

type ExplainResult struct {
	Id             string            `json:"id,omitempty"`
	Root           *core.ExplainNode `json:"root,omitempty"`
	TotalCost      *float64          `json:"totalCost,omitempty"`
	Raw            string            `json:"raw,omitempty"`
	Errors         []string          `json:"errors,omitempty"`
	ErrorPosition  *int              `json:"errorPosition,omitempty"` // 1-based character offset when the DB reports an error location (e.g. Postgres)
	DurationMs     int64             `json:"durationMs,omitempty"`
}

type FileNode struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Type string `json:"type"`

	Name string `json:"name"`

	FolderID       string                    `json:"folder_id"`
	Databases      []DatabaseRef             `json:"databases,omitempty"`
	QueryResults   map[string]*QueryResult   `json:"queryResults,omitempty"`
	PlanResults    map[string]*ExplainResult `json:"planResults,omitempty"`
	ExplainResults map[string]*ExplainResult `json:"explainResults,omitempty"`
	Badges         []string                  `json:"badges"`
}

func BuildFileNode(f FileDTO) *FileNode {
	folderID := f.FolderID
	if *folderID == "" {
		folderID = utils.Ptr("root")
	}

	var databases []DatabaseRef
	if f.Databases != nil {
		databases = *f.Databases
	} else {
		databases = []DatabaseRef{}
	}

	return &FileNode{
		ID:   *f.ID,
		URI:  *f.URI,
		Type: "file",

		Name: *utils.DefaultIfNil(f.Name, ""),

		FolderID:       *folderID,
		Databases:      databases,
		QueryResults:   f.QueryResults,
		PlanResults:    f.PlanResults,
		ExplainResults: f.ExplainResults,
	}
}

func (f *FileNode) GetIDs() []string {
	return []string{f.ID}
}

func (f *FileNode) GetParentIDs() []string {
	return []string{f.FolderID}
}

func (f *FileNode) RemoveChildByIDs(IDs []string) bool {
	return false
}

func (f *FileNode) GetChildren() []Node {
	return []Node{}
}

func (f *FileNode) AddChild(n Node) bool {
	return true
}
