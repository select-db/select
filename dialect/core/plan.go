package core

import (
	"fmt"
	"strings"
	"sync"
)

// PlanParser describes the contract for dialect-specific plan (estimated, non-executing) handlers.
type PlanParser interface {
	Dialect() string
	BuildPlanQuery(query string) string
	Parse(rawPlan interface{}) (*ExplainNode, error)
	GetFormattingConfig() FormattingConfig
}

var (
	planRegistryMu sync.RWMutex
	planParsers    = map[string]func() PlanParser{}
)

// RegisterPlanParser registers a dialect plan parser factory.
func RegisterPlanParser(name string, ctor func() PlanParser) {
	planRegistryMu.Lock()
	defer planRegistryMu.Unlock()
	if name == "" || ctor == nil {
		return
	}
	planParsers[strings.ToLower(name)] = ctor
}

// NewPlanParser retrieves a parser for the requested dialect.
func NewPlanParser(name string) (PlanParser, error) {
	planRegistryMu.RLock()
	defer planRegistryMu.RUnlock()
	ctor, ok := planParsers[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("plan parser not found: %s", name)
	}
	return ctor(), nil
}

// BuildVisualTreeForPlan calculates visualization metadata over a plan (estimated only).
func BuildVisualTreeForPlan(root *ExplainNode, formatter PlanParser) (*ExplainNode, float64) {
	if root == nil {
		return nil, 0
	}
	totalCost := aggregateEstimatedCost(root)
	if totalCost == 0 {
		totalCost = aggregateActualTime(root)
	}
	decoratePlanNode(root, 0, totalCost, formatter)
	return root, totalCost
}

func decoratePlanNode(node *ExplainNode, depth int, total float64, formatter PlanParser) {
	if node == nil {
		return
	}
	node.Depth = depth
	node.Warnings = detectWarnings(node)
	node.PercentOfTotal = calculatePercent(node, total)

	if node.CacheStats != nil && formatter != nil {
		FormatCacheStats(node.CacheStats, formatter.GetFormattingConfig())
	}
	for _, child := range node.Children {
		decoratePlanNode(child, depth+1, total, formatter)
	}
}
