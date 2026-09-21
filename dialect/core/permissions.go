package core

import (
	"fmt"
	"strings"
)

const RuleIDPermissionDenied = "permission-denied"

const (
	ActionWorkspaceSettingsWrite = "workspace/settings.write"
	ActionWorkspaceUsersManage   = "workspace/users.manage"
	ActionWorkspaceRolesManage   = "workspace/roles.manage"
	ActionWorkspaceGroupsManage  = "workspace/groups.manage"
	ActionWorkspaceApiKeysManage = "workspace/api-keys.manage"
	ActionWorkspaceAuditRead     = "workspace/audit.read"
)

const (
	ActionManage = "manage"
	ActionSelect = "select"
	ActionInsert = "insert"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionSee    = "see"
)

const MaskedValue = "*****"

// PermissionEntry is a single rule. nil pointer fields mean wildcard.
type PermissionEntry struct {
	DbInstanceID *string
	SchemaName   *string
	TableName    *string
	ColumnName   *string
	Action       string // "select" | "insert" | "update" | "delete" | "see" | "manage"
	Effect       string // "allow" | "deny"
	RoleName     string
}

type PermissionDeniedError struct {
	Action    string
	Schema    string
	Table     string
	Column    string
	RoleName  string // role that denied access, empty if unknown
	StartLine int    // 1-based, 0 = unknown
	StartCol  int    // 0-based
	EndCol    int    // 0-based exclusive
}

func (e *PermissionDeniedError) Error() string {
	var target string
	switch {
	case e.Table == "":
		// A statement we could not resolve names nothing to blame; the
		// connection is what the manage rule is granted on anyway.
		target = "this connection"
	case e.Column != "":
		target = fmt.Sprintf("%s.%s.%s", e.Schema, e.Table, e.Column)
	default:
		target = fmt.Sprintf("%s.%s", e.Schema, e.Table)
	}
	if e.RoleName != "" {
		return fmt.Sprintf("permission denied: %s on %s (role: %s)", e.Action, target, e.RoleName)
	}
	return fmt.Sprintf("permission denied: %s on %s", e.Action, target)
}

type permissionKey struct {
	dbID, schema, table, column, action string
}

// CompiledPermissions is a compiled []PermissionEntry for fast lookups.
type CompiledPermissions struct {
	deny             map[permissionKey]string
	allow            map[permissionKey]string
	managedInstances map[string]bool
	// When set, IsManaged returns true for every DB, forcing the
	// per-statement allow scan even when the role has no rules on a DB
	denyUnmanaged bool
}

func derefWildcard(s *string) string {
	if s == nil || *s == "*" {
		return ""
	}
	return *s
}

func Compile(entries []PermissionEntry) CompiledPermissions {
	idx := CompiledPermissions{
		deny:             make(map[permissionKey]string),
		allow:            make(map[permissionKey]string),
		managedInstances: make(map[string]bool),
	}

	for _, e := range entries {
		dbID := derefWildcard(e.DbInstanceID)
		if e.DbInstanceID != nil {
			idx.managedInstances[dbID] = true
		}

		k := permissionKey{
			dbID,
			derefWildcard(e.SchemaName),
			derefWildcard(e.TableName),
			derefWildcard(e.ColumnName),
			e.Action,
		}
		if e.Effect == "deny" {
			idx.deny[k] = e.RoleName
		} else {
			idx.allow[k] = e.RoleName
		}
	}
	return idx
}

func (idx CompiledPermissions) IsManaged(dbID string) bool {
	return idx.denyUnmanaged || idx.managedInstances[dbID]
}

// WithDenyUnmanaged returns a copy where "no rules on a DB" means deny.
//
// Without it, a database no role has a rule for is unmanaged and every
// statement against it passes: the right default for a local connection the
// developer opened with their own DSN, since refusing there would only be
// refusing them access to their own database. It is the wrong default the
// moment the query runs on our server against our credentials, so everything
// server-side compiles with this set. See authz.CompiledForWorkspace.
func (idx CompiledPermissions) WithDenyUnmanaged() CompiledPermissions {
	idx.denyUnmanaged = true
	return idx
}

