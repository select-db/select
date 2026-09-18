package syncer

// This file centralizes the generator-internal domain knowledge the syncer
// projection encodes — the mappings that aren't derivable from the SQL catalog
// and so are the ones to review when the grammar or the domain taxonomy shifts.
// Deliberately kept together (rather than buried next to their emitters) so
// there's a single place to validate.

// auditDescriptor maps a synced table to its audit event taxonomy: the change
// spec emitted when an apply inserts a row (Create) vs. updates one (Update),
// the spec emitted on delete, plus the column whose value is the audit target id
// (the row's own id, or the FK a junction row is "about"). A lifecycle entity
// splits Create/Update (role.lifecycle.create vs update); a junction row has no
// "update" state, so Create == Update names the single grant/add event either
// way. The audit taxonomy lives in the audit package, not the SQL schema.
type auditDescriptor struct{ Create, Update, Delete, TargetCol string }

var auditSpecs = map[string]auditDescriptor{
	"role":              {"audit.RoleCreated", "audit.RoleUpdated", "audit.RoleDeleted", "id"},
	"permission":        {"audit.PermissionCreated", "audit.PermissionUpdated", "audit.PermissionDeleted", "id"},
	"group":             {"audit.GroupCreated", "audit.GroupUpdated", "audit.GroupDeleted", "id"},
	"user_to_role":      {"audit.UserRoleGranted", "audit.UserRoleGranted", "audit.UserRoleRevoked", "user_id"},
	"workspace_to_user": {"audit.WorkspaceUserAdded", "audit.WorkspaceUserAdded", "audit.WorkspaceUserRemoved", "user_id"},
	"user_to_group":     {"audit.GroupUserAdded", "audit.GroupUserAdded", "audit.GroupUserRemoved", "group_id"},
	"group_to_role":     {"audit.GroupRoleGranted", "audit.GroupRoleGranted", "audit.GroupRoleRevoked", "group_id"},
}

// actionConst maps an @app.api `requires` value to the core.Action constant it
// names. The constant values are "workspace/<value>"; referencing the Go symbol
// (rather than reconstructing the string) keeps the generated gate refactor-safe
// and lets an unknown action fail generation instead of silently going ungated.
var actionConst = map[string]string{
	"settings.write":  "core.ActionWorkspaceSettingsWrite",
	"users.manage":    "core.ActionWorkspaceUsersManage",
	"roles.manage":    "core.ActionWorkspaceRolesManage",
	"groups.manage":   "core.ActionWorkspaceGroupsManage",
	"api-keys.manage": "core.ActionWorkspaceApiKeysManage",
	"audit.read":      "core.ActionWorkspaceAuditRead",
}

// ActionConst resolves an @app.api `requires` value to the Go expression for its
// core.Action constant (e.g. "roles.manage" -> "core.ActionWorkspaceRolesManage").
// ok is false for an unknown value, so a generator fails loudly rather than
// emitting an ungated handler. Shared with the HTTP-handler emitter.
func ActionConst(requires string) (string, bool) {
	c, ok := actionConst[requires]
	return c, ok
}
