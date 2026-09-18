package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Verification is the one place where a bug is not a wrong pixel: it decides
// whether this machine runs code we published or code someone else did. So
// these tests sign with real ed25519 keys and check what the verifier accepts,
// rather than standing in a mock for the part that matters.

const (
	artifactName = "selectDb-test.zip"
	testVersion  = "1.2.3"
)

// signer produces minisign signatures the way the release tooling does, so the
// verifier under test is doing its real work.
type signer struct {
	publicKey string // base64, as baked in by -ldflags
	keyID     [8]byte
	private   ed25519.PrivateKey
}

func newSigner(t *testing.T) signer {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var keyID [8]byte
	if _, err := rand.Read(keyID[:]); err != nil {
		t.Fatalf("key id: %v", err)
	}

	// A minisign public key is "Ed" + key id + the raw ed25519 key.
	encoded := append([]byte("Ed"), keyID[:]...)
	encoded = append(encoded, public...)

	return signer{
		publicKey: base64.StdEncoding.EncodeToString(encoded),
		keyID:     keyID,
		private:   private,
	}
}

// sign returns the contents of a `.minisig` sidecar for the given payload.
func (s signer) sign(content []byte, trustedComment string) string {
	signature := ed25519.Sign(s.private, content)

	line := append([]byte("Ed"), s.keyID[:]...)
	line = append(line, signature...)

	// The global signature covers the signature and the trusted comment
	// together, which is what stops the comment being edited after the fact.
	global := ed25519.Sign(s.private, append(signature, []byte(trustedComment)...))

	return strings.Join([]string{
		"untrusted comment: signature from a test",
		base64.StdEncoding.EncodeToString(line),
		"trusted comment: " + trustedComment,
		base64.StdEncoding.EncodeToString(global),
		"",
	}, "\n")
}

// release stands in for the download host: it serves the sidecar and the
// checksums file over TLS, because the updater refuses plain HTTP.
type release struct {
	baseURL   string
	artifact  string // path to the artifact on disk
	signature string
	checksums string
}

func newRelease(t *testing.T, content []byte) *release {
	t.Helper()

	path := filepath.Join(t.TempDir(), artifactName)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	rel := &release{artifact: path}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + artifactName + ".minisig":
			if rel.signature == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(rel.signature))
		case "/checksums.sha256":
			if rel.checksums == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(rel.checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	// The server's certificate is self-signed, so the updater needs the client
	// that trusts it. Everything else about the request stays as it ships.
	previous := checkClient
	checkClient = server.Client()
	t.Cleanup(func() { checkClient = previous })

	rel.baseURL = server.URL
	return rel
}

// withEmbeddedKey bakes a public key into the build under test, the way
// production ldflags do.
func withEmbeddedKey(t *testing.T, key string) {
	t.Helper()

	previous := minisignPublicKey
	minisignPublicKey = key
	t.Cleanup(func() { minisignPublicKey = previous })
}

func checksumLine(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "  " + artifactName + "\n"
}

func TestVerifyAcceptsAGenuineRelease(t *testing.T) {
	content := []byte("a release")
	rel := newRelease(t, content)
	key := newSigner(t)
	rel.signature = key.sign(content, "version=v"+testVersion)
	withEmbeddedKey(t, key.publicKey)

	if err := verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL); err != nil {
		t.Fatalf("a correctly signed release should verify: %v", err)
	}
}

func TestVerifyRejectsSomeoneElsesKey(t *testing.T) {
	content := []byte("a release")

	t.Run("a different key entirely", func(t *testing.T) {
		rel := newRelease(t, content)
		attacker := newSigner(t)
		rel.signature = attacker.sign(content, "version=v"+testVersion)
		withEmbeddedKey(t, newSigner(t).publicKey) // ours, not theirs

		assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
	})

	// The sneakier version: the signature claims our key id, so anything that
	// only compares identifiers would wave it through.
	t.Run("our key id, their key material", func(t *testing.T) {
		rel := newRelease(t, content)
		ours := newSigner(t)
		attacker := newSigner(t)
		attacker.keyID = ours.keyID
		rel.signature = attacker.sign(content, "version=v"+testVersion)
		withEmbeddedKey(t, ours.publicKey)

		assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
	})
}

func TestVerifyRejectsATamperedArtifact(t *testing.T) {
	content := []byte("a release")
	rel := newRelease(t, content)
	key := newSigner(t)
	rel.signature = key.sign(content, "version=v"+testVersion)
	withEmbeddedKey(t, key.publicKey)

	// One byte, after signing.
	if err := os.WriteFile(rel.artifact, []byte("a reCease"), 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
}

// An old release is genuinely signed, so the only thing standing between it and
// a downgrade is the version pinned in the trusted comment.
func TestVerifyRejectsAReplayedOlderRelease(t *testing.T) {
	content := []byte("an old release")
	rel := newRelease(t, content)
	key := newSigner(t)
	rel.signature = key.sign(content, "version=v1.0.0")
	withEmbeddedKey(t, key.publicKey)

	assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
}

func TestVerifyRejectsAMalformedSignature(t *testing.T) {
	// An unset sidecar is served as a 404, which is its own scenario: a release
	// that was published without a signature must not install either.
	for name, sidecar := range map[string]string{
		"never published": "",
		"truncated":       "untrusted comment: x\nnot base64\n",
		"garbage":         "\x00\x01\x02",
	} {
		t.Run(name, func(t *testing.T) {
			rel := newRelease(t, []byte("a release"))
			rel.signature = sidecar
			withEmbeddedKey(t, newSigner(t).publicKey)

			assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
		})
	}
}

// Staging verifies by checksum because it has no key. A production build must
// not reach that path when its signature check fails, or shipping a valid
// checksums file would be enough to install anything.
func TestVerifyNeverFallsBackToChecksums(t *testing.T) {
	content := []byte("a release")
	rel := newRelease(t, content)
	rel.checksums = checksumLine(t, rel.artifact) // correct, and irrelevant
	rel.signature = newSigner(t).sign(content, "version=v"+testVersion)
	withEmbeddedKey(t, newSigner(t).publicKey)

	assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
}

func TestChecksumVerificationWithoutAKey(t *testing.T) {
	content := []byte("a staging build")
	withEmbeddedKey(t, "") // staging ships without one

	t.Run("matching digest", func(t *testing.T) {
		rel := newRelease(t, content)
		rel.checksums = checksumLine(t, rel.artifact)

		if err := verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL); err != nil {
			t.Fatalf("a matching checksum should verify: %v", err)
		}
	})

	t.Run("digest of something else", func(t *testing.T) {
		rel := newRelease(t, content)
		rel.checksums = strings.Repeat("a", 64) + "  " + artifactName + "\n"

		assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
	})

	t.Run("artifact absent from the list", func(t *testing.T) {
		rel := newRelease(t, content)
		rel.checksums = strings.Repeat("a", 64) + "  something-else.zip\n"

		assertRejected(t, verifyArtifact(testVersion, artifactName, rel.artifact, rel.baseURL))
	})
}

func assertRejected(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected the update to be rejected, but it verified")
	}
}
