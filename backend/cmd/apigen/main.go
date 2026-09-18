// Command apigen generates the sync API from the SQL schema. It introspects the
// live database (POSTGRES_DSN) plus the @app.* tag comments — migrations must
// already be applied — and:
//
//   - inspect:  dumps the derived entity model as JSON
//   - generate: writes the generated tree —
//   - gen/<table>/    per-entity CRUD SQL + syncer glue
//   - syncer/types/   wire row structs
//   - gen/            dispatch / authorize / changes registries
//   - apigen/gen/     the frontend capabilities + OpenAPI documents
//
// Run scripts/generate_queries.sh (sqlc) after generate so db/generated matches.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/apigen/api"
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/httpapi"
	"backend/internal/apigen/openapi"
	"backend/internal/apigen/schema"
	"backend/internal/apigen/syncer"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	mode := "inspect"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// Load backend/.env when run from the backend dir, matching cmd/server.
	_ = godotenv.Load()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		fail(fmt.Errorf("POSTGRES_DSN is required"))
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fail(err)
	}
	defer func() { _ = db.Close() }()

	raw, err := schema.Introspect(context.Background(), db, "app", "audit")
	if err != nil {
		fail(err)
	}
	entities, lintErrs := schema.Build(raw)
	for _, e := range lintErrs {
		fmt.Fprintln(os.Stderr, "lint:", e)
	}
	if len(lintErrs) > 0 {
		os.Exit(1)
	}

	switch mode {
	case "inspect":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entities); err != nil {
			fail(err)
		}
	case "generate":
		root, err := moduleRoot()
		if err != nil {
			fail(err)
		}
		if err := generate(root, entities); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown mode %q (want inspect|generate)", mode))
	}
}

// generate writes every generated artifact into the tree: per synced entity
// its SQL + glue + row type, then the cross-table syncer registries, then the
// frontend capabilities document.
func generate(root string, entities []schema.Entity) error {
	scoped := workspaceScopedTables(entities)
	for _, e := range entities {
		if !e.Sync {
			continue
		}
		if err := generateSyncEntity(root, e, scoped); err != nil {
			return err
		}
	}
	if err := generateDispatchRegistry(root, entities); err != nil {
		return err
	}
	if err := generateAuthorizeRegistry(root, entities); err != nil {
		return err
	}
	if err := generateChangesAggregation(root, entities); err != nil {
		return err
	}
	if err := generateScope(root, entities); err != nil {
		return err
	}
	if err := generateAPICapabilities(root, entities); err != nil {
		return err
	}
	if err := generateOpenAPI(root, entities); err != nil {
		return err
	}
	return generateAPIHandlers(root, entities)
}

// generateAPIHandlers writes the generated REST route table (rest.Entity
// descriptors + RegisterRoutes) the hand-written rest runtime consumes: one
// package folder per @app.api entity plus the top-level routes.go, under
// internal/api/gen. File names are path-qualified (e.g. role/resource.go), so
// writeFile's MkdirAll creates the per-table subdirs.
func generateAPIHandlers(root string, entities []schema.Entity) error {
	files, err := httpapi.EmitRoutes(entities)
	if err != nil {
		return err
	}
	base := filepath.Join(root, "internal", "api", "gen")
	for _, f := range files {
		if err := writeFile(filepath.Join(base, filepath.FromSlash(f.Name)), f.Content); err != nil {
			return err
		}
	}
	fmt.Println("generated api routes")
	return nil
}

// generateSyncEntity writes one synced table's three artifacts: its sqlc CRUD
// queries, the syncer glue, and the wire row type.
func generateSyncEntity(root string, e schema.Entity, scoped map[string]bool) error {
	if err := generateEntitySQL(root, e); err != nil {
		return err
	}
	if err := generateEntityGlue(root, e, scoped); err != nil {
		return err
	}
	if err := generateEntityRowType(root, e); err != nil {
		return err
	}
	fmt.Printf("generated syncer sql + glue + row type for %s\n", e.Table)
	return nil
}

// generateEntitySQL writes the table's sqlc CRUD queries into gen/<table>/sql/.
func generateEntitySQL(root string, e schema.Entity) error {
	dir := filepath.Join(syncerGenDir(root), e.Table, "sql")
	for _, f := range syncer.EmitSyncSQL(e) {
		if err := writeFile(filepath.Join(dir, f.Name), f.Content); err != nil {
			return err
		}
	}
	return nil
}

// generateEntityGlue writes the table's syncer glue — apply / apply_delete /
// fetch_current / getsince — into gen/<table>/.
func generateEntityGlue(root string, e schema.Entity, scoped map[string]bool) error {
	files, err := syncer.EmitSyncGlue(e, scoped)
	if err != nil {
		return fmt.Errorf("%s glue: %w", e.Table, err)
	}
	dir := filepath.Join(syncerGenDir(root), e.Table)
	for _, f := range files {
		if err := writeFile(filepath.Join(dir, f.Name), f.Content); err != nil {
			return err
		}
	}
	return nil
}

