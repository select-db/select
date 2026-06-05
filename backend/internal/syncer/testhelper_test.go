package syncer

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/peterldowns/pgtestdb"
	"github.com/peterldowns/pgtestdb/migrators/goosemigrator"
	"github.com/stretchr/testify/require"

	"backend/db"
)

const (
	testPGUser     = "postgres"
	testPGPassword = "postgres"
	testPGDatabase = "postgres"
)

var testPGConfig pgtestdb.Config

func freePort() uint32 {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("find free port: %v", err))
	}
	defer func() { _ = l.Close() }()
	return uint32(l.Addr().(*net.TCPAddr).Port)
}

func TestMain(m *testing.M) {
	port := freePort()
	testPGConfig = pgtestdb.Config{
		DriverName: "postgres",
		Host:       "localhost",
		Port:       fmt.Sprintf("%d", port),
		User:       testPGUser,
		Password:   testPGPassword,
		Database:   testPGDatabase,
		Options:    "sslmode=disable",
	}

	runtimeDir, err := os.MkdirTemp("", "embedded-postgres-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create runtime dir: %v\n", err)
		os.Exit(1)
	}

	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(port).
			Username(testPGUser).
			Password(testPGPassword).
			Database(testPGDatabase).
			RuntimePath(runtimeDir),
	)
	if err := pg.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "embedded postgres start: %v\n", err)
		os.Exit(1)
	}

	stop := func(code int) {
		_ = pg.Stop()
		_ = os.RemoveAll(runtimeDir)
		os.Exit(code)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		stop(1)
	}()

	stop(m.Run())
}

func newID() string { return uuid.NewString() }

func seedUser(t *testing.T, conn *sql.DB, id, name string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app."user" (id, name, email) VALUES ($1::uuid, $2, $3)`, id, name, id+"@test.local")
	require.NoError(t, err)
}

func seedWorkspace(t *testing.T, conn *sql.DB, id, name, ownerID string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app.workspace (id, name, owner_id) VALUES ($1::uuid, $2, $3::uuid)`, id, name, ownerID)
	require.NoError(t, err)
}

func seedRole(t *testing.T, conn *sql.DB, id, workspaceID, name string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app.role (id, workspace_id, name) VALUES ($1::uuid, $2::uuid, $3)`, id, workspaceID, name)
	require.NoError(t, err)
}

func seedPermission(t *testing.T, conn *sql.DB, id, roleID, workspaceID, action, effect string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.permission (id, role_id, workspace_id, action, effect) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)`,
		id, roleID, workspaceID, action, effect,
	)
	require.NoError(t, err)
}

// newTestDB creates an isolated database for one test via pgtestdb template cloning
// and wires it into the global db package.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	migrator := goosemigrator.New(
		"migrations",
		goosemigrator.WithFS(db.MigrationsFS),
	)
	conn := pgtestdb.New(t, testPGConfig, migrator)
	require.NotNil(t, conn)
	db.SetDB(conn)
	return conn
}
