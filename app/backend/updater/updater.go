// Package updater checks for new versions and applies updates on user request.
//
// On startup, CheckVersion silently queries the remote /version endpoint and
// emits an "update" event with status "available" if a newer version exists.
// The user can then trigger Apply via the frontend to download, verify, install,
// and relaunch.
//
// Production builds bake the publisher's minisign public key into
// minisignPublicKey via ldflags. The signature's trusted comment must contain
// "version=vX.Y.Z" matching the version being installed (anti-rollback: an
// attacker can't replay an old signed archive against a newer version claim).
//
// Rotating the key: sign the cutover release with the OLD secret key (so
// existing clients still accept it) while baking the NEW public key into this
// build. Clients install the cutover via the old key, then trust the new key
// going forward. the only hard requirement is a backup of the secret key so it can't be lost.
//
// Staging builds leave the key empty and fall back to a SHA256 check against
// checksums.sha256. This is only acceptable on the internal staging channel;
// production refuses to update without an embedded primary key.
package updater

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jedisct1/go-minisign"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// UpdateEvent is emitted as the "update" Wails event payload.
type UpdateEvent struct {
	Status   string `json:"status"`   // "available" | "downloading" | "installing" | "error"
	Progress int    `json:"progress"` // 0-100 during download
	Version  string `json:"version"`
	Message  string `json:"message"`
}

type versionResponse struct {
	Backend          string `json:"backend"`
	MinAppVersion    string `json:"min_app_version"`
	LatestAppVersion string `json:"latest_app_version"`
	// ReleaseBaseURL is where the client downloads the release assets, supplied
	// by the backend so the repo can move without re-shipping clients. It is
	// UNTRUSTED: production still verifies the minisign signature against the
	// embedded key, so a hostile value can at most fail the update, not push code.
	ReleaseBaseURL string `json:"release_base_url"`
}

// Updater exposes update operations as Wails bindings.
type Updater struct {
	ctx           context.Context
	mu            sync.Mutex
	latestVersion string
	releaseBase   string
}

func New() *Updater {
	return &Updater{}
}

func (u *Updater) SetContext(ctx context.Context) {
	u.ctx = ctx
}

var (
	checkClient = &http.Client{Timeout: 10 * time.Second}
	dlClient    = &http.Client{Timeout: 10 * time.Minute}

	// Embedded publisher key, base64 (no comment line). Set via:
	//   -ldflags "-X selectDb/backend/updater.minisignPublicKey=..."
	// Empty in dev and staging builds; required in production.
	minisignPublicKey = ""
)

// requireHTTPS rejects non-HTTPS update transport (plaintext hop = RCE MITM)
func requireHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid update URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("refusing non-HTTPS update URL")
	}
	return nil
}

// fallbackReleaseBase is the built-in release location, used only when the
// backend does not supply release_base_url (e.g. a client talking to an older
// backend during rollout). Drop once every backend reports the field.
func fallbackReleaseBase(version string) string {
	return fmt.Sprintf("https://github.com/select-db/select/releases/download/v%s", version)
}

// checksumBaseURL is the release holding the checksums/signature files and the
// binary. In production it follows the backend-supplied base (falling back to
// the built-in location). Staging always pins github.com staging-latest and
// ignores base, so a hostile base cannot redirect the staging download.
func checksumBaseURL(version, base string) string {
	if os.Getenv("APP_ENV") == "staging" {
		return "https://github.com/select-db/select/releases/download/staging-latest"
	}
	if base != "" {
		return base
	}
	return fallbackReleaseBase(version)
}

func checksumsURL(version, base string) string {
	return checksumBaseURL(version, base) + "/checksums.sha256"
}

// CheckVersion fetches /version and emits an "available" event if a newer
// version exists. Does not download or install anything.
func (u *Updater) CheckVersion() {
	current := os.Getenv("APP_VERSION")
	if current == "" || current == "dev" {
		return
	}

	if os.Getenv("APP_ENV") == "production" && minisignPublicKey == "" {
		return
	}

	cleanupBak()

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		return
	}
	if err := requireHTTPS(apiURL); err != nil {
		return
	}

	resp, err := checkClient.Get(apiURL + "/version") // #nosec G704 -- apiURL is the app's configured backend; HTTPS enforced by requireHTTPS
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var ver versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return
	}
	if !semverLess(current, ver.LatestAppVersion) {
		return
	}

	u.mu.Lock()
	u.latestVersion = ver.LatestAppVersion
	u.releaseBase = ver.ReleaseBaseURL
	u.mu.Unlock()

	emit(u.ctx, UpdateEvent{Status: "available", Version: ver.LatestAppVersion})
}

