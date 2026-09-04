// Command e2eseed writes the state an e2e run needs into a throwaway data
// directory: a migrated database, a user, the workspace they are in, and its
// files. The app shows a login screen without them.
//
// Seeding goes through the app's own migrations and queries, so the fixture
// cannot drift from the real schema. Tokens are not seeded: they live in the OS
// keyring, so the suite emits the `login` event instead.
//
// Test-only: nothing in the shipped binary imports it.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/db"
	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/graph"
	"selectDb/internal/sample"
	"selectDb/internal/server"
	"selectDb/internal/utils"
)

// Fixed so specs can name what they expect to see.
const (
	UserID        = "e2e-user"
	RoleID        = "e2e-role"
	RoleName      = "analyst-readonly"
	UserName      = "Sam Okafor"
	UserEmail     = "sam@example.com"
	WorkspaceID   = "e2e-workspace"
	WorkspaceName = "analytics"
)

// The rest of the team, so the Users, Roles and Groups screens have more than
// one row. Addresses use example.com (RFC 2606), which cannot resolve.
const (
	RoleEngineerID = "e2e-role-engineer"
	RoleEngineer   = "data-engineer"
	RoleAdminID    = "e2e-role-admin"
	RoleAdmin      = "workspace-admin"

	GroupAnalyticsID = "e2e-group-analytics"
	GroupAnalytics   = "analytics"
	GroupPlatformID  = "e2e-group-platform"
	GroupPlatform    = "data-platform"
)

// teammate is a workspace member other than the owner, with whatever is
// granted to them directly and the group they sit in.
type teammate struct {
	id    string
	name  string
	email string
	role  string // "" when their access comes only through their group
	group string
}

// Two hold a role directly, two inherit one through a group; the Users page
// shows both kinds.
var teammates = []teammate{
	{"e2e-user-priya", "Priya Raman", "priya@example.com", RoleEngineerID, GroupPlatformID},
	{"e2e-user-tom", "Tom Boateng", "tom@example.com", "", GroupAnalyticsID},
	{"e2e-user-lena", "Lena Fischer", "lena@example.com", RoleAdminID, GroupPlatformID},
	{"e2e-user-marco", "Marco Silva", "marco@example.com", "", GroupAnalyticsID},
}

// branchName is the work in progress the source-control view shows.
const branchName = "feat/cohort-report"

// cohortsSQL is the commit ahead of the remote; topCustomersEdit is the
// uncommitted change on top of it.
const cohortsNarrowed = `-- Cohort report, first cut.
--
-- Still deciding whether to window by signup month or by first order,
-- so this is deliberately wide until we know.
SELECT
  *
FROM
  customers c
WHERE
  c.created_at >= '2026-01-05'
ORDER BY
  c.created_at DESC;
`

// topCustomersEdited is the working copy. Edited in the middle rather than
// appended to, so the diff carries a removal beside its additions.
const topCustomersEdited = `SELECT
  c.email,
  COUNT(*) AS orders,
  printf('$%.2f', SUM(o.total_cents) / 100.0) AS spend
FROM
  orders o
  JOIN customers c ON c.id = o.customer_id
WHERE
  o.status = 'paid'
GROUP BY
  c.email
ORDER BY
  spend DESC
LIMIT
  50;
`

