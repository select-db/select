package postgresql

import (
	"encoding/json"
	"fmt"
	"strings"

	core "github.com/selectDb/dialect/core"
)

func init() {
	core.RegisterPlanParser("postgresql", func() core.PlanParser {
		return &pgPlanParser{}
	})
}

type pgPlanParser struct{}

var _ core.PlanParser = (*pgPlanParser)(nil)

func (p *pgPlanParser) Dialect() string {
	return "postgresql"
}

func (p *pgPlanParser) BuildPlanQuery(query string) string {
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	return fmt.Sprintf("EXPLAIN (FORMAT JSON) %s", q)
}

func (p *pgPlanParser) GetFormattingConfig() core.FormattingConfig {
	return core.FormattingConfig{
		BlockSizeBytes: 8192,
		MemoryUnit:     "KB",
	}
}

func (p *pgPlanParser) Parse(rawPlan interface{}) (*core.ExplainNode, error) {
	payload, err := normalizeRawPlan(rawPlan)
	if err != nil {
		return nil, err
	}
	var planPayload []map[string]interface{}
	if err := json.Unmarshal(payload, &planPayload); err != nil {
		return nil, fmt.Errorf("postgresql plan: %w", err)
	}
	if len(planPayload) == 0 {
		return nil, fmt.Errorf("postgresql plan: empty plan output")
	}
	planMap, ok := planPayload[0]["Plan"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("postgresql plan: missing Plan section")
	}
	builder := &pgPlanBuilder{counter: 0}
	return builder.parseNode(planMap)
}

type pgPlanBuilder struct {
	counter int
}

func (b *pgPlanBuilder) nextID() string {
	id := fmt.Sprintf("node_%d", b.counter)
	b.counter++
	return id
}

func (b *pgPlanBuilder) parseNode(plan map[string]interface{}) (*core.ExplainNode, error) {
	if plan == nil {
		return nil, fmt.Errorf("postgresql plan: nil plan node")
	}
	node := &core.ExplainNode{
		ID:       b.nextID(),
		Type:     core.NodeTypeOther,
		Children: []*core.ExplainNode{},
		Metadata: make(map[string]interface{}),
	}
	if nodeType, ok := plan["Node Type"].(string); ok {
		node.Operation = nodeType
		node.Type = mapPlanNodeType(nodeType)
	}
	if v, ok := plan["Relation Name"].(string); ok {
		node.TargetTable = &v
	}
	if v, ok := plan["Index Name"].(string); ok {
		node.IndexName = &v
	}
	node.Condition = getPlanCondition(plan)
	node.SortKey = extractPlanSortKey(plan)
	if v, ok := plan["Total Cost"].(float64); ok {
		node.EstimatedCost = &v
	}
	if v, ok := plan["Plan Rows"].(float64); ok {
		node.EstimatedRows = &v
	}
	childrenRawCacheStats := b.parsePlanChildren(plan, node)
	for key, value := range plan {
		if !isPlanStandardField(key) {
			node.Metadata[key] = value
		}
	}
	computePlanExclusiveCost(node)
	if node.CacheStats != nil {
		node.CacheStats = computePlanExclusiveCacheStats(node.CacheStats, childrenRawCacheStats)
	}
	return node, nil
}

func getPlanCondition(plan map[string]interface{}) *string {
	for _, key := range []string{"Filter", "Hash Cond", "Merge Cond"} {
		if v, ok := plan[key].(string); ok {
			return &v
		}
	}
	return nil
}

func extractPlanSortKey(plan map[string]interface{}) []string {
	sortKey, ok := plan["Sort Key"]
	if !ok {
		return nil
	}
	switch v := sortKey.(type) {
	case string:
		return []string{v}
	case []interface{}:
		var keys []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				keys = append(keys, str)
			}
		}
		return keys
	}
	return nil
}

func (b *pgPlanBuilder) parsePlanChildren(plan map[string]interface{}, node *core.ExplainNode) []*core.CacheStats {
	plans, ok := plan["Plans"].([]interface{})
	if !ok {
		return nil
	}
	childrenRawStats := make([]*core.CacheStats, 0, len(plans))
	for _, rawChild := range plans {
		childMap, ok := rawChild.(map[string]interface{})
		if !ok {
			continue
		}
		childrenRawStats = append(childrenRawStats, extractPlanCacheStats(childMap))
		childNode, err := b.parseNode(childMap)
		if err == nil {
			node.Children = append(node.Children, childNode)
		}
	}
	return childrenRawStats
}

func computePlanExclusiveCost(node *core.ExplainNode) {
	if node.EstimatedCost == nil {
		return
	}
	var childrenTotal float64
	for _, child := range node.Children {
		if child.EstimatedCost != nil {
			childrenTotal += *child.EstimatedCost
		}
	}
	exclusiveCost := *node.EstimatedCost - childrenTotal
	if exclusiveCost > 0 {
		node.ExclusiveCost = &exclusiveCost
	} else {
		zero := 0.0
		node.ExclusiveCost = &zero
	}
}

func normalizeRawPlan(raw interface{}) ([]byte, error) {
	switch v := raw.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case json.RawMessage:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("postgresql plan: unexpected payload type %T", raw)
	}
}

