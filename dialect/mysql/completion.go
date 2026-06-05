package mysql

import (
	"context"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
)

func (d *Dialect) Complete(
	ctx context.Context,
	sql string,
	caretLine, caretOffset int,
	meta core.Metadata,
) ([]core.Candidate, error) {
	caret := &coreRefs.CompletionCaret{Line: caretLine, Col: caretOffset}

	refs, cteTables, err := coreRefs.CollectReferencesFromPython(
		d.analyzer, sql, d, meta, caret,
	)
	if err != nil {
		return nil, err
	}

	_, columnAliases, err := coreRefs.CollectColumnRefsFromPython(
		d.analyzer, sql, d, meta, caret,
	)
	if err != nil {
		return nil, err
	}

	completionCtx, err := coreRefs.ParseCompletionContextFromPython(
		d.analyzer, sql, caretLine, caretOffset, meta,
	)
	if err != nil {
		return nil, err
	}

	strategy := core.NewCompletionStrategy(d)
	return strategy.CompleteFromSQL(
		completionCtx, sql, caretLine, caretOffset, meta,
		refs, cteTables, columnAliases,
		d.GetReservedKeywords(),
	), nil
}
