package wsfold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CompletionCandidate struct {
	Key         string
	Value       string
	Description string
	Attached    bool
	TrustClass  TrustClass
	Name        string
	Slug        string
	Source      CompletionSource
}

type TrustedSummonPickerState struct {
	Candidates []CompletionCandidate
	Refreshing bool
	Status     string
}

func (a *App) Complete(cwd string, command string, prefix string) ([]CompletionCandidate, error) {
	switch command {
	case "summon":
		return a.completeRepoIndex(cwd, prefix, TrustClassTrusted)
	case "summon-external":
		return a.completeRepoIndex(cwd, prefix, TrustClassExternal)
	case "dismiss":
		return a.completeManifest(cwd, prefix)
	default:
		return nil, nil
	}
}

func (a *App) TrustedSummonPickerState(cwd string) (TrustedSummonPickerState, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return TrustedSummonPickerState{}, err
	}

	localCandidates, err := trustedLocalCompletionCandidates(cwd, cfg.TrustedDir, a.Runner)
	if err != nil {
		return TrustedSummonPickerState{}, err
	}

	remoteState, err := trustedRemoteIndexState(cfg, a.Runner)
	if err != nil {
		return TrustedSummonPickerState{}, err
	}

	return TrustedSummonPickerState{
		Candidates: mergeTrustedSummonCandidates(localCandidates, trustedRemoteCompletionCandidates(remoteState.Repos)),
		Refreshing: remoteState.NeedsRefresh && remoteState.GitHubReady,
		Status:     remoteState.StatusMessage,
	}, nil
}

func (a *App) RefreshTrustedSummonPickerState(cwd string) (TrustedSummonPickerState, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return TrustedSummonPickerState{}, err
	}

	refreshErr := error(nil)
	if _, err := refreshTrustedRemoteIndex(cfg, a.Runner); err != nil {
		refreshErr = err
	}
	state, err := a.TrustedSummonPickerState(cwd)
	if err != nil {
		return TrustedSummonPickerState{}, err
	}
	return state, refreshErr
}

func (a *App) completeRepoIndex(cwd string, prefix string, requested TrustClass) ([]CompletionCandidate, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	root := cfg.ExternalDir
	if requested == TrustClassTrusted {
		root = cfg.TrustedDir
	}

	repos, err := discoverCompletionRepos(root, requested, a.Runner)
	if err != nil {
		return nil, err
	}

	attached := attachedCheckoutPaths(cwd)
	candidates := completionCandidatesFromRepos(repos, attached, prefix)

	sortCandidates(candidates)
	return candidates, nil
}

func trustedLocalCompletionCandidates(cwd string, root string, runner Runner) ([]CompletionCandidate, error) {
	repos, err := discoverCompletionRepos(root, TrustClassTrusted, runner)
	if err != nil {
		return nil, err
	}
	return completionCandidatesFromRepos(repos, attachedCheckoutPaths(cwd), ""), nil
}

func discoverCompletionRepos(root string, trustClass TrustClass, runner Runner) ([]Repo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read completion root %s: %w", root, err)
	}

	repos := make([]Repo, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		repoPath := filepath.Join(root, entry.Name())
		gitPath := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitPath); err != nil {
			continue
		}

		repos = append(repos, buildRepo(repoPath, trustClass, runner))
	}

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Name != repos[j].Name {
			return repos[i].Name < repos[j].Name
		}
		return repos[i].CheckoutPath < repos[j].CheckoutPath
	})

	return repos, nil
}