func mapPlanNodeType(nodeType string) core.NodeType {
	lower := strings.ToLower(nodeType)
	switch {
	case strings.Contains(lower, "scan"):
		return core.NodeTypeScan
	case strings.Contains(lower, "join"):
		return core.NodeTypeJoin
	case strings.Contains(lower, "aggregate"):
		return core.NodeTypeAggregate
	case strings.Contains(lower, "sort"):
		return core.NodeTypeSort
	case strings.Contains(lower, "filter"):
		return core.NodeTypeFilter
	case strings.Contains(lower, "subquery"):
		return core.NodeTypeSubquery
	case strings.Contains(lower, "insert"):
		return core.NodeTypeInsert
	case strings.Contains(lower, "update"):
		return core.NodeTypeUpdate
	case strings.Contains(lower, "delete"):
		return core.NodeTypeDelete
	default:
		return core.NodeTypeOther
	}
}

func extractPlanCacheStats(plan map[string]interface{}) *core.CacheStats {
	var stats core.CacheStats
	cacheFields := map[string]**float64{
		"Shared Hit Blocks":     &stats.SharedHitBlocks,
		"Shared Read Blocks":    &stats.SharedReadBlocks,
		"Shared Dirtied Blocks": &stats.SharedDirtiedBlocks,
		"Shared Written Blocks": &stats.SharedWrittenBlocks,
		"Temp Read Blocks":      &stats.TempReadBlocks,
		"Temp Written Blocks":   &stats.TempWriteBlocks,
		"Local Hit Blocks":      &stats.LocalHitBlocks,
		"Local Read Blocks":     &stats.LocalReadBlocks,
		"Local Dirtied Blocks":  &stats.LocalDirtiedBlocks,
		"Local Written Blocks":  &stats.LocalWrittenBlocks,
	}
	hasStats := false
	for key, ptr := range cacheFields {
		if val, ok := plan[key].(float64); ok {
			*ptr = &val
			hasStats = true
		}
	}
	if hasStats {
		return &stats
	}
	return nil
}

func computePlanExclusiveCacheStats(parentStats *core.CacheStats, childrenRawStats []*core.CacheStats) *core.CacheStats {
	if parentStats == nil || len(childrenRawStats) == 0 {
		return parentStats
	}
	type fieldAccessor struct {
		parent **float64
		getter func(*core.CacheStats) *float64
	}
	exclusive := &core.CacheStats{}
	fields := []fieldAccessor{
		{&exclusive.SharedHitBlocks, func(s *core.CacheStats) *float64 { return s.SharedHitBlocks }},
		{&exclusive.SharedReadBlocks, func(s *core.CacheStats) *float64 { return s.SharedReadBlocks }},
		{&exclusive.SharedDirtiedBlocks, func(s *core.CacheStats) *float64 { return s.SharedDirtiedBlocks }},
		{&exclusive.SharedWrittenBlocks, func(s *core.CacheStats) *float64 { return s.SharedWrittenBlocks }},
		{&exclusive.TempReadBlocks, func(s *core.CacheStats) *float64 { return s.TempReadBlocks }},
		{&exclusive.TempWriteBlocks, func(s *core.CacheStats) *float64 { return s.TempWriteBlocks }},
		{&exclusive.LocalHitBlocks, func(s *core.CacheStats) *float64 { return s.LocalHitBlocks }},
		{&exclusive.LocalReadBlocks, func(s *core.CacheStats) *float64 { return s.LocalReadBlocks }},
		{&exclusive.LocalDirtiedBlocks, func(s *core.CacheStats) *float64 { return s.LocalDirtiedBlocks }},
		{&exclusive.LocalWrittenBlocks, func(s *core.CacheStats) *float64 { return s.LocalWrittenBlocks }},
	}
	parentFields := []*float64{
		parentStats.SharedHitBlocks, parentStats.SharedReadBlocks, parentStats.SharedDirtiedBlocks,
		parentStats.SharedWrittenBlocks, parentStats.TempReadBlocks, parentStats.TempWriteBlocks,
		parentStats.LocalHitBlocks, parentStats.LocalReadBlocks, parentStats.LocalDirtiedBlocks,
		parentStats.LocalWrittenBlocks,
	}
	const epsilon = 0.0001
	for i, field := range fields {
		parent := parentFields[i]
		if parent == nil {
			continue
		}
		var childrenTotal float64
		for _, childStats := range childrenRawStats {
			if childStats != nil {
				if childVal := field.getter(childStats); childVal != nil {
					childrenTotal += *childVal
				}
			}
		}
		if exclusiveVal := *parent - childrenTotal; exclusiveVal > epsilon {
			*field.parent = &exclusiveVal
		}
	}
	for _, field := range fields {
		if *field.parent != nil {
			return exclusive
		}
	}
	return nil
}

var planStandardFields = map[string]struct{}{
	"Node Type": {}, "Plans": {}, "Relation Name": {}, "Schema": {}, "Alias": {},
	"Filter": {}, "Hash Cond": {}, "Merge Cond": {}, "Index Name": {}, "Sort Key": {},
	"Plan Rows": {}, "Plan Width": {}, "Total Cost": {}, "Startup Cost": {},
	"Actual Rows": {}, "Actual Total Time": {}, "Actual Startup Time": {}, "Actual Loops": {},
	"Rows Removed by Filter": {}, "Rows Removed by Index Recheck": {}, "Peak Memory Usage": {},
	"Shared Hit Blocks": {}, "Shared Read Blocks": {}, "Shared Dirtied Blocks": {}, "Shared Written Blocks": {},
	"Temp Read Blocks": {}, "Temp Write Blocks": {}, "Local Hit Blocks": {}, "Local Read Blocks": {},
	"Local Dirtied Blocks": {}, "Local Written Blocks": {},
}

func isPlanStandardField(key string) bool {
	_, ok := planStandardFields[key]
	return ok
}
