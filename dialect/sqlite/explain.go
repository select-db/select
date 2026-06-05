package sqlite

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	core "github.com/selectDb/dialect/core"
)

func init() {
	core.RegisterExplainParser("sqlite", func() core.ExplainParser {
		return &sqliteExplainParser{}
	})
}

type sqliteExplainParser struct{}

var _ core.ExplainParser = (*sqliteExplainParser)(nil)

func (p *sqliteExplainParser) Dialect() string {
	return "sqlite"
}

func (p *sqliteExplainParser) BuildExplainQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return fmt.Sprintf("EXPLAIN QUERY PLAN %s", q)
}

// Parse parses SQLite EXPLAIN QUERY PLAN output.
// SQLite returns a result set with columns: selectid, order, from, detail
// Accepts string format (tab-separated values from readExplainRows) or *sql.Rows
func (p *sqliteExplainParser) Parse(rawExplain interface{}) (*core.ExplainNode, error) {
	var planRows []planRow

	switch v := rawExplain.(type) {
	case string:
		// Parse from tab-separated string format (from readExplainRows)
		lines := strings.Split(v, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// SQLite EXPLAIN QUERY PLAN returns: selectid, order, from, detail (tab-separated)
			parts := strings.Split(line, "\t")
			if len(parts) < 4 {
				continue
			}

			planRow := planRow{}

			// Parse selectid
			if selectid, err := parseInt64(parts[0]); err == nil {
				planRow.SelectID = selectid
			}

			// Parse order
			if order, err := parseInt64(parts[1]); err == nil {
				planRow.Order = order
			}

			// Parse from
			if from, err := parseInt64(parts[2]); err == nil {
				planRow.From = from
			}

			// Detail is the rest (may contain tabs)
			planRow.Detail = strings.Join(parts[3:], "\t")

			planRows = append(planRows, planRow)
		}
	case *sql.Rows:
		// Parse directly from sql.Rows
		defer func() { _ = v.Close() }()
		for v.Next() {
			var selectid, order, from sql.NullInt64
			var detail sql.NullString

			if err := v.Scan(&selectid, &order, &from, &detail); err != nil {
				return nil, fmt.Errorf("sqlite explain: failed to scan row: %w", err)
			}

			planRows = append(planRows, planRow{
				SelectID: selectid.Int64,
				Order:    order.Int64,
				From:     from.Int64,
				Detail:   detail.String,
			})
		}
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("sqlite explain: error iterating rows: %w", err)
		}
	default:
		return nil, fmt.Errorf("sqlite explain: expected string or *sql.Rows, got %T", rawExplain)
	}

	if len(planRows) == 0 {
		return nil, fmt.Errorf("sqlite explain: no plan rows returned")
	}

	builder := &planBuilder{counter: 0}
	return builder.buildTree(planRows)
}

type planRow struct {
	SelectID int64
	Order    int64
	From     int64
	Detail   string
}

type planBuilder struct {
	counter int
}

func (b *planBuilder) nextID() string {
	id := fmt.Sprintf("node_%d", b.counter)
	b.counter++
	return id
}

// buildTree constructs a tree from SQLite plan rows.
// SQLite EXPLAIN QUERY PLAN returns rows in execution order.
// The hierarchy is typically flat, but we can infer parent-child relationships
// based on the operation types and the order column.
func (b *planBuilder) buildTree(rows []planRow) (*core.ExplainNode, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("sqlite explain: empty plan")
	}

	// Build nodes from rows
	nodes := make([]*core.ExplainNode, len(rows))
	for i, row := range rows {
		node, err := b.parseRow(row)
		if err != nil {
			return nil, err
		}
		nodes[i] = node
	}

	// SQLite plans are typically executed in order, with later operations
	// being children of earlier ones. However, some operations like
	// "USE TEMP B-TREE" are sub-operations of the previous node.
	// We'll build a simple tree where:
	// 1. The first node is the root
	// 2. Subsequent nodes are children unless they're clearly sub-operations
	// 3. Sub-operations (USE TEMP, LIST SUBQUERY, etc.) are children of the previous node

	root := nodes[0]
	stack := []*core.ExplainNode{root}

	for i := 1; i < len(nodes); i++ {
		current := nodes[i]

		// Determine parent: if current is a sub-operation, parent is the last node
		// Otherwise, it's typically a sibling or child based on operation type
		if b.isSubOperation(current.Operation) {
			// Sub-operations are always children of the previous node
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, current)
			}
			// Don't push sub-operations onto stack
		} else {
			// Regular operations: check if this should be a child or sibling
			// In SQLite, operations are typically executed sequentially,
			// so we'll make them children of the root or previous major operation
			if len(stack) > 0 {
				// For now, make it a child of the root for simplicity
				// This can be refined based on operation types
				root.Children = append(root.Children, current)
			}
			// Push major operations onto stack (but we'll keep it simple for now)
			stack = append(stack, current)
			if len(stack) > 3 {
				// Limit stack depth
				stack = stack[len(stack)-3:]
			}
		}
	}

	return root, nil
}

func (p *sqliteExplainParser) GetFormattingConfig() core.FormattingConfig {
	// SQLite uses 4KB pages by default, but this is configurable
	// SQLite doesn't typically report memory in explain plans
	return core.FormattingConfig{
		BlockSizeBytes: 4096, // SQLite default page size is 4KB
		MemoryUnit:     "KB",  // Default assumption (SQLite doesn't report memory)
	}
}

