package schema

// Fixture tables shared by the schema tests and the projection-emitter tests
// (which live in sibling packages and so can't reach test-only helpers). They
// mirror the real app.* tables closely enough to exercise derivation + emission
// offline, without a database. Kept here (not in _test.go) precisely so the
// emitter packages can import them.

// RoleTable is the simplest synced entity — everything derives from convention;
// only the sync + api tags sit on the table.
func RoleTable() RawTable {
	return RawTable{
		Schema:  "app",
		Name:    "role",
		Comment: "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage",
		Columns: []RawColumn{
			{Name: "id", DataType: "uuid", NotNull: true},
			{Name: "workspace_id", DataType: "uuid", NotNull: true},
			{Name: "name", DataType: "text"},
			{Name: "updated_at", DataType: "timestamp with time zone", NotNull: true},
			{Name: "deleted_at", DataType: "timestamp with time zone"},
		},
		PrimaryKey:  []string{"id"},
		ForeignKeys: []RawFK{{Column: "workspace_id", RefSchema: "app", RefTable: "workspace", RefColumn: "id"}},
	}
}

// PermissionTable has a role FK (exercises the cross-workspace scope guard) and
// a NOT NULL effect column with a DB default (exercises default coalescing).
func PermissionTable() RawTable {
	return RawTable{
		Schema: "app", Name: "permission",
		Comment: "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage",
		Columns: []RawColumn{
			{Name: "id", DataType: "uuid", NotNull: true},
			{Name: "role_id", DataType: "uuid", NotNull: true},
			{Name: "workspace_id", DataType: "uuid", NotNull: true},
			{Name: "db_instance_id", DataType: "text"},
			{Name: "schema_name", DataType: "text"},
			{Name: "table_name", DataType: "text"},
			{Name: "column_name", DataType: "text"},
			{Name: "action", DataType: "text", NotNull: true},
			{Name: "effect", DataType: "text", NotNull: true},
			{Name: "updated_at", DataType: "timestamptz", NotNull: true},
			{Name: "deleted_at", DataType: "timestamptz"},
		},
		PrimaryKey: []string{"id"},
		ForeignKeys: []RawFK{
			{Column: "role_id", RefSchema: "app", RefTable: "role", RefColumn: "id"},
			{Column: "workspace_id", RefSchema: "app", RefTable: "workspace", RefColumn: "id"},
		},
	}
}

// UserToRoleTable is a junction with two non-tenant FKs (user, role).
func UserToRoleTable() RawTable {
	return RawTable{
		Schema: "app", Name: "user_to_role",
		Comment: "@app.sync @app.api.create|update|delete requires roles.manage",
		Columns: []RawColumn{
			{Name: "id", DataType: "uuid", NotNull: true},
			{Name: "user_id", DataType: "uuid", NotNull: true},
			{Name: "role_id", DataType: "uuid", NotNull: true},
			{Name: "workspace_id", DataType: "uuid", NotNull: true},
			{Name: "updated_at", DataType: "timestamptz", NotNull: true},
			{Name: "deleted_at", DataType: "timestamptz"},
		},
		PrimaryKey: []string{"id"},
		ForeignKeys: []RawFK{
			{Column: "user_id", RefSchema: "app", RefTable: "user", RefColumn: "id"},
			{Column: "role_id", RefSchema: "app", RefTable: "role", RefColumn: "id"},
			{Column: "workspace_id", RefSchema: "app", RefTable: "workspace", RefColumn: "id"},
		},
	}
}