// DatasourceID re-exports the sample database's id, so a spec and the fixture
// cannot disagree about which database they mean.
const DatasourceID = sample.WarehouseID

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <data-dir>", filepath.Base(os.Args[0]))
	}

	if err := seed(os.Args[1]); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func seed(dataDir string) error {
	// The app resolves its data directory from the OS config dir. Same variables
	// the Playwright config gives the server.
	for _, key := range []string{"XDG_CONFIG_HOME", "HOME"} {
		if err := os.Setenv(key, dataDir); err != nil {
			return err
		}
	}
	if err := os.Setenv("APP_ENV", "dev"); err != nil {
		return err
	}

	// The app registers modernc's driver under this name at startup; the
	// migrations and queries below open it the same way.
	sql.Register("sqlite3", &sqlite.Driver{})

	domain := server.DefaultDomainForEnv()
	if err := server.WriteCurrentDomain(domain); err != nil {
		return fmt.Errorf("select server %q: %w", domain, err)
	}

	dbPath, err := server.ServerDBPath(domain)
	if err != nil {
		return err
	}
	if err := db.RunMigrationsAt(dbPath); err != nil {
		return fmt.Errorf("migrate %s: %w", dbPath, err)
	}

	handle, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer func() { _ = handle.Close() }()

	if err := seedTables(context.Background(), generated.New(handle)); err != nil {
		return err
	}

	// The same workspace a real person is given. Everything below is the test
	// account wrapped around it.
	if err := sample.Write(WorkspaceID); err != nil {
		return fmt.Errorf("sample workspace: %w", err)
	}

	if err := appendChatKey(); err != nil {
		return fmt.Errorf("chat key: %w", err)
	}

	if err := writeAvatars(); err != nil {
		return fmt.Errorf("avatars: %w", err)
	}

	// Last: everything above writes into the workspace, so an earlier init would
	// leave half the fixture untracked.
	return initWorkspaceRepo(dataDir)
}

func seedTables(ctx context.Context, queries *generated.Queries) error {
	if _, err := queries.UpsertUser(ctx, generated.UpsertUserParams{
		ID:   UserID,
		Name: db_types.JSONNullString{NullString: sql.NullString{String: UserName, Valid: true}},
	}); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// UpsertUser has no email column; the sync upsert does, and the Users page
	// has a column for it.
	if err := queries.UpsertUserForSync(ctx, generated.UpsertUserForSyncParams{
		ID:    UserID,
		Name:  text(UserName),
		Email: text(UserEmail),
	}); err != nil {
		return fmt.Errorf("set owner email: %w", err)
	}

	if _, err := queries.CreateWorkspace(ctx, generated.CreateWorkspaceParams{
		ID:      WorkspaceID,
		Name:    WorkspaceName,
		OwnerID: db_types.JSONNullString{NullString: sql.NullString{String: UserID, Valid: true}},
	}); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	if _, err := queries.CreateWorkspaceToUser(ctx, generated.CreateWorkspaceToUserParams{
		ID:          "e2e-workspace-to-user",
		WorkspaceID: WorkspaceID,
		UserID:      UserID,
	}); err != nil {
		return fmt.Errorf("join user to workspace: %w", err)
	}

	// `current` is what GetCurrentUser and GetCurrentWorkspace select on: it is
	// the difference between a seeded database and a signed-in one.
	if err := queries.UpdateCurrentWorkspaceToUser(ctx, generated.UpdateCurrentWorkspaceToUserParams{
		UserID:      UserID,
		WorkspaceID: WorkspaceID,
	}); err != nil {
		return fmt.Errorf("make the workspace current: %w", err)
	}

	return seedRoles(ctx, queries)
}

// seedRoles gives the workspace a role that can read the warehouse except two
// columns. The column-level rules are real, not implied by the screenshot.
func seedRoles(ctx context.Context, queries *generated.Queries) error {
	if err := queries.UpsertRoleForSync(ctx, generated.UpsertRoleForSyncParams{
		ID:          RoleID,
		WorkspaceID: WorkspaceID,
		Name:        RoleName,
	}); err != nil {
		return fmt.Errorf("create role: %w", err)
	}

	// Without this the rules exist and bind nobody.
	if err := queries.UpsertUserToRoleForSync(ctx, generated.UpsertUserToRoleForSyncParams{
		ID:          "e2e-user-role",
		UserID:      UserID,
		RoleID:      RoleID,
		WorkspaceID: WorkspaceID,
	}); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}

	// Lookup runs most-specific to least, so a database-wide grant is '*' at
	// every level below it, not NULL. Actions are the lowercase set the UI
	// renders as columns.
	rules := []struct {
		id                    string
		schema, table, column string
		action, effect        string
	}{
		{"e2e-perm-read", "*", "*", "*", "select", "allow"},
		{"e2e-perm-see", "*", "*", "*", "see", "allow"},

		// Read orders on production without seeing customers.email. Both actions:
		// a column you cannot select is not one you can view either.
		{"e2e-perm-email", "main", "customers", "email", "select", "deny"},
		{"e2e-perm-email-see", "main", "customers", "email", "see", "deny"},

		{"e2e-perm-insert", "*", "*", "*", "insert", "deny"},
		{"e2e-perm-update", "*", "*", "*", "update", "deny"},
		{"e2e-perm-delete", "*", "*", "*", "delete", "deny"},
	}
	for _, r := range rules {
		if err := queries.UpsertPermissionForSync(ctx, generated.UpsertPermissionForSyncParams{
			ID:           r.id,
			RoleID:       RoleID,
			WorkspaceID:  WorkspaceID,
			DbInstanceID: nullable(DatasourceID),
			SchemaName:   nullable(r.schema),
			TableName:    nullable(r.table),
			ColumnName:   nullable(r.column),
			Action:       r.action,
			Effect:       r.effect,
		}); err != nil {
			return fmt.Errorf("create permission %s: %w", r.id, err)
		}
	}
	return seedTeam(ctx, queries)
}

