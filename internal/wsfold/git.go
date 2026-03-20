package wsfold

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Runner struct {
	Env []string
}

func (r Runner) Git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, message)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func ensurePrimaryWorkspaceRoot(runner Runner, cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}

	root, err := runner.Git(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("current directory must be a git repository or worktree root: %w", err)
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve git root: %w", err)
	}

	if filepath.Clean(root) != filepath.Clean(abs) {
		return "", fmt.Errorf("current directory must be the repository or worktree root: %s", abs)
	}

	return abs, nil
}

func repoOrigin(runner Runner, path string) string {
	origin, err := runner.Git(path, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return origin
}
