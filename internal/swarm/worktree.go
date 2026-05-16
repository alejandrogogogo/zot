package swarm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitWorktree creates per-agent working directories via `git worktree
// add`. If the repo path isn't a git repository it falls back to a
// plain mkdir under root/<id>, which is still useful for tests and
// for running agents in non-git directories.
type gitWorktree struct {
	root string // <swarm-root>/worktrees
	repo string // user CWD; git operations resolve from here
}

// Create makes a new worktree on branch off of HEAD. The branch is
// created if it doesn't already exist. Returns the absolute path.
func (g *gitWorktree) Create(id, branch, base string) (string, error) {
	dir := filepath.Join(g.root, id)
	if err := os.MkdirAll(g.root, 0o755); err != nil {
		return "", err
	}
	if !isGitRepo(g.repo) {
		// Non-git fallback: just a fresh directory. The agent works
		// here; nothing is staged for merge.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	cmd := exec.Command("git", "worktree", "add", "-b", branch, dir)
	cmd.Dir = g.repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If the branch already exists (e.g. a leftover from a
		// previous run), retry without -b so the user doesn't need
		// to clean up by hand.
		if strings.Contains(string(out), "already exists") {
			cmd = exec.Command("git", "worktree", "add", dir, branch)
			cmd.Dir = g.repo
			out, err = cmd.CombinedOutput()
		}
	}
	if err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return dir, nil
}

// Remove deletes the worktree. Uses `git worktree remove --force` so
// dirty trees don't block cleanup; the user is the one running this
// command explicitly.
func (g *gitWorktree) Remove(id, dir string) error {
	if dir == "" {
		return errors.New("empty worktree dir")
	}
	if !isGitRepo(g.repo) {
		return os.RemoveAll(dir)
	}
	cmd := exec.Command("git", "worktree", "remove", "--force", dir)
	cmd.Dir = g.repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		// `git worktree remove` refuses to operate on an unknown path;
		// in that case fall back to a plain rmdir so the user can
		// always clean up.
		if strings.Contains(string(out), "not a working tree") {
			return os.RemoveAll(dir)
		}
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func isGitRepo(dir string) bool {
	if dir == "" {
		return false
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// memWorktree is a WorktreeManager used by tests. It does no git
// operations; it just makes a fresh subdirectory under root.
type memWorktree struct{ root string }

// MemWorktree returns a WorktreeManager that creates plain
// subdirectories. Exposed for tests and for callers running zot
// outside a git repository.
func MemWorktree(root string) WorktreeManager { return &memWorktree{root: root} }

func (m *memWorktree) Create(id, branch, base string) (string, error) {
	dir := filepath.Join(m.root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (m *memWorktree) Remove(id, dir string) error {
	if dir == "" {
		return errors.New("empty worktree dir")
	}
	return os.RemoveAll(dir)
}
