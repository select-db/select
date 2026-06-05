package core

import (
	"fmt"
	"strings"
	"sync"
)

// NodeType represents a normalized operation category in an execution plan.
type NodeType string

const (
	NodeTypeScan      NodeType = "scan"
	NodeTypeJoin      NodeType = "join"
	NodeTypeAggregate NodeType = "aggregate"
	NodeTypeSort      NodeType = "sort"
	NodeTypeFilter    NodeType = "filter"
	NodeTypeSubquery  NodeType = "subquery"
	NodeTypeInsert    NodeType = "insert"
	NodeTypeUpdate    NodeType = "update"
	NodeTypeDelete    NodeType = "delete"
	NodeTypeOther     NodeType = "other"
)

// ExplainNode is the dialect-agnostic representation of a plan node.
type ExplainNode struct {
	ID             string                 `json:"id"`
	Type           NodeType               `json:"type"`
	Operation      string                 `json:"operation"`
	TargetTable    *string                `json:"targetTable,omitempty"`
	IndexName      *string                `json:"indexName,omitempty"`
	Condition      *string                `json:"condition,omitempty"`
	SortKey        []string               `json:"sortKey,omitempty"`
	EstimatedRows  *float64               `json:"estimatedRows,omitempty"`
	EstimatedCost  *float64               `json:"estimatedCost,omitempty"`
	ExclusiveCost  *float64               `json:"exclusiveCost,omitempty"` // Cost excluding children
	ActualRows     *float64               `json:"actualRows,omitempty"`
	ActualTime     *float64               `json:"actualTime,omitempty"`    // Total time (including children)
	ExclusiveTime  *float64               `json:"exclusiveTime,omitempty"` // Time excluding children
	PeakMemory     *float64               `json:"peakMemory,omitempty"`    // Peak memory usage in KB
	Children       []*ExplainNode         `json:"children"`
	Metadata       map[string]interface{} `json:"metadata"`
	RawOutput      *string                `json:"rawOutput,omitempty"`
	Depth          int                    `json:"depth"`
	PercentOfTotal float64                `json:"percentOfTotal"`
	Warnings       []string               `json:"warnings"`
	CacheStats     *CacheStats            `json:"cacheStats,omitempty"`
}

// CacheStats represents cache/buffer statistics from explain plans.
type CacheStats struct {
	SharedHitBlocks    *float64 `json:"sharedHitBlocks,omitempty"`
	SharedReadBlocks   *float64 `json:"sharedReadBlocks,omitempty"`
	SharedDirtiedBlocks *float64 `json:"sharedDirtiedBlocks,omitempty"`
	SharedWrittenBlocks *float64 `json:"sharedWrittenBlocks,omitempty"`
	TempReadBlocks     *float64 `json:"tempReadBlocks,omitempty"`
	TempWriteBlocks    *float64 `json:"tempWriteBlocks,omitempty"`
	LocalHitBlocks     *float64 `json:"localHitBlocks,omitempty"`
	LocalReadBlocks    *float64 `json:"localReadBlocks,omitempty"`
	LocalDirtiedBlocks *float64 `json:"localDirtiedBlocks,omitempty"`
	LocalWrittenBlocks *float64 `json:"localWrittenBlocks,omitempty"`
	// Formatted display strings (computed in backend)
	SharedHitBlocksFormatted    *string `json:"sharedHitBlocksFormatted,omitempty"`
	SharedReadBlocksFormatted   *string `json:"sharedReadBlocksFormatted,omitempty"`
	TempReadBlocksFormatted     *string `json:"tempReadBlocksFormatted,omitempty"`
	TempWriteBlocksFormatted    *string `json:"tempWriteBlocksFormatted,omitempty"`
}

// ExplainRequest contains the input necessary to run an explain.
type ExplainRequest struct {
	Dialect string `json:"dialect"`
	Query   string `json:"query"`
	Analyze bool   `json:"analyze"`
	Format  string `json:"format"`
}

// ExplainResponse is returned to the frontend visualizer.
type ExplainResponse struct {
	Success    bool         `json:"success"`
	Root       *ExplainNode `json:"root,omitempty"`
	TotalCost  *float64     `json:"totalCost,omitempty"`
	Error      *string      `json:"error,omitempty"`
	RawExplain string       `json:"rawExplain,omitempty"`
}

// FormattingConfig contains dialect-specific formatting information for display.
type FormattingConfig struct {
	BlockSizeBytes int    // Block size in bytes (e.g., 8192 for PostgreSQL)
	MemoryUnit     string // Memory unit: "KB", "MB", "GB" (e.g., "KB" for PostgreSQL)
}

