package sqllang

import (
	"context"
	"fmt"

	"selectDb/backend/utils"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

// CompleteResult is the completion response returned to the caller.
type CompleteResult struct {
	ID         string                `json:"id"`
	Candidates []CompletionCandidate `json:"candidates"`
	Errors     []string              `json:"errors"`
}

// CompletionCandidate wraps a core.Candidate with a Monaco-specific kind integer.
type CompletionCandidate struct {
	core.Candidate
	Kind int `json:"kind"` // Monaco completion item kind
}

// Complete returns SQL completion candidates for the identifier at the cursor.
func (s *SqlLang) Complete(p PositionParams) CompleteResult {
	id, err := utils.GenerateRandomID(4)
	if err != nil {
		return CompleteResult{Errors: []string{"failed to generate result id"}}
	}

	dbInstance := s.graph.GetDBInstanceNodeByID(p.DbInstanceID)
	if dbInstance == nil {
		return CompleteResult{ID: id, Errors: []string{fmt.Sprintf("DB instance not found: %s", p.DbInstanceID)}}
	}
	meta, err := s.getMeta(dbInstance, false)
	if err != nil {
		return CompleteResult{ID: id, Errors: []string{fmt.Sprintf("failed to get metadata: %v", err)}}
	}

	sql, line, col := s.resolveEditorPosition(p)

	d := engine.GetDialect(dbInstance.DBType)
	if d == nil {
		return CompleteResult{ID: id, Errors: []string{fmt.Sprintf("unsupported DB type for completion: %s", dbInstance.DBType)}}
	}

	if s.lintRunner != nil && s.lintRunner.Analyzer != nil {
		d.SetAnalyzer(s.lintRunner.Analyzer)
	}

	candidates, err := d.Complete(context.Background(), sql, line, col, *meta)
	if err != nil {
		return CompleteResult{ID: id, Errors: []string{fmt.Sprintf("completion failed: %v", err)}}
	}

	result := CompleteResult{ID: id, Candidates: make([]CompletionCandidate, len(candidates))}
	for i, c := range candidates {
		result.Candidates[i] = CompletionCandidate{Candidate: c, Kind: candidateTypeToMonacoKind(c.Type)}
	}
	return result
}

// candidateTypeToMonacoKind converts core.CandidateType to Monaco completion item kind.
func candidateTypeToMonacoKind(ct core.CandidateType) int {
	switch ct {
	case core.CandidateTypeKeyword:
		return 14 // Keyword
	case core.CandidateTypeFunction:
		return 1 // Function
	case core.CandidateTypeSchema:
		return 2 // Module
	case core.CandidateTypeTable:
		return 7 // Class
	case core.CandidateTypeForeignTable:
		return 7 // Class
	case core.CandidateTypeView:
		return 11 // Interface
	case core.CandidateTypeMaterializedView:
		return 11 // Interface
	case core.CandidateTypeColumn:
		return 5 // Field
	case core.CandidateTypeType:
		return 7 // Class / type
	case core.CandidateTypeEnumValue:
		return 16 // EnumMember
	default:
		return 0 // Text
	}
}
