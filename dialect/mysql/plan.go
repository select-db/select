package mysql

import (
	"encoding/json"
	"fmt"
	"strings"

	core "github.com/selectDb/dialect/core"
)

func init() {
	core.RegisterPlanParser("mysql", func() core.PlanParser {
		return &mysqlPlanParser{}
	})
}

type mysqlPlanParser struct{}

var _ core.PlanParser = (*mysqlPlanParser)(nil)

func (p *mysqlPlanParser) Dialect() string { return "mysql" }

func (p *mysqlPlanParser) BuildPlanQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return fmt.Sprintf("EXPLAIN FORMAT=JSON %s", q)
}

func (p *mysqlPlanParser) GetFormattingConfig() core.FormattingConfig {
	return core.FormattingConfig{
		BlockSizeBytes: 16384,
		MemoryUnit:     "B",
	}
}

// Parse handles MySQL EXPLAIN FORMAT=JSON output. The shape is:
// { "query_block": { … } } where query_block holds nested keys like
// "ordering_operation", "grouping_operation", "nested_loop", "table",
// "attached_subqueries", "union_result", etc.
func (p *mysqlPlanParser) Parse(rawPlan interface{}) (*core.ExplainNode, error) {
	payload, err := normalizeRawPlanMySQL(rawPlan)
	if err != nil {
		return nil, err
	}
	var top map[string]interface{}
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil, fmt.Errorf("mysql plan: %w", err)
	}
	qb, ok := top["query_block"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("mysql plan: missing query_block")
	}
	builder := &mysqlPlanBuilder{}
	return builder.parseQueryBlock(qb)
}

type mysqlPlanBuilder struct {
	counter int
}

func (b *mysqlPlanBuilder) nextID() string {
	id := fmt.Sprintf("node_%d", b.counter)
	b.counter++
	return id
}

// parseQueryBlock walks the keys of a query_block (or any operation node) and
// builds a tree. Unknown keys are stashed into Metadata so callers can still
// surface them.
func (b *mysqlPlanBuilder) parseQueryBlock(qb map[string]interface{}) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:        b.nextID(),
		Type:      core.NodeTypeOther,
		Operation: "query_block",
		Children:  []*core.ExplainNode{},
		Metadata:  make(map[string]interface{}),
	}
	if v, ok := qb["select_id"].(float64); ok {
		node.Metadata["select_id"] = v
	}
	if cost, ok := qb["cost_info"].(map[string]interface{}); ok {
		if qc, ok := cost["query_cost"].(string); ok {
			if f, err := parseFloat(qc); err == nil {
				node.EstimatedCost = &f
			}
		}
	}

	// Walk operation keys in a stable order: outermost-first.
	keys := []string{
		"ordering_operation",
		"duplicates_removal",
		"grouping_operation",
		"windowing",
		"nested_loop",
		"table",
		"union_result",
		"materialized_from_subquery",
	}
	for _, k := range keys {
		raw, ok := qb[k]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case map[string]interface{}:
			if child, err := b.parseOperation(k, v); err == nil && child != nil {
				node.Children = append(node.Children, child)
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if child, err := b.parseOperation(k, m); err == nil && child != nil {
						node.Children = append(node.Children, child)
					}
				}
			}
		}
	}

	if sub, ok := qb["attached_subqueries"].([]interface{}); ok {
		for _, item := range sub {
			if m, ok := item.(map[string]interface{}); ok {
				if qb2, ok := m["query_block"].(map[string]interface{}); ok {
					if child, err := b.parseQueryBlock(qb2); err == nil && child != nil {
						child.Operation = "attached_subquery"
						node.Children = append(node.Children, child)
					}
				}
			}
		}
	}

	for key, value := range qb {
		if isMySQLPlanStandardField(key) {
			continue
		}
		node.Metadata[key] = value
	}
	computeExclusiveCostMySQL(node)
	return node, nil
}

// parseOperation handles a single operation node by name.
func (b *mysqlPlanBuilder) parseOperation(kind string, op map[string]interface{}) (*core.ExplainNode, error) {
	switch kind {
	case "table":
		return b.parseTable(op)
	case "nested_loop":
		// nested_loop is shaped as an array; the caller flattens it.
		return b.parseGeneric(kind, op)
	case "materialized_from_subquery":
		if qb, ok := op["query_block"].(map[string]interface{}); ok {
			child, err := b.parseQueryBlock(qb)
			if err == nil && child != nil {
				child.Operation = "materialized_from_subquery"
				child.Type = core.NodeTypeSubquery
			}
			return child, err
		}
		return b.parseGeneric(kind, op)
	case "union_result":
		return b.parseUnionResult(op)
	case "ordering_operation", "duplicates_removal", "grouping_operation", "windowing":
		return b.parseGeneric(kind, op)
	}
	return b.parseGeneric(kind, op)
}

// parseGeneric wraps an arbitrary operation in an ExplainNode and recurses into
// any nested operation keys it contains.
func (b *mysqlPlanBuilder) parseGeneric(kind string, op map[string]interface{}) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:        b.nextID(),
		Type:      mapMySQLOperation(kind),
		Operation: kind,
		Children:  []*core.ExplainNode{},
		Metadata:  make(map[string]interface{}),
	}
	if cost, ok := op["cost_info"].(map[string]interface{}); ok {
		populateMySQLCost(node, cost)
	}
	if v, ok := op["rows_examined_per_scan"].(float64); ok {
		node.EstimatedRows = &v
	}
	for _, key := range []string{"nested_loop", "table", "ordering_operation", "grouping_operation", "windowing", "duplicates_removal", "materialized_from_subquery", "union_result"} {
		raw, ok := op[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case map[string]interface{}:
			if child, err := b.parseOperation(key, v); err == nil && child != nil {
				node.Children = append(node.Children, child)
			}
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if child, err := b.parseOperation(key, m); err == nil && child != nil {
						node.Children = append(node.Children, child)
					}
				}
			}
		}
	}
	for k, v := range op {
		if isMySQLPlanStandardField(k) {
			continue
		}
		node.Metadata[k] = v
	}
	return node, nil
}