// Apply downloads, verifies, installs, and relaunches the latest version.
// Called from the frontend when the user accepts the update.
func (u *Updater) Apply() {
	u.mu.Lock()
	latest := u.latestVersion
	base := u.releaseBase
	u.mu.Unlock()

	if latest == "" {
		return
	}

	ctx := u.ctx
	emit(ctx, UpdateEvent{Status: "downloading", Progress: 0, Version: latest, Message: "Downloading update..."})

	tmpDir, err := os.MkdirTemp("", "selectdb-update-")
	if err != nil {
		emit(ctx, UpdateEvent{Status: "error", Message: fmt.Sprintf("Failed to create temp dir: %v", err)})
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var dlURL, archiveName string
	if os.Getenv("APP_ENV") == "staging" {
		platform := goruntime.GOOS + "-" + goruntime.GOARCH
		archiveName = "selectDb-" + platform + ".zip"
		dlURL = checksumBaseURL(latest, base) + "/" + archiveName
	} else {
		dlURL = artifactURL(latest, base)
		archiveName = filepath.Base(dlURL)
	}
	if err := requireHTTPS(dlURL); err != nil {
		emit(ctx, UpdateEvent{Status: "error", Version: latest, Message: "Update channel is not secure"})
		return
	}
	archivePath := filepath.Join(tmpDir, archiveName)

	if err := download(ctx, dlURL, archivePath, latest); err != nil {
		emit(ctx, UpdateEvent{Status: "error", Version: latest, Message: fmt.Sprintf("Download failed: %v", err)})
		return
	}

	emit(ctx, UpdateEvent{Status: "downloading", Progress: 100, Version: latest, Message: "Verifying..."})
	if err := verifyArtifact(latest, archiveName, archivePath, base); err != nil {
		emit(ctx, UpdateEvent{Status: "error", Version: latest, Message: "Update verification failed"})
		return
	}

	emit(ctx, UpdateEvent{Status: "installing", Version: latest, Message: "Installing update..."})

	if err := install(archivePath, tmpDir); err != nil {
		emit(ctx, UpdateEvent{Status: "error", Version: latest, Message: fmt.Sprintf("Install failed: %v", err)})
		return
	}

	wailsruntime.Quit(ctx)
}

func emit(ctx context.Context, e UpdateEvent) {
	wailsruntime.EventsEmit(ctx, "update", e)
}

// cleanupBak removes the .bak left by atomicSwap on a previous update.
func cleanupBak() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if goruntime.GOOS == "darwin" {
		if bundle := findAppBundle(exe); bundle != "" {
			_ = os.RemoveAll(bundle + ".bak")
		}
		return
	}
	_ = os.RemoveAll(exe + ".bak")
	analyzer := "analyzer"
	if goruntime.GOOS == "windows" {
		analyzer = "analyzer.exe"
	}
	_ = os.RemoveAll(filepath.Join(filepath.Dir(exe), analyzer+".bak"))
}

// artifactURL returns the release asset URL for the current platform under
// base (the backend-supplied release_base_url). An empty base falls back to the
// built-in location so a client on an older backend still updates.
func artifactURL(version, base string) string {
	if base == "" {
		base = fallbackReleaseBase(version)
	}
	switch goruntime.GOOS {
	case "darwin":
		return fmt.Sprintf("%s/selectDb-darwin-%s.zip", base, goruntime.GOARCH)
	case "windows":
		return fmt.Sprintf("%s/selectDb-windows-amd64.zip", base)
	default:
		return fmt.Sprintf("%s/selectDb-linux-amd64.zip", base)
	}
}

