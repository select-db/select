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
	ActionDDL    = "ddl"
	ActionSee    = "see"
)

const MaskedValue = "*****"

// PermissionEntry is a single rule. nil pointer fields mean wildcard.
type PermissionEntry struct {
	DbInstanceID *string
	SchemaName   *string
	TableName    *string
	ColumnName   *string
	Action       string // "select" | "insert" | "update" | "delete" | "ddl"
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
	if e.Column != "" {
		target = fmt.Sprintf("%s.%s.%s", e.Schema, e.Table, e.Column)
	} else {
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
	allowed, _ := idx.isAllowed(dbInstanceID, "", "", "", "manage")
	return allowed
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

		if action == "ddl" || len(stmt.Fields) == 0 {
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
			continue
		}

		for _, field := range stmt.Fields {
			if field.Table != table.Name || field.Schema != table.Schema {
				continue
			}

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
	}

	for _, sub := range stmt.Subqueries {
		if err := checkStatement(sub, dbInstanceID, compiledPermissions); err != nil {
			return err
		}
	}

	return nil
}

// EvaluateSee returns which driver-column positions to mask, or errors
// when a see-denied column can't be masked (used only inside a function).
//
// Matches Fields to driverCols by alias or name (case-insensitive).
// If multiple Fields share a name (JOINs), any see-denied one masks the position.
func EvaluateSee(stmt InspectStatement, driverCols []string, dbInstanceID string, perms CompiledPermissions) ([]int, error) {
	if !perms.IsManaged(dbInstanceID) {
		return nil, nil
	}

	if stmt.Operation != InspectOpSelect {
		return nil, nil
	}

	if len(driverCols) == 0 {
		return nil, nil
	}

	matched := make([]bool, len(stmt.Fields))
	var maskPositions []int

	for i, dc := range driverCols {
		deny := false
		for fi := range stmt.Fields {
			f := &stmt.Fields[fi]
			if !fieldOutputName(*f, dc) {
				continue
			}

			matched[fi] = true
			if f.Schema == "" || f.Table == "" {
				continue
			}

			allowed, _ := perms.isAllowed(dbInstanceID, f.Schema, f.Table, f.Name, ActionSee)
			if !allowed {
				deny = true
			}
		}

		if deny {
			maskPositions = append(maskPositions, i)
		}
	}

	// Unmatched see-denied Field = used inside a function, can't mask safely
	for fi, f := range stmt.Fields {
		if matched[fi] {
			continue
		}
		if f.Schema == "" || f.Table == "" {
			continue
		}
		allowed, role := perms.isAllowed(dbInstanceID, f.Schema, f.Table, f.Name, ActionSee)
		if allowed {
			continue
		}
		return nil, &PermissionDeniedError{
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

	return maskPositions, nil
}

func fieldOutputName(f InspectField, driverCol string) bool {
	name := f.Name
	if f.Alias != nil && *f.Alias != "" {
		name = *f.Alias
	}
	return strings.EqualFold(name, driverCol)
}

func operationToAction(op InspectOperation) string {
	switch op {
	case InspectOpSelect:
		return "select"
	case InspectOpInsert:
		return "insert"
	case InspectOpUpdate:
		return "update"
	case InspectOpDelete:
		return "delete"
	case InspectOpCreate, InspectOpAlter, InspectOpDrop, InspectOpTruncate:
		return "ddl"
	default:
		return string(op)
	}
}
