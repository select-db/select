package git

import "testing"

// The status call must never panic or error even when the DB is unavailable;
// the frontend relies on it always rendering.
func TestGetGitWorkspaceStatus_NoPanicWithoutDB(t *testing.T) {
	_, _, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	st, err := g.GetGitWorkspaceStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st == nil {
		t.Fatal("nil status")
	}
	if st.ConfiguredRemoteUrl != "" {
		t.Errorf("configured remote should be empty without a DB, got %q", st.ConfiguredRemoteUrl)
	}
}
