package sqllang

import (
	"fmt"
	"strings"
	"unicode"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/engine"
)

// HoverResult contains markdown for the hovered identifier.
type HoverResult struct {
	Markdown string `json:"markdown"`
}

// hoverVariable checks whether the cursor is on a $variable token in the raw SQL
// and returns hover markdown for it (env var value or SQL file preview), or "".
func (s *SqlLang) hoverVariable(sql, fileID string, line1, col0 int) string {
	lines := strings.Split(sql, "\n")
	if line1 < 1 || line1 > len(lines) {
		return ""
	}
	lineStr := lines[line1-1]
	matches := varPattern.FindAllStringSubmatchIndex(lineStr, -1)
	var varName string
	for _, m := range matches {
		if col0 >= m[0] && col0 < m[1] {
			varName = lineStr[m[2]:m[3]]
			break
		}
	}
	if varName == "" {
		return ""
	}

	envVars, err := s.graph.GetUriVariables(fileID)
	if err == nil {
		for _, v := range envVars {
			if v.Name == varName {
				value := v.Value
				if len(value) > 300 {
					value = value[:300] + "..."
				}
				if value == "" {
					value = "(empty)"
				}
				parts := []string{
					fmt.Sprintf("**Environment variable** `$%s`", v.Name),
					fmt.Sprintf("`%s`", value),
				}
				if v.Source != "" {
					parts = append(parts, fmt.Sprintf("Source: `%s`", v.Source))
				}
				return strings.Join(parts, "\n\n")
			}
		}
	}

	fileRefs, err := s.graph.GetUriSqlFileRefs(fileID)
	if err == nil {
		for _, f := range fileRefs {
			if f.Name == varName {
				header := fmt.Sprintf("**SQL file** `%s`", f.Path)
				if f.Path == "" {
					header = fmt.Sprintf("**SQL file** `%s`", f.URI)
				}
				if preview := strings.TrimSpace(f.Preview); preview != "" {
					return header + "\n```sql\n" + preview + "\n```"
				}
				return header
			}
		}
	}

	return ""
}

// Hover returns brief markdown for the identifier at the cursor.
func (s *SqlLang) Hover(p PositionParams) HoverResult {
	// Variable hover operates on raw SQL before any substitution.
	if p.FileID != "" {
		if md := s.hoverVariable(p.SQL, p.FileID, p.Line, p.Column); md != "" {
			return HoverResult{Markdown: md}
		}
	}

	dbInstance := s.graph.GetDBInstanceNodeByID(p.DbInstanceID)
	if dbInstance == nil {
		return HoverResult{}
	}
	meta, err := s.getMeta(dbInstance, false)
	if err != nil {
		return HoverResult{}
	}
	sql, line, col := s.resolveEditorPosition(p)
	d := engine.GetDialect(dbInstance.DBType)

	qual := identChainAt(sql, line, col)
	if pos := cursorChainPos(sql, line, col); pos >= 0 && pos < len(qual)-1 {
		qual = qual[:pos+1]
	}

	// For CTE/subquery aliases (single token), prefer showing the virtual table
	// columns over resolving through to the physical backing table.
	if d != nil && len(qual) == 1 {
		_, vtabs, idx := sqlRefsAtCaret(d, sql, meta, line, col, s.getAnalyzer())
		norm := d.NormalizeIdentifier
		if vt := coreRefs.VirtualTableForQualifier(vtabs, qual[0], idx, norm); vt != nil {
			return HoverResult{
				Markdown: sqlHoverMarkdownColumnList(fmt.Sprintf("**%s** (virtual)", vt.Table), vt.Columns),
			}
		}
	}

	// Physical resolution.
	if obj := resolveCore(meta, d, sql, line, col, s.getAnalyzer()); obj != nil {
		return HoverResult{Markdown: renderHoverMarkdown(meta, d, obj)}
	}

	// Virtual table fallback for qualified identifiers (e.g. cte.column).
	if d != nil && len(qual) == 2 {
		refs, vtabs, idx := sqlRefsAtCaret(d, sql, meta, line, col, s.getAnalyzer())
		norm := d.NormalizeIdentifier
		normEq := func(x, y string) bool { return norm(x) == norm(y) }
		a, b := qual[0], qual[1]
		if vt := virtualTableForQualifier(vtabs, refs, a, idx, norm); vt != nil {
			if c := findColumnInTable(vt.Columns, b, normEq); c != nil {
				title := fmt.Sprintf("**%s.%s** (virtual)", vt.Table, c.Name)
				var sec []string
				if strings.TrimSpace(c.Type) != "" {
					sec = append(sec, fmt.Sprintf("`%s`", c.Type))
				}
				if c.Nullable {
					sec = append(sec, "nullable")
				}
				if len(sec) == 0 {
					return HoverResult{Markdown: sqlHoverSpaced(title, "_virtual column_")}
				}
				return HoverResult{Markdown: sqlHoverSpaced(title, sec...)}
			}
		}
	}

	return HoverResult{}
}

