package wsfold

import (
	"errors"
	"fmt"
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
	localName := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	repo := Repo{
		LocalName:    localName,
		Name:         strings.ToLower(filepath.Base(path)),
		CheckoutPath: path,
		TrustClass:   trustClass,
	}

	repo.OriginURL = repoOrigin(runner, path)
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

func findOrCloneRepo(cfg Config, runner Runner, idx RepoIndex, ref string, requested TrustClass) (Repo, error) {
	repo, err := idx.Resolve(ref, requested)
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
			return Repo{}, fmt.Errorf("repo ref %q is not classified as trusted; use summon-untrusted", ref)
		}
	case TrustClassExternal:
		if classification == TrustClassTrusted {
			return Repo{}, fmt.Errorf("repo ref %q is classified as trusted; use summon", ref)
		}
	default:
		return Repo{}, fmt.Errorf("unsupported trust class %q", requested)
	}

	remoteURL, owner, name, err := remoteURLFromRef(ref)
	if err != nil {
		return Repo{}, err
	}

	root := cfg.ExternalDir
	if requested == TrustClassTrusted {
		root = cfg.TrustedDir
	}
	destination := filepath.Join(root, owner, name)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return Repo{}, fmt.Errorf("create clone parent: %w", err)
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		return buildRepo(destination, requested, runner), nil
	}

	if _, err := runner.Git("", "clone", remoteURL, destination); err != nil {
		return Repo{}, err
	}
	if _, err := runner.Git(destination, "remote", "set-url", "origin", remoteURL); err != nil {
		return Repo{}, err
	}

	return buildRepo(destination, requested, runner), nil
}
