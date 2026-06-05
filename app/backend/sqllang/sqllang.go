package sqllang

import (
	"selectDb/backend/db/generated"
	"selectDb/backend/graph"

	core "github.com/selectDb/dialect/core"
	ta "github.com/selectDb/dialect/core/tokenanalyzer"
)

type GetMetadataFunc func(instance *graph.DBInstanceNode, noCache bool) (*core.Metadata, error)
type InspectFunc func(statement string, dbInstance *graph.DBInstanceNode) []core.InspectStatement

type SqlLang struct {
	graph      *graph.Graph
	queries    *generated.Queries
	getMeta    GetMetadataFunc
	inspect    InspectFunc
	lintRunner *ta.LintRunner
}

func New(g *graph.Graph, queries *generated.Queries, getMeta GetMetadataFunc, inspect InspectFunc) *SqlLang {
	runner := ta.NewLintRunner()
	if execPath, args, ok := resolveAnalyzer(); ok {
		runner.Analyzer = ta.NewAnalyzer(execPath, args...)
	}

	return &SqlLang{
		graph:      g,
		queries:    queries,
		getMeta:    getMeta,
		inspect:    inspect,
		lintRunner: runner,
	}
}

func (s *SqlLang) getAnalyzer() core.Analyzer {
	if s.lintRunner != nil && s.lintRunner.Analyzer != nil {
		return s.lintRunner.Analyzer
	}
	return nil
}

func (s *SqlLang) Close() {
	if s.lintRunner != nil && s.lintRunner.Analyzer != nil {
		s.lintRunner.Analyzer.Close()
	}
}
