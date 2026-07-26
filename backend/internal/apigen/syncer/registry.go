package syncer

// This file centralizes the generator-internal domain knowledge the syncer
// projection encodes — the mappings that aren't derivable from the SQL catalog
// and so are the ones to review when the grammar or the domain taxonomy shifts.
// Deliberately kept together (rather than buried next to their emitters) so
// there's a single place to validate.

// auditDescriptor maps a synced table to its audit event taxonomy: the change
// spec emitted on apply (upsert) and on delete, plus the column whose value is
// the audit target id (the row's own id, or the FK a junction row is "about").
// The audit taxonomy lives in the audit package, not the SQL schema.
type auditDescriptor struct{ Apply, Delete, TargetCol string }

var auditSpecs = map[string]auditDescriptor{
	"role":              {"audit.RoleUpserted", "audit.RoleDeleted", "id"},
	"permission":        {"audit.PermissionUpserted", "audit.PermissionDeleted", "id"},
	"group":             {"audit.GroupUpserted", "audit.GroupDeleted", "id"},
	"user_to_role":      {"audit.RoleAssigned", "audit.RoleUnassigned", "user_id"},
	"workspace_to_user": {"audit.MemberAdded", "audit.MemberRemoved", "user_id"},
	"user_to_group":     {"audit.GroupMemberAdded", "audit.GroupMemberRemoved", "group_id"},
	"group_to_role":     {"audit.GroupRoleAttached", "audit.GroupRoleDetached", "group_id"},
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
}
