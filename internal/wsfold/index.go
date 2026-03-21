package wsfold

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RepoIndex struct {
	Repos []Repo
}

func DiscoverRepositories(cfg Config, runner Runner) (RepoIndex, error) {
	var repos []Repo
	seen := map[string]struct{}{}

	for _, rootWithTrust := range []struct {
		root       string
		trustClass TrustClass
	}{
		{root: cfg.TrustedDir, trustClass: TrustClassTrusted},
		{root: cfg.ExternalDir, trustClass: TrustClassExternal},
	} {
		discovered, err := discoverReposUnderRoot(rootWithTrust.root, rootWithTrust.trustClass, runner)
		if err != nil {
			return RepoIndex{}, err
		}

		for _, repo := range discovered {
			if _, ok := seen[repo.CheckoutPath]; ok {
				continue
			}
			seen[repo.CheckoutPath] = struct{}{}
			repos = append(repos, repo)
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].TrustClass != repos[j].TrustClass {
			return repos[i].TrustClass < repos[j].TrustClass
		}
		return repos[i].CheckoutPath < repos[j].CheckoutPath
	})

	return RepoIndex{Repos: repos}, nil
}

func discoverReposUnderRoot(root string, trustClass TrustClass, runner Runner) ([]Repo, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}

	repos := make([]Repo, 0)
	seen := map[string]struct{}{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if name == ".git" {
			repoPath := filepath.Dir(path)
			if _, ok := seen[repoPath]; !ok {
				seen[repoPath] = struct{}{}
				repos = append(repos, buildRepo(repoPath, trustClass, runner))
			}

			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() && path != root && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		if d.IsDir() && name == ".git" {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}

	return repos, nil
}

func buildRepo(path string, trustClass TrustClass, runner Runner) Repo {
	path = filepath.Clean(path)
	return hydrateRepo(buildRepoWithoutOrigin(path, trustClass), runner)
}

func buildRepoWithoutOrigin(path string, trustClass TrustClass) Repo {
	path = filepath.Clean(path)
	localName := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	return Repo{
		LocalName:    localName,
		Name:         strings.ToLower(filepath.Base(path)),
		CheckoutPath: path,
		TrustClass:   trustClass,
	}
}

func hydrateRepo(repo Repo, runner Runner) Repo {
	repo.OriginURL = repoOrigin(runner, repo.CheckoutPath)
	if owner, name, ok := parseGitHubSlug(repo.OriginURL); ok {
		repo.Slug = owner + "/" + name
		repo.Name = name
	}
	return repo
}

func (idx RepoIndex) Resolve(ref string, requested TrustClass) (Repo, error) {
	ref = normalizeRepoRef(ref)

	if repo, ok := idx.resolveExactSlug(ref, requested); ok {
		return repo, nil
	}

	shortName := repoNameFromRef(ref)
	candidates := idx.byShortName(shortName, requested)
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return Repo{}, ambiguityError(ref, candidates)
	}

	return Repo{}, os.ErrNotExist
}

func (idx RepoIndex) resolveExactSlug(ref string, requested TrustClass) (Repo, bool) {
	slugMatches := make([]Repo, 0)
	for _, repo := range idx.Repos {
		if repo.Slug == strings.ToLower(ref) {
			slugMatches = append(slugMatches, repo)
		}
	}

	if len(slugMatches) == 0 {
		return Repo{}, false
	}

	filtered := filterByTrust(slugMatches, requested)
	if len(filtered) == 1 {
		return filtered[0], true
	}

	if len(filtered) == 0 && len(slugMatches) == 1 {
		return slugMatches[0], true
	}

	return Repo{}, false
}

func (idx RepoIndex) byShortName(name string, requested TrustClass) []Repo {
	matches := make([]Repo, 0)
	for _, repo := range idx.Repos {
		normalized := strings.ToLower(name)
		if repo.Name == normalized || repo.LocalName == normalized {
			matches = append(matches, repo)
		}
	}

	preferred := filterByTrust(matches, requested)
	if len(preferred) > 0 {
		return preferred
	}
	return matches
}

func filterByTrust(repos []Repo, requested TrustClass) []Repo {
	filtered := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		if repo.TrustClass == requested {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func ambiguityError(ref string, candidates []Repo) error {
	lines := make([]string, 0, len(candidates))
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].TrustClass != candidates[j].TrustClass {
			return candidates[i].TrustClass < candidates[j].TrustClass
		}
		return candidates[i].CheckoutPath < candidates[j].CheckoutPath
	})
	for _, candidate := range candidates {
		lines = append(lines, fmt.Sprintf("%s (%s)", candidate.CheckoutPath, candidate.TrustClass))
	}
	return fmt.Errorf("repo ref %q is ambiguous: %s", ref, strings.Join(lines, ", "))
}

func findOrCloneRepo(cfg Config, runner Runner, progress io.Writer, ref string, requested TrustClass) (Repo, error) {
	repo, err := resolveExistingRepo(cfg, runner, ref, requested)
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Repo{}, err
	}

	classification, owner, name, err := classifyCloneTarget(cfg, ref)
	if err != nil {
		return Repo{}, err
	}

	switch requested {
	case TrustClassTrusted:
		if classification != TrustClassTrusted {
			return Repo{}, fmt.Errorf("repo ref %q is not classified as trusted; use summon-untrusted for local external repos only", ref)
		}
	case TrustClassExternal:
		if classification == TrustClassTrusted {
			return Repo{}, fmt.Errorf("repo ref %q is classified as trusted; use summon", ref)
		}
		return Repo{}, fmt.Errorf("repo ref %q was not found locally under %s; summon-untrusted only supports local external repos", ref, cfg.ExternalDir)
	default:
		return Repo{}, fmt.Errorf("unsupported trust class %q", requested)
	}

	destination, err := chooseTrustedRepoClonePath(cfg, runner, owner, name)
	if err != nil {
		return Repo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Repo{}, fmt.Errorf("create clone parent: %w", err)
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		return buildRepo(destination, requested, runner), nil
	}

	if err := cloneTrustedGitHubRepo(runner, progress, owner, name, destination); err != nil {
		return Repo{}, err
	}

	return buildRepo(destination, requested, runner), nil
}

