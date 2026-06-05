package db

import (
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestMapOrdinalToField(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		args          []driver.NamedValue
		wantTableName string
		wantOperation string
		wantArgs      []string
	}{
		{
			name: "UPDATE with id and arg",
			query: `-- name: UpdateFileContent :one
			UPDATE file
			SET content = ?1
			WHERE id = ?2
			RETURNING id, name, content, edit_mode, workspace_id, folder_id, db_instance_id`,
			args: []driver.NamedValue{
				{Ordinal: 1}, {Ordinal: 2},
			},
			wantTableName: "file",
			wantOperation: "update",
			wantArgs:      []string{"content", "id"},
		}, {
			name: "UPDATE with id and args",
			query: `-- name: UpdateDbInstance :one
			UPDATE db_instance
			SET
				name = ?1,
				db_type = ?2,
				dsn = ?3,
				ssh_enabled = ?4,
				ssh_dsn = ?5,
				ssh_public_key = ?6,
				ssh_private_key_encrypted = ?7
			WHERE id = ?8
			RETURNING id, name, db_type, dsn, ssh_enabled, ssh_dsn, ssh_public_key, ssh_private_key_encrypted, workspace_id`,
			args: []driver.NamedValue{
				{Ordinal: 1}, {Ordinal: 2}, {Ordinal: 3}, {Ordinal: 4}, {Ordinal: 5}, {Ordinal: 6}, {Ordinal: 7}, {Ordinal: 8},
			},
			wantTableName: "db_instance",
			wantOperation: "update",
			wantArgs:      []string{"name", "db_type", "dsn", "ssh_enabled", "ssh_dsn", "ssh_public_key", "ssh_private_key_encrypted", "id"},
		}, {
			name: "INSERT with args",
			query: `-- name: CreateDbInstance :one
			INSERT INTO db_instance (
				id,
				name,
				db_type,
				dsn,
				ssh_enabled,
				ssh_dsn,
				ssh_public_key,
				ssh_private_key_encrypted,
				workspace_id
			) VALUES (
				?1,
				?2,
				?3,
				?4,
				?5,
				?6,
				?7,
				?8,
				?9
			)
			RETURNING id, name, db_type, dsn, ssh_enabled, ssh_dsn, ssh_public_key, ssh_private_key_encrypted, workspace_id`,
			args: []driver.NamedValue{
				{Ordinal: 1}, {Ordinal: 2}, {Ordinal: 3}, {Ordinal: 4}, {Ordinal: 5}, {Ordinal: 6}, {Ordinal: 7}, {Ordinal: 8}, {Ordinal: 9},
			},
			wantTableName: "db_instance",
			wantOperation: "insert",
			wantArgs:      []string{"id", "name", "db_type", "dsn", "ssh_enabled", "ssh_dsn", "ssh_public_key", "ssh_private_key_encrypted", "workspace_id"},
		}, {
			name: "DELETE where id",
			query: `-- name: DeleteFile :exec
			DELETE FROM file
			WHERE id = ?1`,
			args: []driver.NamedValue{
				{Ordinal: 1},
			},
			wantTableName: "file",
			wantOperation: "delete",
			wantArgs:      []string{"id"},
		}, {
			name: "DELETE with composite WHERE",
			query: `-- name: DeleteComposite :exec
			DELETE FROM file
			WHERE id = ?1 AND workspace_id = ?2`,
			args: []driver.NamedValue{
				{Ordinal: 1}, {Ordinal: 2},
			},
			wantTableName: "file",
			wantOperation: "delete",
			wantArgs:      []string{"id", "workspace_id"},
		}, {
			name:  "UPDATE with RETURNING clause",
			query: `UPDATE file SET name = ?1 WHERE id = ?2 RETURNING id, name`,
			args: []driver.NamedValue{
				{Ordinal: 1}, {Ordinal: 2},
			},
			wantTableName: "file",
			wantOperation: "update",
			wantArgs:      []string{"name", "id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOperation, gotArgs, gotTableName, err := mapArgsAndOperation(tt.query, tt.args)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotTableName != tt.wantTableName {
				t.Errorf("table mismatch: got %q, want %q", gotTableName, tt.wantTableName)
			}

			if gotOperation != tt.wantOperation {
				t.Errorf("mutation type mismatch: got %q, want %q", gotOperation, tt.wantOperation)
			}

			gotNames := make([]string, len(gotArgs))
			for i, a := range gotArgs {
				gotNames[i] = a.Name
			}

			if !reflect.DeepEqual(gotNames, tt.wantArgs) {
				t.Errorf("arg names mismatch: got %v, want %v", gotNames, tt.wantArgs)
			}
		})
	}
}