// renderHoverMarkdown formats a resolved object as hover markdown.
// It performs a second in-memory metadata lookup to get the full object details.
func renderHoverMarkdown(meta *core.Metadata, d core.SQLDialect, obj *resolvedObject) string {
	normEq := func(x, y string) bool { return strings.EqualFold(x, y) }
	if d != nil {
		norm := d.NormalizeIdentifier
		normEq = func(x, y string) bool { return norm(x) == norm(y) }
	}
	switch obj.Kind {

	case "column":
		_, tab, _ := findRelationWithSchema(meta, obj.Schema, obj.Rel)
		if tab == nil {
			return ""
		}
		col := findColumnInTable(tab.Columns, obj.Col, normEq)
		if col == nil {
			return ""
		}

		nullable := ""
		if col.Nullable {
			nullable = "*Nullable*"
		}

		return sqlHoverSpaced(
			fmt.Sprintf("**%s.%s**", obj.Rel, col.Name),
			fmt.Sprintf("`%s` %s", col.Type, nullable),
		)

	case "table", "view":
		_, tab, _ := findRelationWithSchema(meta, obj.Schema, obj.Rel)
		if tab == nil {
			return ""
		}
		label := "Table"
		if obj.Kind == "view" {
			label = "View"
		}
		return sqlHoverMarkdownColumnList(fmt.Sprintf("**%s** `%s.%s`", label, obj.Schema, obj.Rel), tab.Columns)
	case "function":
		for _, f := range meta.AllFunctions() {
			if (obj.OID > 0 && f.OID == obj.OID) || strings.EqualFold(f.Name, obj.Rel) {
				return sqlHoverMarkdownFunction(f)
			}
		}
	case "type":
		for _, t := range meta.AllTypes() {
			if strings.EqualFold(t.Name, obj.Rel) {
				disp := t.Display
				if disp == "" {
					disp = t.Name
				}
				parts := []string{}
				if desc := strings.TrimSpace(t.Description); desc != "" {
					parts = append(parts, "\n\n---\n\n*"+desc+"*\n\n---\n\n")
				}
				parts = append(parts, fmt.Sprintf("_%s_", t.Kind))
				if len(t.EnumLabels) > 0 {
					parts = append(parts, strings.Join(t.EnumLabels, ", "))
				}
				return sqlHoverSpaced(fmt.Sprintf("**Type** `%s`", disp), parts...)
			}
		}
	}
	return ""
}

const sqlHoverSectionBreak = "\n\n"

func sqlHoverSpaced(title string, parts ...string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(title))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString(sqlHoverSectionBreak)
		b.WriteString(p)
	}
	return b.String()
}

func sqlHoverMarkdownTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func sqlHoverMarkdownFunction(f core.Function) string {
	mdEsc := func(s string) string { return strings.ReplaceAll(s, "`", "'") }
	args := strings.TrimSpace(f.Args)
	res := strings.TrimSpace(f.Result)
	if res == "" {
		res = "?"
	}
	argsE := mdEsc(args)
	resE := mdEsc(res)
	paramVal := "()"
	if argsE != "" {
		paramVal = "(" + argsE + ")"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**%s**(%s) → `%s`", mdEsc(f.Name), argsE, resE)
	b.WriteString(sqlHoverSectionBreak)
	if desc := strings.TrimSpace(f.Description); desc != "" {
		fmt.Fprintf(&b, "\n\n---\n\n*%s* \n\n---\n\n", desc)
	}
	fmt.Fprintf(&b, "**Parameters**, `%s`  \n\n", paramVal)
	fmt.Fprintf(&b, "**Returns**, `%s`", resE)
	if s := strings.TrimSpace(f.Schema); s != "" {
		fmt.Fprintf(&b, "  \n\n**Schema**, `%s`", mdEsc(s))
	}
	return b.String()
}

func sqlHoverMarkdownColumnList(header string, cols []core.Column) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(header))
	b.WriteString(sqlHoverSectionBreak)
	if len(cols) == 0 {
		b.WriteString("_No columns in catalog._")
		return b.String()
	}
	b.WriteString("| column | type |\n")
	b.WriteString("|--------|------|\n")
	for _, col := range cols {
		typ := strings.TrimSpace(col.Type)
		if typ == "" {
			typ = "?"
		}
		if col.Nullable {
			typ += " nullable"
		}
		fmt.Fprintf(&b, "| %s | %s |\n",
			sqlHoverMarkdownTableCell(col.Name),
			sqlHoverMarkdownTableCell(typ))
	}
	return b.String()
}