// text wraps a string for the nullable columns the sync upserts take.
func text(v string) db_types.JSONNullString {
	return db_types.JSONNullString{NullString: sql.NullString{String: v, Valid: v != ""}}
}

// seedTeam adds two roles, two groups and four people. None of it touches the
// owner, so the denial on customers.email that three other figures rest on is
// unchanged.
func seedTeam(ctx context.Context, queries *generated.Queries) error {
	// Roles first: a group and a user can only point at one that exists.
	for _, r := range []struct{ id, name string }{
		{RoleEngineerID, RoleEngineer},
		{RoleAdminID, RoleAdmin},
	} {
		if err := queries.UpsertRoleForSync(ctx, generated.UpsertRoleForSyncParams{
			ID:          r.id,
			WorkspaceID: WorkspaceID,
			Name:        r.name,
		}); err != nil {
			return fmt.Errorf("create role %s: %w", r.name, err)
		}
	}

	// Rules for them, so the Roles list does not show a column of zeroes. The
	// engineer can write to the warehouse but not reshape it; the admin's grant
	// is at workspace level, which is what a null datasource means.
	rules := []struct {
		id, role       string
		datasource     string // "" for a workspace-level rule
		action, effect string
	}{
		{"e2e-perm-eng-select", RoleEngineerID, DatasourceID, "select", "allow"},
		{"e2e-perm-eng-see", RoleEngineerID, DatasourceID, "see", "allow"},
		{"e2e-perm-eng-insert", RoleEngineerID, DatasourceID, "insert", "allow"},
		{"e2e-perm-eng-update", RoleEngineerID, DatasourceID, "update", "allow"},
		{"e2e-perm-eng-delete", RoleEngineerID, DatasourceID, "delete", "allow"},
		{"e2e-perm-eng-ddl", RoleEngineerID, DatasourceID, "ddl", "deny"},

		{"e2e-perm-admin-manage", RoleAdminID, "", "manage", "allow"},
		{"e2e-perm-admin-select", RoleAdminID, DatasourceID, "select", "allow"},
		{"e2e-perm-admin-see", RoleAdminID, DatasourceID, "see", "allow"},
	}
	for _, r := range rules {
		params := generated.UpsertPermissionForSyncParams{
			ID:          r.id,
			RoleID:      r.role,
			WorkspaceID: WorkspaceID,
			Action:      r.action,
			Effect:      r.effect,
		}
		if r.datasource != "" {
			params.DbInstanceID = nullable(r.datasource)
			params.SchemaName = nullable("*")
			params.TableName = nullable("*")
			params.ColumnName = nullable("*")
		}
		if err := queries.UpsertPermissionForSync(ctx, params); err != nil {
			return fmt.Errorf("create permission %s: %w", r.id, err)
		}
	}

	// Groups, and the role each one carries. "local" is the schema's own default
	// for a group made here rather than synced from an identity provider.
	for _, g := range []struct{ id, name, role string }{
		{GroupAnalyticsID, GroupAnalytics, RoleID},
		{GroupPlatformID, GroupPlatform, RoleEngineerID},
	} {
		if err := queries.UpsertGroupForSync(ctx, generated.UpsertGroupForSyncParams{
			ID:          g.id,
			WorkspaceID: WorkspaceID,
			Name:        g.name,
			Source:      "local",
		}); err != nil {
			return fmt.Errorf("create group %s: %w", g.name, err)
		}
		if err := queries.UpsertGroupToRoleForSync(ctx, generated.UpsertGroupToRoleForSyncParams{
			ID:          g.id + "-role",
			GroupID:     g.id,
			RoleID:      g.role,
			WorkspaceID: WorkspaceID,
		}); err != nil {
			return fmt.Errorf("grant role to group %s: %w", g.name, err)
		}
	}

	// The owner joins analytics too. That group carries analyst-readonly, which
	// he already holds directly, so this adds a row to a list and not a single
	// permission.
	if err := queries.UpsertUserToGroupForSync(ctx, generated.UpsertUserToGroupForSyncParams{
		ID:          UserID + "-group",
		UserID:      UserID,
		GroupID:     GroupAnalyticsID,
		WorkspaceID: WorkspaceID,
		Source:      "local",
	}); err != nil {
		return fmt.Errorf("add owner to group: %w", err)
	}

	for _, m := range teammates {
		if err := queries.UpsertUserForSync(ctx, generated.UpsertUserForSyncParams{
			ID:    m.id,
			Name:  text(m.name),
			Email: text(m.email),
		}); err != nil {
			return fmt.Errorf("create user %s: %w", m.name, err)
		}
		// A user the workspace does not hold is a user no screen here lists.
		if err := queries.UpsertWorkspaceToUserForSync(ctx, generated.UpsertWorkspaceToUserForSyncParams{
			ID:          m.id + "-workspace",
			WorkspaceID: WorkspaceID,
			UserID:      m.id,
		}); err != nil {
			return fmt.Errorf("join %s to workspace: %w", m.name, err)
		}
		if err := queries.UpsertUserToGroupForSync(ctx, generated.UpsertUserToGroupForSyncParams{
			ID:          m.id + "-group",
			UserID:      m.id,
			GroupID:     m.group,
			WorkspaceID: WorkspaceID,
			Source:      "local",
		}); err != nil {
			return fmt.Errorf("add %s to group: %w", m.name, err)
		}
		if m.role == "" {
			continue
		}
		if err := queries.UpsertUserToRoleForSync(ctx, generated.UpsertUserToRoleForSyncParams{
			ID:          m.id + "-role",
			UserID:      m.id,
			RoleID:      m.role,
			WorkspaceID: WorkspaceID,
		}); err != nil {
			return fmt.Errorf("grant role to %s: %w", m.name, err)
		}
	}

	return nil
}