func (idx CompiledPermissions) CanManage(dbInstanceID string) bool {
	allowed, _ := idx.manageAllowed(dbInstanceID)
	return allowed
}

// manageAllowed reports whether dbInstanceID may be administrated, and the role
// that decided it. Manage is granted on the connection, so schema, table and
// column are empty here; one lookup is what stops running a statement and
// editing the connection disagreeing about who holds it.
func (idx CompiledPermissions) manageAllowed(dbInstanceID string) (bool, string) {
	return idx.isAllowed(dbInstanceID, "", "", "", ActionManage)
}

// IsAllowed checks a workspace-level action (no db_instance_id)
func (idx CompiledPermissions) IsAllowed(action string) bool {
	allowed, _ := idx.isAllowed("", "", "", "", action)
	return allowed
}

// scan tries all wildcard combinations for a fixed db and action
func (idx CompiledPermissions) scan(m map[permissionKey]string, dbID, schema, table, column, action string) (bool, string) {
	schemas := [2]string{schema, ""}
	tables := [2]string{table, ""}
	cols := [2]string{column, ""}

	for _, s := range schemas {
		for _, t := range tables {
			for _, c := range cols {
				if v, ok := m[permissionKey{dbID, s, t, c, action}]; ok {
					return true, v
				}
			}
		}
	}

	return false, ""
}

func (idx CompiledPermissions) isAllowed(dbID, schema, table, column, action string) (bool, string) {
	denied, role := idx.scan(idx.deny, dbID, schema, table, column, action)
	if denied {
		return false, role
	}

	allowed, role := idx.scan(idx.allow, dbID, schema, table, column, action)
	return allowed, role
}