// quotedIdentPairAt handles the case where the cursor is inside a double-quoted
// identifier, e.g. `mytable."my col"` → ["mytable", "my col"].
func quotedIdentPairAt(line string, col0 int) []string {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return nil
	}
	if col0 >= n {
		col0 = n - 1
	}
	if col0 < 0 {
		col0 = 0
	}
	if runes[col0] == '"' {
		return nil
	}
	open := -1
	for i := col0; i >= 0; i-- {
		if runes[i] == '"' {
			open = i
			break
		}
	}
	if open < 0 {
		return nil
	}
	closeIdx := -1
	for j := open + 1; j < n; j++ {
		if runes[j] == '"' {
			if j+1 < n && runes[j+1] == '"' {
				j++
				continue
			}
			closeIdx = j
			break
		}
	}
	if closeIdx < 0 || col0 <= open || col0 >= closeIdx {
		return nil
	}
	inner := strings.ReplaceAll(string(runes[open+1:closeIdx]), `""`, `"`)
	k := open - 1
	for k >= 0 && unicode.IsSpace(runes[k]) {
		k--
	}
	if k < 0 || runes[k] != '.' {
		return []string{inner}
	}
	k--
	for k >= 0 && unicode.IsSpace(runes[k]) {
		k--
	}
	if k < 0 || !core.IsIdentifierChar(runes[k]) {
		return nil
	}
	qEnd := k
	for qEnd >= 0 && core.IsIdentifierChar(runes[qEnd]) {
		qEnd--
	}
	qual := string(runes[qEnd+1 : k+1])
	if qual == "" {
		return nil
	}
	return []string{qual, inner}
}

// cursorChainPos returns the 0-based index of the dot-chain token the cursor
// (col0) sits on, e.g. for "aa.id" with cursor on "aa" → 0, on "id" → 1.
// Returns -1 if the position cannot be determined.
func cursorChainPos(sql string, line1, col0 int) int {
	lines := strings.Split(sql, "\n")
	if line1 < 1 || line1 > len(lines) {
		return -1
	}
	runes := []rune(lines[line1-1])
	n := len(runes)
	if n == 0 {
		return -1
	}
	if col0 < 0 {
		col0 = 0
	}
	if col0 >= n {
		col0 = n - 1
	}
	i := col0
	for i > 0 && !core.IsIdentifierChar(runes[i]) && runes[i] != '.' {
		i--
	}
	if !core.IsIdentifierChar(runes[i]) && runes[i] != '.' {
		return -1
	}
	start := i
	for start > 0 && (core.IsIdentifierChar(runes[start-1]) || runes[start-1] == '.' || runes[start-1] == '"') {
		start--
	}
	dots := 0
	for j := start; j < i; j++ {
		if runes[j] == '.' {
			dots++
		}
	}
	return dots
}

// identChainAt resolves the dot-separated identifier chain the cursor sits on,
// e.g. `schema.table.column` → ["schema", "table", "column"].
func identChainAt(sql string, line1, col0 int) []string {
	lines := strings.Split(sql, "\n")
	if line1 < 1 || line1 > len(lines) {
		return nil
	}
	s := lines[line1-1]
	if q := quotedIdentPairAt(s, col0); len(q) >= 1 {
		return q
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}
	if col0 < 0 {
		col0 = 0
	}
	if col0 > len(runes) {
		col0 = len(runes)
	}
	i := col0
	if i >= len(runes) {
		i = len(runes) - 1
	}
	for i > 0 && !core.IsIdentifierChar(runes[i]) && runes[i] != '.' {
		i--
	}
	if i < len(runes) && !core.IsIdentifierChar(runes[i]) && runes[i] != '.' {
		return nil
	}
	start, end := i, i
	for start > 0 && (core.IsIdentifierChar(runes[start-1]) || runes[start-1] == '.') {
		start--
	}
	for end < len(runes)-1 && (core.IsIdentifierChar(runes[end+1]) || runes[end+1] == '.') {
		end++
	}
	frag := strings.TrimSpace(string(runes[start : end+1]))
	if frag == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(frag, ".") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func findColumnInTable(cols []core.Column, name string, eq func(string, string) bool) *core.Column {
	for i := range cols {
		if eq(cols[i].Name, name) {
			return &cols[i]
		}
	}
	return nil
}

func sqlRefsAtCaret(d core.SQLDialect, sql string, meta *core.Metadata, line, col int, analyzer ...core.Analyzer) ([]core.RelationRef, []core.RelationRef, int) {
	if d == nil || meta == nil || sql == "" {
		return nil, nil, 0
	}

	if len(analyzer) > 0 && analyzer[0] != nil {
		refs, vtabs, err := coreRefs.CollectReferencesFromPython(analyzer[0], sql, d, *meta)
		if err == nil {
			return refs, vtabs, coreRefs.CaretCharOffset(sql, line, col)
		}
	}

	return nil, nil, 0
}
