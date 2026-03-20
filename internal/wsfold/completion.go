package wsfold

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type CompletionCandidate struct {
	Value       string
	Description string
}

func (a *App) Complete(cwd string, command string, prefix string) ([]CompletionCandidate, error) {
	switch command {
	case "summon":
		return a.completeRepoIndex(prefix, TrustClassTrusted)
	case "summon-untrusted":
		return a.completeRepoIndex(prefix, TrustClassExternal)
	case "dismiss":
		return a.completeManifest(cwd, prefix)
	default:
		return nil, nil
	}
}

func (a *App) completeRepoIndex(prefix string, requested TrustClass) ([]CompletionCandidate, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	index, err := DiscoverRepositories(cfg, a.Runner)
	if err != nil {
		return nil, err
	}

	candidates := make([]CompletionCandidate, 0, len(index.Repos))
	seen := map[string]struct{}{}
	for _, repo := range index.Repos {
		if repo.TrustClass != requested {
			continue
		}

		value := repo.DisplayRef()
		if prefix != "" && !strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}

		description := fmt.Sprintf("%s  %s", repo.TrustClass, repo.CheckoutPath)
		candidates = append(candidates, CompletionCandidate{
			Value:       value,
			Description: description,
		})
	}

	sortCandidates(candidates)
	return candidates, nil
}

func (a *App) completeManifest(cwd string, prefix string) ([]CompletionCandidate, error) {
	primaryRoot, err := resolveWorkspaceRoot(a.Runner, cwd)
	if err != nil {
		return nil, err
	}

	manifest, err := loadManifest(primaryRoot)
	if err != nil {
		return nil, err
	}

	all := append(append([]Entry{}, manifest.Trusted...), manifest.External...)
	candidates := make([]CompletionCandidate, 0, len(all))
	seen := map[string]struct{}{}
	for _, entry := range all {
		value := entry.RepoRef
		if prefix != "" && !strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}

		description := fmt.Sprintf("%s  %s", entry.TrustClass, entry.CheckoutPath)
		candidates = append(candidates, CompletionCandidate{
			Value:       value,
			Description: description,
		})
	}

	sortCandidates(candidates)
	return candidates, nil
}

func sortCandidates(candidates []CompletionCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Value != candidates[j].Value {
			return candidates[i].Value < candidates[j].Value
		}
		return candidates[i].Description < candidates[j].Description
	})
}

func resolveWorkspaceRoot(runner Runner, cwd string) (string, error) {
	root, err := runner.Git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(root), nil
}