func cloneTrustedGitHubRepo(runner Runner, progress io.Writer, owner string, name string, destination string) error {
	probe := probeGitHubCLI(runner)
	if !probe.Ready {
		return fmt.Errorf("trusted remote clone requires GitHub CLI authentication; %s; run gh auth login", strings.TrimPrefix(probe.Message, "remote index unavailable: "))
	}

	if progress != nil {
		_, _ = fmt.Fprintf(progress, "cloning trusted repo %s into %s\n", owner+"/"+name, destination)
	}

	if _, err := runner.GitHub("", "repo", "clone", owner+"/"+name, destination); err != nil {
		return fmt.Errorf("trusted remote clone via gh repo clone failed: %w", err)
	}
	return nil
}

func chooseTrustedRepoClonePath(cfg Config, runner Runner, owner string, name string) (string, error) {
	primary := filepath.Join(cfg.TrustedDir, strings.ToLower(strings.TrimSpace(name)))
	ok, err := trustedClonePathAvailable(primary, owner, name, runner)
	if err != nil {
		return "", err
	}
	if ok {
		return primary, nil
	}

	fallback := filepath.Join(cfg.TrustedDir, trustedRepoFolderName(owner, name))
	ok, err = trustedClonePathAvailable(fallback, owner, name, runner)
	if err != nil {
		return "", err
	}
	if ok {
		return fallback, nil
	}

	return "", fmt.Errorf("trusted repo %q cannot be cloned because both %q and %q are already used by other repositories", owner+"/"+name, primary, fallback)
}

func trustedRepoFolderName(owner string, name string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "-" + strings.ToLower(strings.TrimSpace(owner))
}

func trustedClonePathAvailable(path string, owner string, name string, runner Runner) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat trusted clone path %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, nil
	}
	if !isGitRepo(path) {
		return false, nil
	}

	repo := hydrateRepo(buildRepoWithoutOrigin(path, TrustClassTrusted), runner)
	return repo.Slug == owner+"/"+name, nil
}

func resolveExistingRepo(cfg Config, runner Runner, ref string, requested TrustClass) (Repo, error) {
	if owner, name, ok := parseGitHubSlug(ref); ok {
		candidates, err := discoverReposBySlug(cfg, runner, owner+"/"+name)
		if err != nil {
			return Repo{}, err
		}
		if len(candidates) == 0 {
			return Repo{}, os.ErrNotExist
		}

		filtered := filterByTrust(candidates, requested)
		if len(filtered) == 1 {
			return filtered[0], nil
		}
		if len(filtered) > 1 {
			return Repo{}, ambiguityError(ref, filtered)
		}
		if len(candidates) == 1 {
			return candidates[0], nil
		}

		return Repo{}, ambiguityError(ref, candidates)
	}

	directRepos, err := discoverDirectRepos(cfg, requested)
	if err != nil {
		return Repo{}, err
	}

	idx := RepoIndex{Repos: directRepos}
	repo, err := idx.Resolve(ref, requested)
	if err == nil {
		return hydrateRepo(repo, runner), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Repo{}, err
	}

	return Repo{}, os.ErrNotExist
}

func discoverDirectRepos(cfg Config, requested TrustClass) ([]Repo, error) {
	repos := make([]Repo, 0)
	for _, rootWithTrust := range []struct {
		root       string
		trustClass TrustClass
	}{
		{root: cfg.TrustedDir, trustClass: TrustClassTrusted},
		{root: cfg.ExternalDir, trustClass: TrustClassExternal},
	} {
		discovered, err := discoverDirectReposUnderRoot(rootWithTrust.root, rootWithTrust.trustClass)
		if err != nil {
			return nil, err
		}
		repos = append(repos, discovered...)
	}

	return repos, nil
}

func discoverDirectReposUnderRoot(root string, trustClass TrustClass) ([]Repo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", root, err)
	}

	repos := make([]Repo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		repoPath := filepath.Join(root, entry.Name())
		if !isGitRepo(repoPath) {
			continue
		}
		repos = append(repos, buildRepoWithoutOrigin(repoPath, trustClass))
	}
	return repos, nil
}

func discoverReposBySlug(cfg Config, runner Runner, slug string) ([]Repo, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	candidates := make([]Repo, 0)

	repos, err := discoverDirectReposUnderRoot(cfg.TrustedDir, TrustClassTrusted)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		hydrated := hydrateRepo(repo, runner)
		if hydrated.Slug == slug {
			candidates = append(candidates, hydrated)
		}
	}

	if owner, name, ok := splitSlug(slug); ok {
		externalPath := filepath.Join(cfg.ExternalDir, owner, name)
		if isGitRepo(externalPath) {
			hydrated := hydrateRepo(buildRepoWithoutOrigin(externalPath, TrustClassExternal), runner)
			if hydrated.Slug == slug {
				candidates = append(candidates, hydrated)
			}
		}
	}

	return candidates, nil
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info != nil
}