// FormatCacheStats formats cache statistics for display using the provided config.
func FormatCacheStats(stats *CacheStats, config FormattingConfig) {
	if stats == nil {
		return
	}
	if stats.SharedHitBlocks != nil {
		formatted := formatBlocksAsBytes(*stats.SharedHitBlocks, config.BlockSizeBytes)
		stats.SharedHitBlocksFormatted = &formatted
	}
	if stats.SharedReadBlocks != nil {
		formatted := formatBlocksAsBytes(*stats.SharedReadBlocks, config.BlockSizeBytes)
		stats.SharedReadBlocksFormatted = &formatted
	}
	if stats.TempReadBlocks != nil {
		formatted := formatBlocksAsBytes(*stats.TempReadBlocks, config.BlockSizeBytes)
		stats.TempReadBlocksFormatted = &formatted
	}
	if stats.TempWriteBlocks != nil {
		formatted := formatBlocksAsBytes(*stats.TempWriteBlocks, config.BlockSizeBytes)
		stats.TempWriteBlocksFormatted = &formatted
	}
}

func formatBlocksAsBytes(blocks float64, blockSizeBytes int) string {
	if blocks == 0 {
		return "0"
	}
	bytes := blocks * float64(blockSizeBytes)
	if bytes < 1024 {
		return fmt.Sprintf("%.0f B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", bytes/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", bytes/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", bytes/(1024*1024*1024))
	}
}

// ExplainParser describes the contract for dialect-specific explain handlers.
type ExplainParser interface {
	Dialect() string
	BuildExplainQuery(query string) string
	Parse(rawExplain interface{}) (*ExplainNode, error)
	GetFormattingConfig() FormattingConfig
}

var (
	explainRegistryMu sync.RWMutex
	explainParsers    = map[string]func() ExplainParser{}
)

// RegisterExplainParser registers a dialect explain parser factory.
func RegisterExplainParser(name string, ctor func() ExplainParser) {
	explainRegistryMu.Lock()
	defer explainRegistryMu.Unlock()
	if name == "" || ctor == nil {
		return
	}
	explainParsers[strings.ToLower(name)] = ctor
}

// NewExplainParser retrieves a parser for the requested dialect.
func NewExplainParser(name string) (ExplainParser, error) {
	explainRegistryMu.RLock()
	defer explainRegistryMu.RUnlock()
	ctor, ok := explainParsers[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("explain parser not found: %s", name)
	}
	return ctor(), nil
}

// BuildVisualTree calculates visualization metadata over an explain plan.
func BuildVisualTree(root *ExplainNode, formatter ExplainParser) (*ExplainNode, float64) {
	if root == nil {
		return nil, 0
	}

	totalCost := aggregateEstimatedCost(root)
	if totalCost == 0 {
		// Fallback to actual time totals when cost isn't available.
		totalCost = aggregateActualTime(root)
	}

	decorateExplainNode(root, 0, totalCost, formatter)
	return root, totalCost
}

func aggregateEstimatedCost(node *ExplainNode) float64 {
	if node == nil {
		return 0
	}
	var total float64
	if node.EstimatedCost != nil {
		total += *node.EstimatedCost
	}
	for _, child := range node.Children {
		total += aggregateEstimatedCost(child)
	}
	return total
}

func aggregateActualTime(node *ExplainNode) float64 {
	if node == nil {
		return 0
	}
	var total float64
	if node.ActualTime != nil {
		total += *node.ActualTime
	}
	for _, child := range node.Children {
		total += aggregateActualTime(child)
	}
	return total
}

func decorateExplainNode(node *ExplainNode, depth int, total float64, formatter ExplainParser) {
	if node == nil {
		return
	}
	node.Depth = depth
	node.Warnings = detectWarnings(node)
	node.PercentOfTotal = calculatePercent(node, total)

	// Format cache stats if present
	if node.CacheStats != nil && formatter != nil {
		FormatCacheStats(node.CacheStats, formatter.GetFormattingConfig())
	}

	for _, child := range node.Children {
		decorateExplainNode(child, depth+1, total, formatter)
	}
}

func calculatePercent(node *ExplainNode, total float64) float64 {
	if node == nil || total <= 0 {
		return 0
	}
	if node.EstimatedCost != nil && *node.EstimatedCost > 0 {
		return (*node.EstimatedCost / total) * 100
	}
	if node.ActualTime != nil && *node.ActualTime > 0 {
		return (*node.ActualTime / total) * 100
	}
	return 0
}

func detectWarnings(node *ExplainNode) []string {
	var warnings []string
	if node == nil {
		return warnings
	}

	if node.ActualRows != nil && node.EstimatedRows != nil {
		actual := *node.ActualRows
		estimate := *node.EstimatedRows
		switch {
		case estimate > 0 && actual > estimate*2:
			warnings = append(warnings, "Actual rows significantly higher than estimate.")
		case estimate > 0 && actual < estimate*0.5:
			warnings = append(warnings, "Actual rows significantly lower than estimate.")
		}
	}

	if node.ActualTime != nil && node.EstimatedCost != nil {
		if *node.ActualTime > *node.EstimatedCost*2 {
			warnings = append(warnings, "Actual runtime notably higher than estimated cost.")
		}
	}

	return warnings
}
