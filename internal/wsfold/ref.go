package wsfold

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var githubHTTPSPattern = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
var githubSSHPattern = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
var fileMirrorPattern = regexp.MustCompile(`^file://.*/([^/]+)/([^/]+?)(?:\.git)?$`)

func normalizeRepoRef(ref string) string {
	return strings.TrimSpace(strings.TrimSuffix(ref, ".git"))
}

func splitSlug(ref string) (string, string, bool) {
	normalized := normalizeRepoRef(ref)
	parts := strings.Split(normalized, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), true
}

func repoNameFromRef(ref string) string {
	if owner, name, ok := splitSlug(ref); ok {
		_ = owner
		return name
	}
	return strings.ToLower(filepath.Base(normalizeRepoRef(ref)))
}

func parseGitHubSlug(ref string) (owner string, repo string, ok bool) {
	if owner, repo, ok := splitSlug(ref); ok {
		return owner, repo, true
	}

	for _, pattern := range []*regexp.Regexp{githubHTTPSPattern, githubSSHPattern} {
		matches := pattern.FindStringSubmatch(strings.TrimSpace(ref))
		if len(matches) == 3 {
			return strings.ToLower(matches[1]), strings.ToLower(strings.TrimSuffix(matches[2], ".git")), true
		}
	}

	matches := fileMirrorPattern.FindStringSubmatch(strings.TrimSpace(ref))
	if len(matches) == 3 {
		return strings.ToLower(matches[1]), strings.ToLower(strings.TrimSuffix(matches[2], ".git")), true
	}

	return "", "", false
}

func classifyCloneTarget(cfg Config, ref string) (TrustClass, string, string, error) {
	owner, repo, ok := parseGitHubSlug(ref)
	if !ok {
		return TrustClassExternal, "", "", nil
	}

	if cfg.IsTrustedGitHubOrg(owner) {
		return TrustClassTrusted, owner, repo, nil
	}

	return TrustClassExternal, owner, repo, nil
}

func remoteURLFromRef(ref string) (string, string, string, error) {
	owner, repo, ok := parseGitHubSlug(ref)
	if !ok {
		return "", "", "", fmt.Errorf("cannot clone %q without an owner/name slug", ref)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), owner, repo, nil
}
