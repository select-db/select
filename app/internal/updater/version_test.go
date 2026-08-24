package updater

import "testing"

// Which version is newer decides whether an update is offered at all. The
// interesting cases are the ones where string comparison and version
// comparison disagree, and the channels that do not use semver.
func TestSemverOrdering(t *testing.T) {
	cases := []struct {
		older, newer string
	}{
		{"0.0.1", "0.0.2"},
		{"0.9.9", "1.0.0"},
		// Lexically "1.10.0" sorts before "1.9.0".
		{"1.9.0", "1.10.0"},
		{"1.2.3", "2.0.0"},
		// Tags carry a v; the version the app reports does not.
		{"v1.2.3", "1.2.4"},
		{"1.2.3", "v1.2.4"},
		// Staging versions are commit counts, not semver.
		{"1023", "1024"},
		// A dev build is older than anything released.
		{"dev", "0.0.1"},
	}

	for _, tc := range cases {
		if !semverLess(tc.older, tc.newer) {
			t.Errorf("%s should be older than %s", tc.older, tc.newer)
		}
		if semverLess(tc.newer, tc.older) {
			t.Errorf("%s should not be older than %s — that would offer a downgrade", tc.newer, tc.older)
		}
	}
}

func TestSameVersionIsNotAnUpdate(t *testing.T) {
	for _, version := range []string{"1.2.3", "v1.2.3", "dev", "1024"} {
		if semverLess(version, version) {
			t.Errorf("%s should not be newer than itself", version)
		}
	}

	// The tag and the version it ships are the same release.
	if semverLess("v1.2.3", "1.2.3") || semverLess("1.2.3", "v1.2.3") {
		t.Error("a v prefix should not make a release newer or older than itself")
	}
}

// Every fetch the updater makes runs through this, so a downgrade to plain HTTP
// anywhere upstream — including a base URL the backend supplies — stops here.
func TestRefusesAnythingButHTTPS(t *testing.T) {
	if err := requireHTTPS("https://example.com/x"); err != nil {
		t.Errorf("https should be allowed: %v", err)
	}

	for _, raw := range []string{
		"http://example.com/x",
		"ftp://example.com/x",
		"file:///etc/passwd",
		"://nonsense",
		"",
	} {
		if err := requireHTTPS(raw); err == nil {
			t.Errorf("%q should be refused", raw)
		}
	}
}

// The backend supplies the release location so the repo can move without
// re-shipping clients, which means it is attacker-influenced input. Staging
// pins its own location and ignores it.
func TestStagingIgnoresTheSuppliedBase(t *testing.T) {
	const hostile = "https://not-us.example.com/releases"

	t.Setenv("APP_ENV", "staging")
	if got := checksumBaseURL("1.2.3", hostile); got == hostile {
		t.Errorf("staging should pin its own release location, got %q", got)
	}

	t.Setenv("APP_ENV", "production")
	if got := checksumBaseURL("1.2.3", hostile); got != hostile {
		t.Errorf("production should follow the supplied base, got %q", got)
	}
}
