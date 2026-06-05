package sqllang

import (
	"fmt"
	"strings"

	"selectDb/backend/graph"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/engine"
)

// ResolveResult is returned by Resolve for ItemInfoModal navigation.
type ResolveResult struct {
	Node  *graph.DBInstanceItemNode `json:"node,omitempty"`
	Found bool                      `json:"found"`
	Kind  string                    `json:"kind"`
}

// resolvedObject is the internal intermediate result of identifier resolution.
type resolvedObject struct {
	Schema string // canonical schema name
	Rel    string // table, view, function, or type name (canonical)
	Col    string // column name (empty for non-column results)
	Kind   string // "table", "view", "column", "function", "type"
	IsView bool   // true when Kind=="column" and the column is in a view
	OID    int64  // function OID (only for Kind=="function")
}

// Resolve returns the graph node ID for the identifier at the cursor.
func (s *SqlLang) Resolve(p PositionParams) ResolveResult {
	dbInstance := s.graph.GetDBInstanceNodeByID(p.DbInstanceID)
	if dbInstance == nil {
		return ResolveResult{}
	}

	meta, err := s.getMeta(dbInstance, false)
	if err != nil {
		return ResolveResult{}
	}

	d := engine.GetDialect(dbInstance.DBType)
	sql, line, col := s.resolveEditorPosition(p)
	obj := resolveCore(meta, d, sql, line, col)
	if obj == nil {
		return ResolveResult{}
	}

	nodeID := buildNodeID(dbInstance.ID, meta, obj)
	node := s.graph.FindDbItemNodeById(dbInstance.ID, nodeID)
	if node == nil {
		return ResolveResult{}
	}

	return ResolveResult{
		Node:  node,
		Found: true,
		Kind:  obj.Kind,
	}
}

