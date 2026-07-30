package schema

import "testing"

// TestImmutableGatesAPINotSyncer pins the decoupling that a red CI once exposed:
// @app.immutable removes a column from the REST API write-set (IsWritable) but
// must NOT remove it from the syncer's write path (Patchable). A synced column
// like SCIM provenance (source) is owned and written by the app→syncer path and
// is NOT NULL, so dropping it from the upsert Merge — while it stays in the
// INSERT column list — binds NULL and violates the constraint. So an immutable
// column stays patchable (syncer writes and conflict-updates it) and exposed
// (readable/filterable), yet is not client-writable.
func TestImmutableGatesAPINotSyncer(t *testing.T) {
	tbl := RoleTable()
	// Add a NOT NULL @app.immutable column with a DB default, mirroring
	// app."group".source (the field whose omission broke the syncer upsert).
	tbl.Columns = append(tbl.Columns, RawColumn{
		Name: "source", DataType: "text", NotNull: true,
		Default: "'local'::text", Comment: "Origin of the row. @app.immutable",
	})

	ents, errs := Build(RawSchema{Tables: []RawTable{tbl}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}

	var source *Field
	for i := range ents[0].Fields {
		if ents[0].Fields[i].Column == "source" {
			source = &ents[0].Fields[i]
		}
	}
	if source == nil {
		t.Fatal("source column missing from built entity")
	}

	if !source.Immutable {
		t.Error("source: @app.immutable not parsed")
	}
	// Syncer still owns it: patchable => populated in the upsert Merge and the
	// ON CONFLICT SET, so the NOT NULL INSERT never binds NULL.
	if !source.Patchable {
		t.Error("source: immutable must stay Patchable (syncer writes it)")
	}
	// Client can't forge it: excluded from the create/update body.
	if IsWritable(*source) {
		t.Error("source: immutable must not be API-writable")
	}
	// Still visible for read and $filter.
	if !source.Exposed {
		t.Error("source: immutable must stay Exposed (read/filter)")
	}
}