func (a *App) completeManifest(cwd string, prefix string) ([]CompletionCandidate, error) {
	primaryRoot, err := resolveWorkspaceRoot(cwd)
	if err != nil {
		return nil, err
	}

	manifest, err := loadManifest(primaryRoot)
	if err != nil {
		return nil, err
	}

	all := append(append([]Entry{}, manifest.Trusted...), manifest.External...)
	valueByPath := preferredManifestValues(all)
	candidates := make([]CompletionCandidate, 0, len(all))
	seen := map[string]struct{}{}
	for _, entry := range all {
		value := valueByPath[entry.CheckoutPath]
		key := entry.Key()
		if prefix != "" && !strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		description := completionDescription(entry.RepoRef, entry.CheckoutPath)
		candidates = append(candidates, CompletionCandidate{
			Key:         key,
			Value:       value,
			Description: description,
			Attached:    true,
			TrustClass:  entry.TrustClass,
			Name:        completionFolderName(entry.CheckoutPath),
			Source:      CompletionSourceLocal,
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

func completionCandidatesFromRepos(repos []Repo, attached map[string]bool, prefix string) []CompletionCandidate {
	valueByPath := preferredCompletionValues(repos)
	candidates := make([]CompletionCandidate, 0, len(repos))
	seen := map[string]struct{}{}
	for _, repo := range repos {
		value := valueByPath[repo.CheckoutPath]
		key := repoCompletionKey(repo)
		if prefix != "" && !strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		description := completionDescription(repo.OriginURL, repo.CheckoutPath)
		candidates = append(candidates, CompletionCandidate{
			Key:         key,
			Value:       value,
			Description: description,
			Attached:    attached[repo.CheckoutPath],
			TrustClass:  repo.TrustClass,
			Name:        completionFolderName(repo.CheckoutPath),
			Slug:        repo.Slug,
			Source:      CompletionSourceLocal,
		})
	}
	return candidates
}

func trustedRemoteCompletionCandidates(repos []TrustedRemoteRepo) []CompletionCandidate {
	candidates := make([]CompletionCandidate, 0, len(repos))
	for _, repo := range repos {
		if repo.Archived || strings.TrimSpace(repo.FullName) == "" {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(repo.Name))
		if name == "" {
			_, parsedName, ok := parseGitHubSlug(repo.FullName)
			if ok {
				name = parsedName
			}
		}
		candidates = append(candidates, CompletionCandidate{
			Key:         trustedRemoteCandidateKey(repo),
			Value:       repo.FullName,
			Description: repo.FullName,
			TrustClass:  TrustClassTrusted,
			Name:        name,
			Slug:        repo.FullName,
			Source:      CompletionSourceRemote,
		})
	}
	sortCandidates(candidates)
	return candidates
}

func mergeTrustedSummonCandidates(local []CompletionCandidate, remote []CompletionCandidate) []CompletionCandidate {
	merged := make([]CompletionCandidate, 0, len(local)+len(remote))
	localBySlug := map[string]struct{}{}

	for _, candidate := range local {
		merged = append(merged, candidate)
		if candidate.Slug != "" {
			localBySlug[strings.ToLower(candidate.Slug)] = struct{}{}
		}
	}

	for _, candidate := range remote {
		if candidate.Slug != "" {
			if _, ok := localBySlug[strings.ToLower(candidate.Slug)]; ok {
				continue
			}
		}
		merged = append(merged, candidate)
	}

	sort.Slice(merged, func(i, j int) bool {
		leftName := merged[i].Name
		if leftName == "" {
			leftName = merged[i].Value
		}
		rightName := merged[j].Name
		if rightName == "" {
			rightName = merged[j].Value
		}
		if leftName != rightName {
			return leftName < rightName
		}
		if merged[i].Source != merged[j].Source {
			return merged[i].Source < merged[j].Source
		}
		return merged[i].Value < merged[j].Value
	})
	return merged
}

func preferredCompletionValues(repos []Repo) map[string]string {
	counts := map[string]int{}
	for _, repo := range repos {
		counts[completionFolderName(repo.CheckoutPath)]++
	}

	values := map[string]string{}
	for _, repo := range repos {
		name := completionFolderName(repo.CheckoutPath)
		if counts[name] == 1 {
			values[repo.CheckoutPath] = name
			continue
		}
		values[repo.CheckoutPath] = repo.DisplayRef()
	}
	return values
}

func preferredManifestValues(entries []Entry) map[string]string {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[completionFolderName(entry.CheckoutPath)]++
	}

	values := map[string]string{}
	for _, entry := range entries {
		name := completionFolderName(entry.CheckoutPath)
		if counts[name] == 1 {
			values[entry.CheckoutPath] = name
			continue
		}
		values[entry.CheckoutPath] = entry.RepoRef
	}
	return values
}

func completionFolderName(path string) string {
	return filepath.Base(strings.TrimSpace(path))
}

func completionDescription(source string, checkoutPath string) string {
	_ = checkoutPath

	if owner, repo, ok := parseGitHubSlug(source); ok {
		return owner + "/" + repo
	} else if trimmed := strings.TrimSpace(source); trimmed != "" {
		return trimmed
	}
	return ""
}

func attachedCheckoutPaths(cwd string) map[string]bool {
	attached := map[string]bool{}

	primaryRoot, err := resolveWorkspaceRoot(cwd)
	if err != nil {
		return attached
	}

	manifest, err := loadManifest(primaryRoot)
	if err != nil {
		return attached
	}

	for _, entry := range manifest.Trusted {
		attached[entry.CheckoutPath] = true
	}
	for _, entry := range manifest.External {
		attached[entry.CheckoutPath] = true
	}

	return attached
}

func repoCompletionKey(repo Repo) string {
	identity := strings.TrimSpace(repo.Slug)
	if identity == "" {
		identity = strings.TrimSpace(repo.CheckoutPath)
	}
	return fmt.Sprintf("%s|%s", repo.TrustClass, strings.ToLower(identity))
}

func trustedRemoteCandidateKey(repo TrustedRemoteRepo) string {
	return fmt.Sprintf("%s|%s", TrustClassTrusted, strings.ToLower(strings.TrimSpace(repo.FullName)))
}