func resolveCore(meta *core.Metadata, d core.SQLDialect, sql string, line, col int, analyzer ...core.Analyzer) *resolvedObject {
	qual := identChainAt(sql, line, col)
	if pos := cursorChainPos(sql, line, col); pos >= 0 && pos < len(qual)-1 {
		qual = qual[:pos+1]
	}

	if len(qual) >= 3 {
		sch, tbl, c := qual[len(qual)-3], qual[len(qual)-2], qual[len(qual)-1]
		if actualSch, tab, isView := findRelationWithSchema(meta, sch, tbl); tab != nil {
			if column := findColumnInTable(tab.Columns, c, strings.EqualFold); column != nil {
				return &resolvedObject{Schema: actualSch, Rel: tab.Name, Col: column.Name, Kind: "column", IsView: isView}
			}
		}
	}

	if len(qual) == 2 {
		a, b := qual[0], qual[1]
		if d != nil {
			refs, vtabs, idx := sqlRefsAtCaret(d, sql, meta, line, col, analyzer...)
			norm := d.NormalizeIdentifier
			normEq := func(x, y string) bool { return norm(x) == norm(y) }

			if r := coreRefs.RelationRefForQualifier(refs, a, idx, norm); r != nil {
				sch, tbl := r.Schema, r.Table
				if sch == "" {
					sch = core.GetDefaultSchema(*meta)
				}
				if actualSch, tab, isView := findRelationWithSchema(meta, sch, tbl); tab != nil {
					if c := findColumnInTable(tab.Columns, b, normEq); c != nil {
						return &resolvedObject{Schema: actualSch, Rel: tab.Name, Col: c.Name, Kind: "column", IsView: isView}
					}
				}
			}

			if vt := virtualTableForQualifier(vtabs, refs, a, idx, norm); vt != nil && vt.ScopeStartPos > 0 {
				if findColumnInTable(vt.Columns, b, normEq) != nil {
					if obj := cteColumnPhysical(meta, vt, refs, b, norm); obj != nil {
						return obj
					}
				}
			}
		}

		if actualSch, tab, isView := findRelationWithSchema(meta, meta.DefaultSchema, a); tab != nil {
			if c := findColumnInTable(tab.Columns, b, strings.EqualFold); c != nil {
				return &resolvedObject{Schema: actualSch, Rel: tab.Name, Col: c.Name, Kind: "column", IsView: isView}
			}
		}

		if actualSch, tab, isView := findRelationWithSchema(meta, a, b); tab != nil {
			kind := "table"
			if isView {
				kind = "view"
			}
			_ = actualSch
			return &resolvedObject{Schema: actualSch, Rel: tab.Name, Kind: kind, IsView: isView}
		}
	}

	if len(qual) == 1 {
		w := qual[0]
		if w == "" {
			return nil
		}
		if d != nil {
			refs, _, idx := sqlRefsAtCaret(d, sql, meta, line, col, analyzer...)
			norm := d.NormalizeIdentifier
			if r := coreRefs.RelationRefForSingleIdent(refs, w, idx, norm); r != nil {
				sch, tbl := r.Schema, r.Table
				if sch == "" {
					sch = core.GetDefaultSchema(*meta)
				}

				if actualSch, tab, isView := findRelationWithSchema(meta, sch, tbl); tab != nil {
					kind := "table"
					if isView {
						kind = "view"
					}
					return &resolvedObject{Schema: actualSch, Rel: tab.Name, Kind: kind}
				}
			}

			// Unqualified column: scan all in-scope FROM refs for a column with this name.
			// Only resolve when exactly one ref owns it to avoid ambiguity.
			normEq := func(x, y string) bool { return norm(x) == norm(y) }
			var colMatch *resolvedObject
			ambiguous := false
			for i := range refs {
				r := &refs[i]
				if !core.ScopeContainsCaret(r.ScopeStartPos, r.ScopeEndPos, idx) {
					continue
				}
				sch := r.Schema
				if sch == "" {
					sch = core.GetDefaultSchema(*meta)
				}
				if actualSch, tab, isView := findRelationWithSchema(meta, sch, r.Table); tab != nil {
					if c := findColumnInTable(tab.Columns, w, normEq); c != nil {
						if colMatch != nil {
							ambiguous = true
							break
						}
						colMatch = &resolvedObject{Schema: actualSch, Rel: tab.Name, Col: c.Name, Kind: "column", IsView: isView}
					}
				}
			}
			if !ambiguous && colMatch != nil {
				return colMatch
			}
		}
		if f := pickFunctionByName(meta, w); f != nil {
			return &resolvedObject{Schema: f.Schema, Rel: f.Name, OID: f.OID, Kind: "function"}
		}
		for _, t := range meta.AllTypes() {
			if strings.EqualFold(t.Name, w) {
				return &resolvedObject{Schema: t.Schema, Rel: t.Name, Kind: "type"}
			}
		}
		for _, sch := range meta.Schemas {
			for _, tab := range sch.Tables {
				if strings.EqualFold(tab.Name, w) {
					return &resolvedObject{Schema: sch.Name, Rel: tab.Name, Kind: "table"}
				}
			}
			for _, tab := range sch.Views {
				if strings.EqualFold(tab.Name, w) {
					return &resolvedObject{Schema: sch.Name, Rel: tab.Name, Kind: "view"}
				}
			}
		}
	}
	return nil
}

// buildNodeID constructs the graph node ID matching the format used by db_client/schema.go.
func buildNodeID(dbInstanceID string, meta *core.Metadata, obj *resolvedObject) string {
	prefix := nodeSchemaPrefix(dbInstanceID, meta, obj.Schema)
	switch obj.Kind {
	case "table":
		return fmt.Sprintf("%s:table:%s", prefix, obj.Rel)
	case "view":
		return fmt.Sprintf("%s:view:%s", prefix, obj.Rel)
	case "column":
		rel := "table"
		if obj.IsView {
			rel = "view"
		}
		return fmt.Sprintf("%s:%s:%s:column:%s", prefix, rel, obj.Rel, obj.Col)
	case "function":
		return fmt.Sprintf("%s:function:%d", prefix, obj.OID)
	case "type":
		return fmt.Sprintf("%s:type:%s", prefix, obj.Rel)
	}
	return ""
}

