package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHarnessCreatesReposAndWorktrees(t *testing.T) {
	t.Parallel()

	h := NewHarness(t)

	repoPath := filepath.Join(h.TrustedRoot, "acme", "service")
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
		t.Fatalf("mkdir repo parent: %v", err)
	}
	h.InitRepo(repoPath)

	worktreePath := h.CreateWorktree(repoPath, "service-worktree")

	for _, path := range []string{
		filepath.Join(h.Workspace, ".git"),
		filepath.Join(repoPath, ".git"),
		filepath.Join(worktreePath, ".git"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected git metadata at %s: %v", path, err)
		}
	}
}