// download fetches url into dest, emitting progress events along the way.
func download(ctx context.Context, url, dest, version string) error {
	resp, err := dlClient.Get(url) // #nosec G704 -- url is HTTPS-enforced; the artifact is minisign-verified before use
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	total := resp.ContentLength
	var written int64
	lastPct := -1
	buf := make([]byte, 32*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 {
				if pct := int(written * 99 / total); pct != lastPct {
					emit(ctx, UpdateEvent{Status: "downloading", Progress: pct, Version: version})
					lastPct = pct
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// verifyArtifact dispatches to the strongest verifier available: minisign
// signature when a publisher key is embedded, SHA256 checksum otherwise.
// The production guard in Check guarantees the SHA256 fallback is only
// reached on the staging channel.
// The dispatch keys off the embedded minisignPublicKey, never the backend
// response, so a hostile release_base_url cannot downgrade production to the
// SHA256 path.
func verifyArtifact(version, filename, filePath, base string) error {
	if minisignPublicKey != "" {
		return verifySignature(version, filename, filePath, base)
	}
	return verifyChecksum(version, filename, filePath, base)
}

// verifySignature downloads the .minisig sidecar from the same release as the
// archive, picks the embedded key whose ID matches the signature, and
// verifies. The trusted comment must include "version=v<X.Y.Z>" matching the
// version being installed, so an old signed archive cannot be replayed
// against a newer version claim.
func verifySignature(version, filename, filePath, base string) error {
	sigBase := checksumBaseURL(version, base)
	sigBytes, err := fetchAll(sigBase + "/" + filename + ".minisig")
	if err != nil {
		return err
	}
	sig, err := minisign.DecodeSignature(string(sigBytes))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	expectedTag := "version=v" + strings.TrimPrefix(version, "v")
	if !strings.Contains(sig.TrustedComment, expectedTag) {
		return fmt.Errorf("signature does not pin %s", expectedTag)
	}

	pub, err := selectPublicKey(sig.KeyId)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	ok, err := pub.Verify(content, sig)
	if err != nil {
		return fmt.Errorf("signature verification: %w", err)
	}
	if !ok {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// selectPublicKey returns the embedded key if its ID matches the signature.
// A malformed embedded key is treated as a build-time bug and surfaced loudly.
func selectPublicKey(sigKeyID [8]byte) (*minisign.PublicKey, error) {
	if minisignPublicKey == "" {
		return nil, fmt.Errorf("no embedded key matches signature key ID")
	}
	pub, err := minisign.NewPublicKey(minisignPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded public key: %w", err)
	}
	if pub.KeyId != sigKeyID {
		return nil, fmt.Errorf("no embedded key matches signature key ID")
	}
	return &pub, nil
}

// verifyChecksum is the legacy SHA256-only verifier used by the staging
// channel. Trust is TLS plus checksums.sha256 being on the same GitHub
// release, which is sufficient for an internal channel but not for users.
func verifyChecksum(version, filename, filePath, base string) error {
	checksums, err := fetchAll(checksumsURL(version, base))
	if err != nil {
		return err
	}

	var expected string
	for _, line := range strings.Split(string(checksums), "\n") {
		if parts := strings.Fields(line); len(parts) == 2 && parts[1] == filename {
			expected = strings.ToLower(parts[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("artifact %q not in checksums", filename)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if actual := hex.EncodeToString(h.Sum(nil)); actual != expected {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

// fetchAll GETs an HTTPS url, capped at 1 MiB, erroring on non-200
func fetchAll(rawURL string) ([]byte, error) {
	if err := requireHTTPS(rawURL); err != nil {
		return nil, err
	}
	resp, err := checkClient.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// fileExists is a tiny os.Stat shortcut for the analyzer-sibling swap path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// atomicSwap replaces dst with src:
//  1. Copy src → dst+".new"  (same filesystem → atomic rename guaranteed)
//  2. Rename dst → dst+".bak"
//  3. Rename dst+".new" → dst
//
// The .bak is cleaned up by cleanupBak() on the next successful startup.
func atomicSwap(src, dst string) error {
	pending, backup := dst+".new", dst+".bak"

	_ = os.RemoveAll(pending)

	if err := copyPath(src, pending); err != nil {
		_ = os.RemoveAll(pending)
		return fmt.Errorf("copy: %w", err)
	}
	if info, err := os.Stat(dst); err == nil {
		_ = os.Chmod(pending, info.Mode())
	}

	_ = os.RemoveAll(backup)
	if err := os.Rename(dst, backup); err != nil {
		_ = os.RemoveAll(pending)
		return fmt.Errorf("backup: %w", err)
	}
	if err := os.Rename(pending, dst); err != nil {
		_ = os.Rename(backup, dst) // restore
		_ = os.RemoveAll(pending)
		return fmt.Errorf("swap: %w", err)
	}
	return nil
}

// copyPath copies src to dst.
// Directories are copied with ditto on macOS (preserves symlinks/resource forks)
// or recursively on other platforms. Files are copied directly.
func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	if goruntime.GOOS == "darwin" {
		return exec.Command("ditto", src, dst).Run()
	}
	return copyDirRecursive(src, dst)
}

func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirRecursive(s, d); err != nil {
				return err
			}
		} else {
			info, _ := e.Info()
			if err := copyFile(s, d, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// findAppBundle walks up the path to find the enclosing .app bundle.
// e.g. "/Applications/Select.app/Contents/MacOS/select" → "/Applications/Select.app"
func findAppBundle(exe string) string {
	parts := strings.Split(filepath.Clean(exe), string(filepath.Separator))
	for i, p := range parts {
		if strings.HasSuffix(p, ".app") {
			return string(filepath.Separator) + filepath.Join(parts[1:i+1]...)
		}
	}
	return ""
}

// unzip extracts a zip archive into dest, preserving directory structure.
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	destClean := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(dest, filepath.Clean(f.Name))
		if !strings.HasPrefix(target, destClean) {
			return fmt.Errorf("invalid path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o700)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		// Ignore archive modes (setuid/world-writable); owner-only, keep +x so
		// Mach-O binaries in a .app still run
		mode := os.FileMode(0o600)
		if f.Mode().Perm()&0o111 != 0 {
			mode = 0o700
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			_ = out.Close()
			return err
		}
		// Cap per-entry size as a decompression-bomb guard. The archive is
		// signature-verified before we reach here, so this is defence in depth.
		const maxEntryBytes = 1 << 30 // 1 GiB
		written, copyErr := io.Copy(out, io.LimitReader(rc, maxEntryBytes+1))
		_ = rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return copyErr
		}
		if written > maxEntryBytes {
			return fmt.Errorf("archive entry %q exceeds size limit", f.Name)
		}
	}
	return nil
}

// semverLess returns true if a < b.
func semverLess(a, b string) bool { return parseSemver(a) < parseSemver(b) }

func parseSemver(v string) int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	maj, _ := strconv.Atoi(parts[0])
	min, _ := strconv.Atoi(parts[1])
	pat, _ := strconv.Atoi(parts[2])
	return maj*1_000_000 + min*1_000 + pat
}
