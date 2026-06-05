package sqlite

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	core "github.com/selectDb/dialect/core"
)

func init() {
	core.RegisterPlanParser("sqlite", func() core.PlanParser {
		return &sqlitePlanParser{}
	})
}

type sqlitePlanParser struct{}

var _ core.PlanParser = (*sqlitePlanParser)(nil)

func (p *sqlitePlanParser) Dialect() string {
	return "sqlite"
}

func (p *sqlitePlanParser) BuildPlanQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return fmt.Sprintf("EXPLAIN QUERY PLAN %s", q)
}

func (p *sqlitePlanParser) GetFormattingConfig() core.FormattingConfig {
	return core.FormattingConfig{
		BlockSizeBytes: 4096,
		MemoryUnit:     "KB",
	}
}

func (p *sqlitePlanParser) Parse(rawPlan interface{}) (*core.ExplainNode, error) {
	var planRows []sqlitePlanRow
	switch v := rawPlan.(type) {
	case string:
		lines := strings.Split(v, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 4 {
				continue
			}
			row := sqlitePlanRow{}
			if selectid, err := parseInt64Plan(parts[0]); err == nil {
				row.SelectID = selectid
			}
			if order, err := parseInt64Plan(parts[1]); err == nil {
				row.Order = order
			}
			if from, err := parseInt64Plan(parts[2]); err == nil {
				row.From = from
			}
			row.Detail = strings.Join(parts[3:], "\t")
			planRows = append(planRows, row)
		}
	case *sql.Rows:
		defer func() { _ = v.Close() }()
		for v.Next() {
			var selectid, order, from sql.NullInt64
			var detail sql.NullString
			if err := v.Scan(&selectid, &order, &from, &detail); err != nil {
				return nil, fmt.Errorf("sqlite plan: failed to scan row: %w", err)
			}
			planRows = append(planRows, sqlitePlanRow{
				SelectID: selectid.Int64,
				Order:    order.Int64,
				From:     from.Int64,
				Detail:   detail.String,
			})
		}
		if err := v.Err(); err != nil {
			return nil, fmt.Errorf("sqlite plan: error iterating rows: %w", err)
		}
	default:
		return nil, fmt.Errorf("sqlite plan: expected string or *sql.Rows, got %T", rawPlan)
	}
	if len(planRows) == 0 {
		return nil, fmt.Errorf("sqlite plan: no plan rows returned")
	}
	builder := &sqlitePlanBuilder{counter: 0}
	return builder.buildTree(planRows)
}

type sqlitePlanRow struct {
	SelectID int64
	Order    int64
	From     int64
	Detail   string
}

type sqlitePlanBuilder struct {
	counter int
}

func (b *sqlitePlanBuilder) nextID() string {
	id := fmt.Sprintf("node_%d", b.counter)
	b.counter++
	return id
}

func (b *sqlitePlanBuilder) buildTree(rows []sqlitePlanRow) (*core.ExplainNode, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("sqlite plan: empty plan")
	}
	nodes := make([]*core.ExplainNode, len(rows))
	for i, row := range rows {
		node, err := b.parseRow(row)
		if err != nil {
			return nil, err
		}
		nodes[i] = node
	}
	root := nodes[0]
	stack := []*core.ExplainNode{root}
	for i := 1; i < len(nodes); i++ {
		current := nodes[i]
		if b.isSubOperation(current.Operation) {
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, current)
			}
		} else {
			if len(stack) > 0 {
				root.Children = append(root.Children, current)
			}
			stack = append(stack, current)
			if len(stack) > 3 {
				stack = stack[len(stack)-3:]
			}
		}
	}
	return root, nil
}

func (b *sqlitePlanBuilder) parseRow(row sqlitePlanRow) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:       b.nextID(),
		Type:     core.NodeTypeOther,
		Children: []*core.ExplainNode{},
		Metadata: make(map[string]interface{}),
	}
	detail := strings.TrimSpace(row.Detail)
	if detail == "" {
		return nil, fmt.Errorf("sqlite plan: empty detail in plan row")
	}
	operation, table, index, condition := b.parseDetail(detail)
	node.Operation = operation
	node.Type = mapPlanNodeType(operation)
	if table != "" {
		node.TargetTable = &table
	}
	if index != "" {
		node.IndexName = &index
	}
	if condition != "" {
		node.Condition = &condition
	}
	node.Metadata["selectid"] = row.SelectID
	node.Metadata["order"] = row.Order
	node.Metadata["from"] = row.From
	node.Metadata["detail"] = row.Detail
	return node, nil
}

func (b *sqlitePlanBuilder) parseDetail(detail string) (operation, table, index, condition string) {
	detail = strings.TrimSpace(detail)
	upper := strings.ToUpper(detail)
	if strings.HasPrefix(upper, "SCAN TABLE") {
		operation = "SCAN TABLE"
		parts := strings.Fields(detail)
		if len(parts) >= 3 {
			table = parts[2]
		}
	} else if strings.HasPrefix(upper, "SEARCH TABLE") {
		operation = "SEARCH TABLE"
		parts := strings.Fields(detail)
		if len(parts) >= 3 {
			table = parts[2]
		}
		if idx := strings.Index(upper, "USING INDEX"); idx != -1 {
			idxPart := detail[idx+len("USING INDEX"):]
			idxPart = strings.TrimSpace(idxPart)
			if parenIdx := strings.Index(idxPart, "("); parenIdx != -1 {
				index = strings.TrimSpace(idxPart[:parenIdx])
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

func (b *sqlitePlanBuilder) isSubOperation(operation string) bool {
	upper := strings.ToUpper(operation)
	subOps := []string{"USE TEMP", "LIST SUBQUERY", "EXECUTE", "CO-ROUTINE", "MATERIALIZE"}
	for _, subOp := range subOps {
		if strings.Contains(upper, subOp) {
			return true
		}
	}
	return false
}

func parseInt64Plan(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseInt(s, 10, 64)
}

func mapPlanNodeType(operation string) core.NodeType {
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
