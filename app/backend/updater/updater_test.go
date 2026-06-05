package updater

import (
	"encoding/json"
	"strings"
	"testing"
)

// End-to-end contract: the JSON the backend's /version handler emits must
// decode into versionResponse (matching json key) and the value must flow
// through to the artifact URL the client actually downloads. The literal here
// is exactly what backend.versionHandler composes for v1.2.3.
func TestReleaseBaseURLFlowsFromVersionJSON(t *testing.T) {
	const body = `{"backend":"1.2.3","min_app_version":"1.0.0","latest_app_version":"1.2.3","release_base_url":"https://dl.example.com/releases/download/v1.2.3"}`

	var ver versionResponse
	if err := json.Unmarshal([]byte(body), &ver); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ver.ReleaseBaseURL != "https://dl.example.com/releases/download/v1.2.3" {
		t.Fatalf("ReleaseBaseURL = %q", ver.ReleaseBaseURL)
	}
	if got := artifactURL(ver.LatestAppVersion, ver.ReleaseBaseURL); !strings.HasPrefix(got, "https://dl.example.com/releases/download/v1.2.3/selectDb-") {
		t.Fatalf("artifactURL = %q", got)
	}
}

// A backend-supplied base is honoured; an empty base falls back to the built-in
// location so a client on an older backend still updates.
func TestArtifactURLBaseAndFallback(t *testing.T) {
	if got := artifactURL("1.2.3", "https://dl.example.com/r/v1.2.3"); !strings.HasPrefix(got, "https://dl.example.com/r/v1.2.3/selectDb-") {
		t.Fatalf("server base not used: %q", got)
	}
	if got := artifactURL("1.2.3", ""); !strings.HasPrefix(got, "https://github.com/select-db/select/releases/download/v1.2.3/selectDb-") {
		t.Fatalf("fallback not used: %q", got)
	}
}

func TestChecksumURLsProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	if got := checksumBaseURL("1.2.3", "https://dl.example.com/v1.2.3"); got != "https://dl.example.com/v1.2.3" {
		t.Fatalf("prod base = %q", got)
	}
	if got := checksumBaseURL("1.2.3", ""); got != "https://github.com/select-db/select/releases/download/v1.2.3" {
		t.Fatalf("prod fallback = %q", got)
	}
	if got := checksumsURL("1.2.3", "https://dl.example.com/v1.2.3"); got != "https://dl.example.com/v1.2.3/checksums.sha256" {
		t.Fatalf("prod checksums = %q", got)
	}
}

// Staging pins its own internal release (staging-latest) and must ignore a
// server-supplied base, so a hostile value cannot redirect the staging download.
func TestChecksumURLsStagingIgnoresBase(t *testing.T) {
	t.Setenv("APP_ENV", "staging")

	const stagingBase = "https://github.com/select-db/select/releases/download/staging-latest"
	if got := checksumBaseURL("99", "https://evil.example.com"); got != stagingBase {
		t.Fatalf("staging base = %q", got)
	}
	if got := checksumsURL("99", "https://evil.example.com"); got != stagingBase+"/checksums.sha256" {
		t.Fatalf("staging checksums = %q", got)
	}
}
