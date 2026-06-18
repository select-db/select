package datasource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/selectDb/dialect/core"
	"golang.org/x/crypto/ssh"
)

// Secrets are write-only. GetHandler returns a deterministic preview only:
// passwords -> length-gated end chars (previewPassword), SSH key -> OpenSSH
// fingerprint (previewKey). On upsert, incoming == preview(stored) means
// untouched -> keep stored, else store verbatim. Keeps debounced auto-save
// safe with no client state.

const maskCore = "••••"

// previewPassword reveals end chars, scaled to length so short secrets stay
// fully masked.
func previewPassword(s string) string {
	r := []rune(s)
	switch n := len(r); {
	case n == 0:
		return ""
	case n < 8:
		return maskCore
	case n < 12:
		return string(r[0:1]) + maskCore + string(r[n-1:n])
	default:
		return string(r[0:2]) + maskCore + string(r[n-2:n])
	}
}

// previewKey returns the OpenSSH public-key fingerprint (ssh-keygen -lf /
// GitHub format), e.g. "SHA256:... (ED25519)". Leaks no key material.
func previewKey(s string) string {
	if s == "" {
		return ""
	}
	signer, err := ssh.ParsePrivateKey([]byte(s))
	if err != nil {
		if _, missing := err.(*ssh.PassphraseMissingError); missing {
			if t, ok := pemType(s); ok {
				return t + " (encrypted)"
			}
			return "encrypted private key"
		}
		// Unparseable: a stable content hash still distinguishes keys.
		sum := sha256.Sum256([]byte(s))
		return "private key · " + hex.EncodeToString(sum[:])[:8]
	}
	pub := signer.PublicKey()
	return ssh.FingerprintSHA256(pub) + " (" + sshKeyKind(pub.Type()) + ")"
}

// sshKeyKind maps an SSH algorithm name to the short label ssh-keygen prints.
func sshKeyKind(t string) string {
	switch {
	case t == "ssh-ed25519" || strings.HasPrefix(t, "ssh-ed25519"):
		return "ED25519"
	case t == "ssh-rsa" || strings.HasPrefix(t, "rsa-sha2"):
		return "RSA"
	case strings.HasPrefix(t, "ecdsa-"):
		return "ECDSA"
	case t == "ssh-dss":
		return "DSA"
	default:
		return strings.ToUpper(t)
	}
}

func pemType(s string) (string, bool) {
	const begin = "-----BEGIN "
	i := strings.Index(s, begin)
	if i < 0 {
		return "", false
	}
	rest := s[i+len(begin):]
	j := strings.Index(rest, "-----")
	if j <= 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:j]), true
}

// maskDSN replaces the DSN password with its preview; rest stays editable.
func maskDSN(dbType, dsn string) string {
	pw := core.DSNPassword(dbType, dsn)
	if pw == "" {
		return dsn
	}
	return core.DSNSetPassword(dbType, dsn, previewPassword(pw))
}

// mergeDSN: incoming password == preview(stored) means untouched -> restore
// stored password onto the (possibly edited) host/db; else store verbatim.
func mergeDSN(dbType, newDSN, oldDSN string) string {
	old := core.DSNPassword(dbType, oldDSN)
	if core.DSNPassword(dbType, newDSN) != previewPassword(old) {
		return newDSN // user typed a complete value (new password, or none)
	}
	if old == "" {
		return core.DSNStripPassword(dbType, newDSN)
	}
	return core.DSNSetPassword(dbType, newDSN, old)
}

func sshStr(v any) string {
	s, _ := v.(string)
	return s
}

// maskSSH replaces password/private_key in the SSH JSON with their preview.
func maskSSH(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	m["password"] = previewPassword(sshStr(m["password"]))
	m["private_key"] = previewKey(sshStr(m["private_key"]))
	b, err := json.Marshal(m)
	if err != nil {
		return s
	}
	return string(b)
}

// mergeSSH: per secret, incoming == preview(stored) -> keep stored, else
// store verbatim (empty = explicit clear).
func mergeSSH(newJSON, oldJSON string) string {
	if strings.TrimSpace(newJSON) == "" {
		return newJSON
	}
	var mn, mo map[string]any
	if err := json.Unmarshal([]byte(newJSON), &mn); err != nil {
		return newJSON
	}
	if err := json.Unmarshal([]byte(oldJSON), &mo); err != nil {
		return newJSON
	}
	mn["password"] = resolveSSHSecret(sshStr(mn["password"]), sshStr(mo["password"]), previewPassword)
	mn["private_key"] = resolveSSHSecret(sshStr(mn["private_key"]), sshStr(mo["private_key"]), previewKey)
	b, err := json.Marshal(mn)
	if err != nil {
		return newJSON
	}
	return string(b)
}

func resolveSSHSecret(incoming, stored string, preview func(string) string) string {
	if stored != "" && incoming == preview(stored) {
		return stored // untouched
	}
	return incoming // complete value the user typed (possibly empty = cleared)
}