// parseTable handles a single "table" operation.
func (b *mysqlPlanBuilder) parseTable(t map[string]interface{}) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:        b.nextID(),
		Operation: "table",
		Type:      core.NodeTypeScan,
		Children:  []*core.ExplainNode{},
		Metadata:  make(map[string]interface{}),
	}
	if name, ok := t["table_name"].(string); ok {
		node.TargetTable = &name
	}
	if accessType, ok := t["access_type"].(string); ok {
		node.Operation = accessType + " on " + getString(t["table_name"])
	}
	if idx, ok := t["key"].(string); ok {
		node.IndexName = &idx
	}
	if cond, ok := t["attached_condition"].(string); ok {
		node.Condition = &cond
	}
	if v, ok := t["rows_examined_per_scan"].(float64); ok {
		node.EstimatedRows = &v
	}
	if v, ok := t["rows_produced_per_join"].(float64); ok && node.EstimatedRows == nil {
		node.EstimatedRows = &v
	}
	if cost, ok := t["cost_info"].(map[string]interface{}); ok {
		populateMySQLCost(node, cost)
	}
	if mat, ok := t["materialized_from_subquery"].(map[string]interface{}); ok {
		if qb, ok := mat["query_block"].(map[string]interface{}); ok {
			if child, err := b.parseQueryBlock(qb); err == nil && child != nil {
				child.Operation = "materialized_from_subquery"
				child.Type = core.NodeTypeSubquery
				node.Children = append(node.Children, child)
			}
		}
	}
	if attached, ok := t["attached_subqueries"].([]interface{}); ok {
		for _, item := range attached {
			if m, ok := item.(map[string]interface{}); ok {
				if qb, ok := m["query_block"].(map[string]interface{}); ok {
					if child, err := b.parseQueryBlock(qb); err == nil && child != nil {
						child.Operation = "attached_subquery"
						node.Children = append(node.Children, child)
					}
				}
			}
		}
	}
	for k, v := range t {
		if isMySQLPlanStandardField(k) {
			continue
		}
		node.Metadata[k] = v
	}
	return node, nil
}

func (b *mysqlPlanBuilder) parseUnionResult(u map[string]interface{}) (*core.ExplainNode, error) {
	node := &core.ExplainNode{
		ID:        b.nextID(),
		Operation: "union",
		Type:      core.NodeTypeOther,
		Children:  []*core.ExplainNode{},
		Metadata:  make(map[string]interface{}),
	}
	if specs, ok := u["query_specifications"].([]interface{}); ok {
		for _, spec := range specs {
			if m, ok := spec.(map[string]interface{}); ok {
				if qb, ok := m["query_block"].(map[string]interface{}); ok {
					if child, err := b.parseQueryBlock(qb); err == nil && child != nil {
						node.Children = append(node.Children, child)
					}
				}
			}
		}
	}
	for k, v := range u {
		if k == "query_specifications" {
			continue
		}
		node.Metadata[k] = v
	}
	return node, nil
}

func populateMySQLCost(node *core.ExplainNode, cost map[string]interface{}) {
	if v, ok := cost["read_cost"].(string); ok {
		if f, err := parseFloat(v); err == nil {
			node.EstimatedCost = &f
		}
	}
	if v, ok := cost["query_cost"].(string); ok && node.EstimatedCost == nil {
		if f, err := parseFloat(v); err == nil {
			node.EstimatedCost = &f
		}
	}
}

func mapMySQLOperation(kind string) core.NodeType {
	switch kind {
	case "nested_loop":
		return core.NodeTypeJoin
	case "grouping_operation":
		return core.NodeTypeAggregate
	case "ordering_operation":
		return core.NodeTypeSort
	case "duplicates_removal":
		return core.NodeTypeFilter
	case "windowing":
		return core.NodeTypeOther
	case "table":
		return core.NodeTypeScan
	case "materialized_from_subquery":
		return core.NodeTypeSubquery
	}
	return core.NodeTypeOther
}

func computeExclusiveCostMySQL(node *core.ExplainNode) {
	if node.EstimatedCost == nil {
		return
	}
	var childrenTotal float64
	for _, child := range node.Children {
		if child.EstimatedCost != nil {
			childrenTotal += *child.EstimatedCost
		}
	}
	exclusive := *node.EstimatedCost - childrenTotal
	if exclusive < 0 {
		exclusive = 0
	}
	node.ExclusiveCost = &exclusive
}

func normalizeRawPlanMySQL(raw interface{}) ([]byte, error) {
	switch v := raw.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case json.RawMessage:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("mysql plan: unexpected payload type %T", raw)
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0, err
	}
	return f, nil
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

var mysqlPlanStandardFields = map[string]struct{}{
	"select_id":              {},
	"cost_info":              {},
	"ordering_operation":     {},
	"duplicates_removal":     {},
	"grouping_operation":     {},
	"windowing":              {},
	"nested_loop":            {},
	"table":                  {},
	"table_name":             {},
	"access_type":            {},
	"key":                    {},
	"attached_condition":     {},
	"rows_examined_per_scan": {},
	"rows_produced_per_join": {},
	"attached_subqueries":    {},
	"union_result":           {},
	"materialized_from_subquery": {},
	"query_specifications":   {},
}

func isMySQLPlanStandardField(key string) bool {
	_, ok := mysqlPlanStandardFields[key]
	return ok
}
