package query

import (
	"context"
	"fmt"
	"strings"
)

// Get fetches one row by id — workspace-scoped and soft-delete-filtered — as a
// map keyed by API field name (same shape as a List row). found is false when no
// such live row exists in the workspace, so a cross-tenant or deleted id reads
// as "not found" (the handler's 404) rather than leaking existence.
func Get(ctx context.Context, q Querier, res Resource, workspaceID, id string) (map[string]any, bool, error) {
	pkCol, pkKind, err := res.pk()
	if err != nil {
		return nil, false, err
	}
	sqlText := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s IS NULL AND %s = $2::%s LIMIT 1",
		strings.Join(res.columns(), ", "), res.Table, tenantColumn, softDeleteColumn, pkCol, sqlType(pkKind))
	rows, err := q.QueryContext(ctx, sqlText, workspaceID, id)
	if err != nil {
		return nil, false, err
	}
	data, err := scanRows(rows, res.Fields)
	if err != nil {
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, nil
	}
	return data[0], true, nil
}

func (r Resource) columns() []string {
	cols := make([]string, len(r.Fields))
	for i, f := range r.Fields {
		cols[i] = f.Column
	}
	return cols
}

func (r Resource) pk() (col string, kind Kind, err error) {
	for _, f := range r.Fields {
		if f.Name == r.PK {
			return f.Column, f.Kind, nil
		}
	}
	return "", 0, fmt.Errorf("query: resource %s has no primary-key field %q", r.Table, r.PK)
}
