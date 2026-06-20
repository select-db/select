package db_client

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"selectDb/internal/graph"
	"selectDb/internal/sqllang"

	"github.com/selectDb/dialect/engine"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// sshPassphrases holds key-file passphrases entered at runtime, keyed by the
// private key file path. In-memory only (process lifetime)
var (
	sshPassphrases   = map[string]string{}
	sshPassphrasesMu sync.RWMutex
)

// SetSSHKeyPassphrase stores (or clears, when empty) the passphrase for an
// encrypted SSH key file, keyed by its path. Held in memory only.
func (dbc *DbClient) SetSSHKeyPassphrase(keyPath, passphrase string) {
	if keyPath == "" {
		return
	}
	sshPassphrasesMu.Lock()
	defer sshPassphrasesMu.Unlock()
	if passphrase == "" {
		delete(sshPassphrases, keyPath)
		return
	}
	sshPassphrases[keyPath] = passphrase
}

// storedSSHPassphrase returns the in-memory passphrase for a key file path, or "".
func storedSSHPassphrase(keyPath string) string {
	if keyPath == "" {
		return ""
	}
	sshPassphrasesMu.RLock()
	defer sshPassphrasesMu.RUnlock()
	return sshPassphrases[keyPath]
}

// resolveSSHConfig substitutes .env variables in the SSH config fields.
func (dbc *DbClient) resolveSSHConfig(sshCfg *graph.DBInstanceSSHConfig, folderID string) (*engine.ResolvedSSHConfig, error) {
	if sshCfg == nil || !sshCfg.Enabled {
		return nil, nil
	}

	resolve := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return sqllang.SubstituteVariables(dbc.Graph, s, folderID)
	}

	host, err := resolve(sshCfg.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH host: %w", err)
	}
	user, err := resolve(sshCfg.User)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH user: %w", err)
	}
	password, err := resolve(sshCfg.Password)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH password: %w", err)
	}
	privateKey, err := resolve(sshCfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH private key: %w", err)
	}
	keyPath, err := resolve(sshCfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH key path: %w", err)
	}

	port := sshCfg.Port
	if port == 0 {
		port = 22
	}

	// Pull an encrypted key-file passphrase from the in-memory store (entered at
	// runtime, keyed by key path). Never persisted; lives only on this resolved
	// config for the duration of the connect.
	var passphrase string
	if sshCfg.AuthMethod == "key_file" {
		passphrase = storedSSHPassphrase(sshCfg.KeyPath)
	}

	return &engine.ResolvedSSHConfig{
		Host:       host,
		Port:       port,
		User:       user,
		AuthMethod: sshCfg.AuthMethod,
		Password:   password,
		PrivateKey: privateKey,
		KeyPath:    keyPath,
		Passphrase: passphrase,
		HostKey:    sshCfg.HostKey, // public key; no $var indirection
	}, nil
}

// ChooseSSHKeyFile opens a native file picker and returns the chosen private key path.
func (dbc *DbClient) ChooseSSHKeyFile() (string, error) {
	var defaultDir string
	if home, herr := os.UserHomeDir(); herr == nil {
		defaultDir = filepath.Join(home, ".ssh")
	}
	path, err := runtime.OpenFileDialog(dbc.ctx, runtime.OpenDialogOptions{
		Title:            "Choose SSH private key",
		DefaultDirectory: defaultDir,
		ShowHiddenFiles:  true, // ~/.ssh and its keys are hidden
	})
	if err != nil {
		return "", fmt.Errorf("open dialog failed: %w", err)
	}
	return path, nil
}
