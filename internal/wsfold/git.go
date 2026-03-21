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

func currentWorkspaceRoot(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	return abs, nil
}

func resolveWorkspaceRoot(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}

	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, ".wsfold")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .wsfold workspace found from %s upward; run `wsfold init` first", abs)
		}
		dir = parent
	}
}

func repoOrigin(runner Runner, path string) string {
	origin, err := runner.Git(path, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return origin
}