// nullable turns "" into SQL NULL, which the permission table reads as "every
// value at this level" rather than as a column literally named "".
func nullable(v string) db_types.JSONNullString {
	return db_types.JSONNullString{NullString: sql.NullString{String: v, Valid: v != ""}}
}

// appendChatKey adds the AI provider key to the workspace .env the sample
// wrote. It is a placeholder and it is test-only: the capture suite answers the
// provider's endpoint itself, so nothing in a run reaches a real API, and no
// workspace a person is given should ship a key at all.
func appendChatKey() error {
	root, err := graph.WorkspaceRootPath(WorkspaceID)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(root, ".env"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = file.WriteString("\n# The AI chat reads its provider key from here.\n" +
		"ANTHROPIC_API_KEY=sk-ant-e2e-placeholder-not-a-real-key\n")
	return err
}

func initWorkspaceRepo(dataDir string) error {
	root, err := graph.WorkspaceRootPath(WorkspaceID)
	if err != nil {
		return err
	}

	origin := filepath.Join(dataDir, "origin.git")
	if err := run("", "git", "init", "--quiet", "--bare", "--initial-branch=main", origin); err != nil {
		return err
	}

	steps := [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		// Identity and signing are per-machine; a fixture cannot inherit them
		// and commit reproducibly.
		{"config", "user.name", UserName},
		{"config", "user.email", "sam@example.com"},
		{"config", "commit.gpgsign", "false"},
		{"remote", "add", "origin", origin},
		{"add", "--all"},
		{"commit", "--quiet", "-m", "the queries we had before this branch"},
		{"push", "--quiet", "-u", "origin", "main"},
		{"checkout", "--quiet", "-b", branchName},
	}
	for _, args := range steps {
		if err := run(root, "git", args...); err != nil {
			return err
		}
	}

	// One commit the remote has not seen, so the view has an "ahead" to report.
	if err := os.WriteFile(filepath.Join(root, "cohorts.sql"), []byte(cohortsNarrowed), 0o600); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "cohorts.sql"},
		{"commit", "--quiet", "-m", "narrow the cohort window to this quarter"},
	} {
		if err := run(root, "git", args...); err != nil {
			return err
		}
	}

	// And uncommitted work, which is the state a person is actually in when
	// they open this view.
	return os.WriteFile(
		filepath.Join(root, "top_customers.sql"),
		[]byte(topCustomersEdited),
		0o600,
	)
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	// A developer's own git config must not reach a fixture: templates, hooks
	// and a global gitignore all change what the seeded repository looks like.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, out)
	}
	return nil
}