func CheckQueryPermissions(statements []InspectStatement, dbInstanceID string, compiledPermissions CompiledPermissions) error {
	if !compiledPermissions.IsManaged(dbInstanceID) {
		return nil
	}

	for _, statememt := range statements {
		err := checkStatement(statememt, dbInstanceID, compiledPermissions)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkStatement(stmt InspectStatement, dbInstanceID string, compiledPermissions CompiledPermissions) error {
	action := operationToAction(stmt.Operation)

	var err error
	if action == ActionManage {
		err = checkInstance(stmt, dbInstanceID, compiledPermissions)
	} else {
		err = checkTables(stmt, action, dbInstanceID, compiledPermissions)
	}
	if err != nil {
		return err
	}

	// A CREATE TABLE AS or an INSERT ... SELECT carries its source query here,
	// so holding manage never stands in for the select the source still needs.
	for _, sub := range stmt.Subqueries {
		if err := checkStatement(sub, dbInstanceID, compiledPermissions); err != nil {
			return err
		}
	}
	return nil
}

// checkInstance checks manage, which is granted on the connection rather than
// per table. It is also the only check a statement we could not resolve can
// get: that statement names no table, so a per-table walk would see nothing.
func checkInstance(stmt InspectStatement, dbInstanceID string, compiledPermissions CompiledPermissions) error {
	allowed, role := compiledPermissions.manageAllowed(dbInstanceID)
	if allowed {
		return nil
	}
	denied := &PermissionDeniedError{Action: ActionManage, RoleName: role}
	if len(stmt.Tables) > 0 {
		denied.Schema, denied.Table = stmt.Tables[0].Schema, stmt.Tables[0].Name
	}
	return denied
}

func checkTables(stmt InspectStatement, action, dbInstanceID string, compiledPermissions CompiledPermissions) error {
	for _, table := range stmt.Tables {
		if table.Schema == "" {
			return &PermissionDeniedError{
				Action:    action,
				Schema:    "",
				Table:     table.Name,
				StartLine: table.StartLine,
				StartCol:  table.StartCol,
				EndCol:    table.EndCol,
			}
		}

		named := false
		for _, field := range stmt.Fields {
			if field.Table != table.Name || field.Schema != table.Schema {
				continue
			}
			named = true

			allowed, role := compiledPermissions.isAllowed(dbInstanceID, table.Schema, table.Name, field.Name, action)
			if !allowed {
				startLine, startCol, endCol := field.StartLine, field.StartCol, field.EndCol
				if startLine == 0 {
					// No position on field (e.g. expanded from SELECT *), use table position
					startLine, startCol, endCol = table.StartLine, table.StartCol, table.EndCol
				}

				return &PermissionDeniedError{
					Action:    action,
					Schema:    table.Schema,
					Table:     table.Name,
					Column:    field.Name,
					RoleName:  role,
					StartLine: startLine,
					StartCol:  startCol,
					EndCol:    endCol,
				}
			}
		}
		if named {
			continue
		}

		// A table none of the fields came from is still read: joined for its
		// rows, filtered on in a WHERE. The per-column walk matches nothing for
		// it, so the table as a whole is what there is to ask about.
		allowed, role := compiledPermissions.isAllowed(dbInstanceID, table.Schema, table.Name, "", action)
		if !allowed {
			return &PermissionDeniedError{
				Action:    action,
				Schema:    table.Schema,
				Table:     table.Name,
				RoleName:  role,
				StartLine: table.StartLine,
				StartCol:  table.StartCol,
				EndCol:    table.EndCol,
			}
		}
	}

	return nil
}

// EvaluateSee returns which driver-column positions to mask, or errors when a
// see-denied column cannot be masked.
//
// Matches fields to driverCols by alias or name (case-insensitive). If several
// fields share a name (JOINs), any see-denied one masks the position. The whole
// statement is read, subqueries included: a derived table returns its columns to
// the outer select, so hiding one means finding it wherever it was read.
func EvaluateSee(stmt InspectStatement, driverCols []string, dbInstanceID string, perms CompiledPermissions) ([]int, error) {
	if !perms.IsManaged(dbInstanceID) {
		return nil, nil
	}

	// A write hands rows back through RETURNING, so this cannot be a select's
	// check alone.
	if !ReturnsRows(stmt.Operation) {
		return nil, nil
	}

	if len(driverCols) == 0 {
		return nil, nil
	}

	nested := nestedReadFields(stmt, nil)
	fields := append(append(make([]InspectField, 0, len(stmt.Fields)+len(nested)), stmt.Fields...), nested...)
	matched := make([]bool, len(fields))
	allAccounted := true
	var maskPositions []int

	for i, dc := range driverCols {
		// The statement's own projection decides where it resolves the column
		// to a table, so a name another scope reuses cannot hide one it shows.
		resolved, deny := seeColumn(stmt.Fields, 0, dc, matched, dbInstanceID, perms)
		if !resolved {
			resolved, deny = seeColumn(nested, len(stmt.Fields), dc, matched, dbInstanceID, perms)
		}
		// A field that resolved to no table carries no permission, so it
		// accounts for nothing.
		if !resolved {
			allAccounted = false
		}
		if deny {
			maskPositions = append(maskPositions, i)
		}
	}

	// A see-denied column the statement selects under no name of its own sits
	// inside an expression, which has no position to mask. Only its own fields
	// are read here: a subquery may select one the outer statement then drops.
	if denied := firstSeeDenied(stmt.Fields, matched, dbInstanceID, perms); denied != nil {
		return nil, denied
	}

	// A result column nothing accounts for may be carrying a hidden one, and
	// the fields left unmatched are what it could be carrying.
	if !allAccounted {
		if denied := firstSeeDenied(fields, matched, dbInstanceID, perms); denied != nil {
			return nil, denied
		}
	}

	return maskPositions, nil
}

// ReturnsRows reports whether an operation can hand rows back to the caller. A
// select does, and so does a write with a RETURNING clause.
func ReturnsRows(op InspectOperation) bool {
	return op == InspectOpSelect || isWrite(op)
}

// seeColumn scans the fields a result column named dc could come out under and
// marks each in matched at its offset. resolved reports whether any of them
// named a table, and deny whether any of those is a column no role may see.
func seeColumn(
	fields []InspectField,
	offset int,
	dc string,
	matched []bool,
	dbInstanceID string,
	perms CompiledPermissions,
) (resolved, deny bool) {
	for fi := range fields {
		f := &fields[fi]
		if !fieldOutputName(*f, dc) {
			continue
		}
		matched[offset+fi] = true
		if f.Schema == "" || f.Table == "" {
			continue
		}
		resolved = true
		if allowed, _ := perms.isAllowed(dbInstanceID, f.Schema, f.Table, f.Name, ActionSee); !allowed {
			deny = true
		}
	}
	return resolved, deny
}

// firstSeeDenied returns the error for the first field no role may see, or nil.
// A field marked in matched is skipped, as is one that resolved to no table.
func firstSeeDenied(fields []InspectField, matched []bool, dbInstanceID string, perms CompiledPermissions) *PermissionDeniedError {
	for fi, f := range fields {
		if fi < len(matched) && matched[fi] {
			continue
		}
		if f.Schema == "" || f.Table == "" {
			continue
		}
		allowed, role := perms.isAllowed(dbInstanceID, f.Schema, f.Table, f.Name, ActionSee)
		if allowed {
			continue
		}
		return &PermissionDeniedError{
			Action:    ActionSee,
			Schema:    f.Schema,
			Table:     f.Table,
			Column:    f.Name,
			RoleName:  role,
			StartLine: f.StartLine,
			StartCol:  f.StartCol,
			EndCol:    f.EndCol,
		}
	}
	return nil
}

// CheckSeePredicates refuses a statement that tests a column no role may see,
// since enough answers about a value are the value.
//
// It runs whether or not the statement returns rows, which is why it is not
// part of EvaluateSee: that one needs the driver's columns, and only a
// statement handing rows back has any.
func CheckSeePredicates(stmts []InspectStatement, dbInstanceID string, perms CompiledPermissions) error {
	if !perms.IsManaged(dbInstanceID) {
		return nil
	}
	for _, stmt := range stmts {
		if denied := firstSeeDenied(predicateFields(stmt, false, nil), nil, dbInstanceID, perms); denied != nil {
			return denied
		}
	}
	return nil
}

// predicateFields returns every field the statement tests rather than returns,
// its subqueries included. A filter's own result columns count as tested, since
// what it selects is compared against something, and so do a write's, since
// what a write reads it stores out of reach of masking.
func predicateFields(stmt InspectStatement, tested bool, into []InspectField) []InspectField {
	into = append(into, stmt.Where...)
	if tested {
		into = append(into, stmt.Fields...)
	}
	stores := isWrite(stmt.Operation)
	for _, sub := range stmt.Subqueries {
		into = predicateFields(sub, tested || sub.Filter || stores, into)
	}
	return into
}

// isWrite reports whether an operation puts rows into a table, where what it
// read is out of reach of masking.
func isWrite(op InspectOperation) bool {
	switch op {
	case InspectOpInsert, InspectOpUpdate, InspectOpDelete:
		return true
	}
	return false
}

// nestedReadFields returns every field the statement's subqueries can return.
// A derived table or a scalar subquery reads a column just as the outer select
// does, and the value it returns is the one that reaches the row. A filter's
// rows are a condition, so it is skipped.
func nestedReadFields(stmt InspectStatement, into []InspectField) []InspectField {
	for _, sub := range stmt.Subqueries {
		if sub.Filter {
			continue
		}
		into = append(into, sub.Fields...)
		into = nestedReadFields(sub, into)
	}
	return into
}

// outputName is the name a field comes out under: its alias where it has one,
// its own name otherwise.
func outputName(f InspectField) string {
	if f.Alias != nil && *f.Alias != "" {
		return *f.Alias
	}
	return f.Name
}

func fieldOutputName(f InspectField, driverCol string) bool {
	return strings.EqualFold(outputName(f), driverCol)
}

// operationToAction maps an inspected operation to the permission it needs.
// Only the four operations we fully resolve down to columns are data actions;
// everything else, the unknown statement included, needs manage. New operations
// are refused until someone classifies them, rather than admitted by silence.
func operationToAction(op InspectOperation) string {
	switch op {
	case InspectOpSelect:
		return ActionSelect
	case InspectOpInsert:
		return ActionInsert
	case InspectOpUpdate:
		return ActionUpdate
	case InspectOpDelete:
		return ActionDelete
	default:
		return ActionManage
	}
}
