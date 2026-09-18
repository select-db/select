package git

import "testing"

// The status call must never panic or error even when the DB is unavailable;
// the frontend relies on it always rendering.
func TestGetGitWorkspaceStatus_NoPanicWithoutDB(t *testing.T) {
	_, _, g, cleanup := setupTestGitRepo(t)
	defer cleanup()

	st, err := g.GetGitWorkspaceStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st == nil {
		t.Fatal("nil status")
	}
	if !st.IsGitRepo {
		t.Error("a repository set up by the fixture should read as one")
	}
	if st.HasRemote {
		t.Errorf("fixture repo has no origin, got remote %q", st.RemoteURL)
	}
}