// writeAvatars draws one avatar per member of the workspace. The app resolves
// an avatar by user id, so the files are all this takes.
//
// Now that the workspace has more than one person in it, the mark is derived
// from the id rather than fixed: same id, same picture on every run, and no
// two people in a list wearing the same face.
func writeAvatars() error {
	appDataDir, err := utils.GetAppDataDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(appDataDir, "assets", "avatars")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	ids := []string{UserID}
	for _, m := range teammates {
		ids = append(ids, m.id)
	}
	for _, id := range ids {
		if err := writeAvatar(dir, id); err != nil {
			return fmt.Errorf("avatar %s: %w", id, err)
		}
	}
	return nil
}

// ownerAvatarBits reproduces the owner's original mark. The low fifteen bits
// are the cell pattern the single-user fixture drew by hand, read as the loop
// below reads them (bit row*3+col, low bit first); the sixteenth is chosen so
// the value lands on field 0, the indigo he already wore.
const ownerAvatarBits uint32 = 0b1101010101111010

// The field colours, dark enough for the light glyph over them to read. Index 0
// is the owner's; see ownerAvatarBits.
var avatarFields = []color.RGBA{
	{R: 0x2B, G: 0x3F, B: 0xD6, A: 0xFF},
	{R: 0x1F, G: 0x7A, B: 0x5C, A: 0xFF},
	{R: 0x9B, G: 0x2C, B: 0x4A, A: 0xFF},
	{R: 0x6B, G: 0x3F, B: 0xA8, A: 0xFF},
	{R: 0xA8, G: 0x5A, B: 0x1F, A: 0xFF},
}

func writeAvatar(dir, id string) error {
	const size = 128
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// FNV-1a over the id: one number, and every choice below reads bits off it.
	// Deterministic, so an avatar is not a diff on every seed.
	var h uint32 = 2166136261
	for i := 0; i < len(id); i++ {
		h ^= uint32(id[i])
		h *= 16777619
	}
	// The owner keeps the mark he had before anyone else was in the workspace.
	// His face is in the status bar of every product figure on the site, and
	// re-rolling it would rewrite all of them to say nothing new.
	if id == UserID {
		h = ownerAvatarBits
	}

	bg := avatarFields[h%uint32(len(avatarFields))]
	fg := color.RGBA{R: 0xD8, G: 0xDE, B: 0xFF, A: 0xFF}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	// A 5x5 grid mirrored left to right, so the result reads as a face-like
	// mark rather than as noise. Fifteen cells, one bit each.
	const cell = size / 5
	for row := 0; row < 5; row++ {
		for col := 0; col < 3; col++ {
			if h>>(uint(row*3+col))&1 == 0 {
				continue
			}
			for _, c := range []int{col, 4 - col} {
				rect := image.Rect(c*cell, row*cell, (c+1)*cell, (row+1)*cell)
				draw.Draw(img, rect, &image.Uniform{C: fg}, image.Point{}, draw.Src)
			}
		}
	}

	file, err := os.Create(filepath.Join(dir, id+"_avatar.png"))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return png.Encode(file, img)
}