// generateEntityRowType writes the table's wire struct into the shared types
// package — it must live beside the hand-written SyncChanges it feeds.
func generateEntityRowType(root string, e schema.Entity) error {
	f, err := syncer.EmitSyncRowType(e)
	if err != nil {
		return fmt.Errorf("%s row type: %w", e.Table, err)
	}
	return writeFile(filepath.Join(root, "internal", "syncer", "types", f.Name), f.Content)
}

// generateDispatchRegistry writes gen/dispatch.go — the table -> handler map.
func generateDispatchRegistry(root string, entities []schema.Entity) error {
	f, err := syncer.EmitSyncDispatch(entities)
	if err != nil {
		return fmt.Errorf("sync dispatch: %w", err)
	}
	return writeGen(syncerGenDir(root), f, "syncer dispatch registry")
}

// generateAuthorizeRegistry writes gen/authorize.go — the per-table required
// workspace actions.
func generateAuthorizeRegistry(root string, entities []schema.Entity) error {
	f, err := syncer.EmitAuthorize(entities)
	if err != nil {
		return fmt.Errorf("authorize: %w", err)
	}
	return writeGen(syncerGenDir(root), f, "syncer authorize registry")
}

// generateChangesAggregation writes gen/changes.go — the changes-since fetch +
// applied-id filter over every synced table.
func generateChangesAggregation(root string, entities []schema.Entity) error {
	f, err := syncer.EmitSyncChanges(entities)
	if err != nil {
		return fmt.Errorf("sync changes: %w", err)
	}
	return writeGen(syncerGenDir(root), f, "syncer changes aggregation")
}

// generateScope writes internal/syncer/scope/scope.go — the <Target>InWorkspace
// cross-workspace FK guards, one per workspace-scoped FK target.
func generateScope(root string, entities []schema.Entity) error {
	f, err := syncer.EmitScope(entities, workspaceScopedTables(entities))
	if err != nil {
		return fmt.Errorf("scope: %w", err)
	}
	path := filepath.Join(root, "internal", "syncer", "scope", f.Name)
	if err := writeFile(path, f.Content); err != nil {
		return err
	}
	fmt.Println("generated scope guards")
	return nil
}

// generateAPICapabilities writes the machine-readable capabilities document the
// frontend consumes to build filter/sort UIs.
func generateAPICapabilities(root string, entities []schema.Entity) error {
	caps, err := api.EmitAPICapabilities(entities)
	if err != nil {
		return err
	}
	path := filepath.Join(root, "internal", "apigen", "gen", "capabilities.json")
	if err := writeFile(path, string(caps)+"\n"); err != nil {
		return err
	}
	fmt.Println("generated capabilities.json")
	return nil
}

// generateOpenAPI writes the OpenAPI document the docs-site API reference
// (Scalar) renders. Emitted alongside capabilities.json under apigen/gen; the
// docs generator copies it into the built site.
func generateOpenAPI(root string, entities []schema.Entity) error {
	doc, err := openapi.EmitOpenAPI(entities)
	if err != nil {
		return err
	}
	path := filepath.Join(root, "internal", "apigen", "gen", "openapi.json")
	if err := writeFile(path, string(doc)+"\n"); err != nil {
		return err
	}
	fmt.Println("generated openapi.json")
	return nil
}

// workspaceScopedTables is the set of tables carrying the tenant column. An FK
// pointing at one of them is workspace-scoped, so the glue emitter guards it
// (scope.<Target>InWorkspace); a global target like user has no such column and
// is left unguarded. Derived here rather than hand-listed in the generator.
func workspaceScopedTables(entities []schema.Entity) map[string]bool {
	scoped := map[string]bool{}
	for _, e := range entities {
		for _, f := range e.Fields {
			if f.Column == schema.TenantColumn {
				scoped[e.Table] = true
				break
			}
		}
	}
	return scoped
}

// syncerGenDir is the root of the generated syncer tree (internal/syncer/gen).
func syncerGenDir(root string) string {
	return filepath.Join(root, "internal", "syncer", "gen")
}

// writeGen writes one generated file into dir and logs what it produced.
func writeGen(dir string, f codegen.GenFile, label string) error {
	if err := writeFile(filepath.Join(dir, f.Name), f.Content); err != nil {
		return err
	}
	fmt.Println("generated " + label)
	return nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// path is the module root joined with fixed segments and the schema's own
	// table names (a dev codegen tool), never external input.
	return os.WriteFile(path, []byte(content), 0o644) // #nosec G703 -- codegen path, not user input
}

// moduleRoot walks up to the directory holding go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from working directory")
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "apigen:", err)
	os.Exit(1)
}