func (b *planBuilder) parseRow(row planRow) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:       b.nextID(),
		Type:     core.NodeTypeOther,
		Children: []*core.ExplainNode{},
		Metadata: make(map[string]interface{}),
	}

	// Parse detail string to extract operation, table, index, etc.
	detail := strings.TrimSpace(row.Detail)
	if detail == "" {
		return nil, fmt.Errorf("sqlite explain: empty detail in plan row")
	}

	// Extract operation type and details
	operation, table, index, condition := b.parseDetail(detail)
	node.Operation = operation
	node.Type = mapNodeType(operation)

	if table != "" {
		node.TargetTable = &table
	}
	if index != "" {
		node.IndexName = &index
	}
	if condition != "" {
		node.Condition = &condition
	}

	// Store raw metadata
	node.Metadata["selectid"] = row.SelectID
	node.Metadata["order"] = row.Order
	node.Metadata["from"] = row.From
	node.Metadata["detail"] = row.Detail

	return node, nil
}

// parseDetail extracts information from SQLite's detail string.
// Examples:
//   - "SCAN TABLE users"
//   - "SEARCH TABLE users USING INDEX idx_name (name=?)"
//   - "USE TEMP B-TREE FOR ORDER BY"
//   - "COMPOUND QUERY"
func (b *planBuilder) parseDetail(detail string) (operation, table, index, condition string) {
	detail = strings.TrimSpace(detail)
	upper := strings.ToUpper(detail)

	// Extract operation type (first word or phrase)
	if strings.HasPrefix(upper, "SCAN TABLE") {
		operation = "SCAN TABLE"
		// Extract table name: "SCAN TABLE users" -> "users"
		parts := strings.Fields(detail)
		if len(parts) >= 3 {
			table = parts[2]
		}
	} else if strings.HasPrefix(upper, "SEARCH TABLE") {
		operation = "SEARCH TABLE"
		// Extract table and index: "SEARCH TABLE users USING INDEX idx_name (name=?)"
		parts := strings.Fields(detail)
		if len(parts) >= 3 {
			table = parts[2]
		}
		// Look for "USING INDEX"
		if idx := strings.Index(upper, "USING INDEX"); idx != -1 {
			idxPart := detail[idx+len("USING INDEX"):]
			idxPart = strings.TrimSpace(idxPart)
			// Extract index name (before parentheses or end of string)
			if parenIdx := strings.Index(idxPart, "("); parenIdx != -1 {
				index = strings.TrimSpace(idxPart[:parenIdx])
				// Extract condition from parentheses
				if parenEnd := strings.Index(idxPart, ")"); parenEnd != -1 && parenEnd > parenIdx {
					condition = idxPart[parenIdx+1 : parenEnd]
				}
			} else {
				index = idxPart
			}
		}
	} else if strings.HasPrefix(upper, "USE TEMP B-TREE") {
		operation = "USE TEMP B-TREE"
		if strings.Contains(upper, "ORDER BY") {
			operation = "USE TEMP B-TREE FOR ORDER BY"
		} else if strings.Contains(upper, "GROUP BY") {
			operation = "USE TEMP B-TREE FOR GROUP BY"
		}
	} else if strings.HasPrefix(upper, "COMPOUND QUERY") {
		operation = "COMPOUND QUERY"
	} else if strings.HasPrefix(upper, "EXECUTE") {
		operation = "EXECUTE"
	} else if strings.HasPrefix(upper, "LIST SUBQUERY") {
		operation = "LIST SUBQUERY"
	} else {
		// Default: use first few words as operation
		parts := strings.Fields(detail)
		if len(parts) > 0 {
			operation = strings.ToUpper(parts[0])
			if len(parts) > 1 {
				operation += " " + strings.ToUpper(parts[1])
			}
		} else {
			operation = detail
		}
	}

	return operation, table, index, condition
}

func (b *planBuilder) isSubOperation(operation string) bool {
	upper := strings.ToUpper(operation)
	// Operations that are typically sub-operations of the previous node
	subOps := []string{
		"USE TEMP",
		"LIST SUBQUERY",
		"EXECUTE",
		"CO-ROUTINE",
		"MATERIALIZE",
	}
	for _, subOp := range subOps {
		if strings.Contains(upper, subOp) {
			return true
		}
	}
	return false
}

func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func mapNodeType(operation string) core.NodeType {
	upper := strings.ToUpper(operation)
	switch {
	case strings.Contains(upper, "SCAN"):
		return core.NodeTypeScan
	case strings.Contains(upper, "SEARCH"):
		return core.NodeTypeScan
	case strings.Contains(upper, "JOIN"):
		return core.NodeTypeJoin
	case strings.Contains(upper, "AGGREGATE") || strings.Contains(upper, "GROUP"):
		return core.NodeTypeAggregate
	case strings.Contains(upper, "ORDER") || strings.Contains(upper, "SORT"):
		return core.NodeTypeSort
	case strings.Contains(upper, "FILTER") || strings.Contains(upper, "WHERE"):
		return core.NodeTypeFilter
	case strings.Contains(upper, "SUBQUERY") || strings.Contains(upper, "COMPOUND"):
		return core.NodeTypeSubquery
	case strings.Contains(upper, "INSERT"):
		return core.NodeTypeInsert
	case strings.Contains(upper, "UPDATE"):
		return core.NodeTypeUpdate
	case strings.Contains(upper, "DELETE"):
		return core.NodeTypeDelete
	default:
		return core.NodeTypeOther
	}
}