// nodeSchemaPrefix returns the graph id prefix for objects under a schema (…:schema:<name>).
func nodeSchemaPrefix(dbInstanceID string, meta *core.Metadata, schema string) string {
	for _, s := range meta.Schemas {
		if strings.EqualFold(s.Name, schema) {
			return fmt.Sprintf("%s:schema:%s", dbInstanceID, s.Name)
		}
	}
	def := core.GetDefaultSchema(*meta)
	return fmt.Sprintf("%s:schema:%s", dbInstanceID, def)
}

func findRelationWithSchema(meta *core.Metadata, schema, relation string) (schemaName string, tab *core.Table, isView bool) {
	for i := range meta.Schemas {
		if !strings.EqualFold(meta.Schemas[i].Name, schema) {
			continue
		}
		s := &meta.Schemas[i]
		for j := range s.Tables {
			if strings.EqualFold(s.Tables[j].Name, relation) {
				return s.Name, &s.Tables[j], false
			}
		}
		for j := range s.Views {
			if strings.EqualFold(s.Views[j].Name, relation) {
				return s.Name, &s.Views[j], true
			}
		}
	}
	return "", nil, false
}

func pickFunctionByName(meta *core.Metadata, w string) *core.Function {
	var first *core.Function
	for _, f := range meta.AllFunctions() {
		if !strings.EqualFold(f.Name, w) {
			continue
		}
		if first == nil {
			first = &f
		}
		if strings.EqualFold(f.Schema, "pg_catalog") {
			return &f
		}
	}
	return first
}

// virtualTableForQualifier resolves a virtual table for the given qualifier,
// first by direct CTE name match, then by alias translation through physical refs.
// This handles queries like `FROM accounts aa ON aa."col"` where the vtab is named
// "accounts" but the qualifier in the query is the alias "aa".
func virtualTableForQualifier(vtabs, refs []core.RelationRef, qualifier string, caretIdx int, norm func(string) string) *core.RelationRef {
	if vt := coreRefs.VirtualTableForQualifier(vtabs, qualifier, caretIdx, norm); vt != nil {
		return vt
	}
	// Try alias → CTE name translation via physical refs.
	if r := coreRefs.RelationRefForQualifier(refs, qualifier, caretIdx, norm); r != nil {
		return coreRefs.VirtualTableForQualifier(vtabs, r.Table, caretIdx, norm)
	}
	return nil
}

func cteColumnPhysical(meta *core.Metadata, vt *core.RelationRef, refs []core.RelationRef, colName string, norm func(string) string) *resolvedObject {
	if vt == nil || vt.ScopeStartPos <= 0 || norm == nil {
		return nil
	}
	cteBodyClose := vt.ScopeStartPos - 1
	normEq := func(a, b string) bool { return norm(a) == norm(b) }
	type schTbl struct{ sch, tbl string }
	byKey := make(map[string]schTbl)
	for i := range refs {
		r := &refs[i]
		if r.ScopeEndPos != cteBodyClose {
			continue
		}
		sch, tbl := r.Schema, r.Table
		if sch == "" {
			sch = core.GetDefaultSchema(*meta)
		}
		_, tab, _ := findRelationWithSchema(meta, sch, tbl)
		if tab == nil || findColumnInTable(tab.Columns, colName, normEq) == nil {
			continue
		}
		k := strings.ToLower(sch) + "\x00" + strings.ToLower(tbl)
		if _, ok := byKey[k]; ok {
			continue
		}
		byKey[k] = schTbl{sch: sch, tbl: tbl}
	}
	if len(byKey) != 1 {
		return nil
	}
	for _, st := range byKey {
		actualSch, tab, isView := findRelationWithSchema(meta, st.sch, st.tbl)
		if tab == nil {
			return nil
		}
		c := findColumnInTable(tab.Columns, colName, normEq)
		if c == nil {
			return nil
		}
		return &resolvedObject{Schema: actualSch, Rel: tab.Name, Col: c.Name, Kind: "column", IsView: isView}
	}
	return nil
}
