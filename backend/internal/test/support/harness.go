// Package support is the shared end-to-end test harness: an embedded
// Postgres, per-test isolated databases, a running audit logger, dev crypto
// (JWT signing key + KEK), and helpers to craft accounts, mint tokens, drive
// the HTTP API, and assert on emitted audit events. It is imported only by test
// binaries. Tests using it must not call t.Parallel(): the audit logger and the
// db package are process-global singletons rebound per test.
package support

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/peterldowns/pgtestdb/migrators/goosemigrator"
	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/internal/kms"
)

const (
	testPGUser     = "postgres"
	testPGPassword = "postgres"
	testPGDatabase = "postgres"
)

// pgConfig is set by StartPostgres and read by NewDB.
var pgConfig pgtestdb.Config

// StartPostgres boots a single embedded Postgres for the whole test binary and
// installs the dev crypto env (JWT signer + KEK) that the auth and datasource
// paths require. Call it from TestMain and defer the returned stop func. The
// returned code is passed straight to os.Exit by the caller.
func StartPostgres() (stop func(code int), err error) {
	port := freePort()
	pgConfig = pgtestdb.Config{
		DriverName: "postgres",
		Host:       "localhost",
		Port:       fmt.Sprintf("%d", port),
		User:       testPGUser,
		Password:   testPGPassword,
		Database:   testPGDatabase,
		Options:    "sslmode=disable",
	}

	runtimeDir, err := os.MkdirTemp("", "e2e-postgres-*")
	if err != nil {
		return nil, fmt.Errorf("create runtime dir: %w", err)
	}
	// Keep crypto material outside RuntimePath: embedded-postgres wipes its
	// runtime dir on Start, which would delete the signing key.
	cryptoDir, err := os.MkdirTemp("", "e2e-crypto-*")
	if err != nil {
		_ = os.RemoveAll(runtimeDir)
		return nil, fmt.Errorf("create crypto dir: %w", err)
	}

	if err := setupCrypto(cryptoDir); err != nil {
		_ = os.RemoveAll(runtimeDir)
		_ = os.RemoveAll(cryptoDir)
		return nil, err
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
		_ = os.RemoveAll(runtimeDir)
		_ = os.RemoveAll(cryptoDir)
		return nil, fmt.Errorf("embedded postgres start: %w", err)
	}

	return func(code int) {
		_ = pg.Stop()
		_ = os.RemoveAll(runtimeDir)
		_ = os.RemoveAll(cryptoDir)
		os.Exit(code)
	}, nil
}

// setupCrypto writes a throwaway RSA key for JWT signing and sets a random KEK,
// both in local (non-KMS) mode. Env is process-global; set once per binary.
func setupCrypto(dir string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate rsa key: %w", err)
	}
	keyPath := filepath.Join(dir, "jwt_signing_key.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		return fmt.Errorf("write signing key: %w", err)
	}
	if err := os.Setenv("PRIVATE_KEY_PATH", keyPath); err != nil {
		return err
	}

	kek := make([]byte, 32)
	if _, err := rand.Read(kek); err != nil {
		return fmt.Errorf("generate kek: %w", err)
	}
	// SELECTDB_KEK doubles as the dev-mode switch for both the KEK provider and
	// the JWT signer (kms.localMode checks it is set).
	return os.Setenv(kms.KEKEnv, base64.StdEncoding.EncodeToString(kek))
}

// NewDB creates an isolated database for one test via pgtestdb template cloning,
// runs migrations, and binds it into the global db package.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()
	migrator := goosemigrator.New("migrations", goosemigrator.WithFS(db.MigrationsFS))
	conn := pgtestdb.New(t, pgConfig, migrator)
	require.NotNil(t, conn)
	db.SetDB(conn)
	return conn
}

func freePort() uint32 {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("find free port: %v", err))
	}
	defer func() { _ = l.Close() }()
	return uint32(l.Addr().(*net.TCPAddr).Port)
}
