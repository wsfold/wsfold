package wsfold

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	ansiGreen      = "\x1b[32m"
	ansiCyan       = "\x1b[36m"
	ansiBold       = "\x1b[1m"
	ansiYellow     = "\x1b[33m"
	ansiRed        = "\x1b[31m"
	ansiReset      = "\x1b[0m"
	ansiGreenBold  = ansiGreen + ansiBold
	ansiCyanBold   = ansiCyan + ansiBold
	ansiYellowBold = ansiYellow + ansiBold
	ansiRedBold    = ansiRed + ansiBold
)

type App struct {
	Runner Runner
	Stdout io.Writer
	Stderr io.Writer
}

func NewApp() *App {
	return &App{
		Runner: Runner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
}

func (a *App) Summon(cwd string, ref string) error {
	return a.summon(cwd, ref, TrustClassTrusted)
}

func (a *App) SummonUntrusted(cwd string, ref string) error {
	return a.summon(cwd, ref, TrustClassExternal)
}

func (a *App) ReindexTrusted() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	repos, err := refreshTrustedRemoteIndex(cfg, a.Runner)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.Stdout, "refreshed trusted index for %d orgs (%d repos)\n", len(cfg.TrustedGitHubOrgs), len(repos))
	return nil
}

func (a *App) Init(cwd string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	primaryRoot, err := currentWorkspaceRoot(cwd)
	if err != nil {
		return err
	}

	manifest := Manifest{
		Version:     manifestVersion,
		PrimaryRoot: primaryRoot,
		Trusted:     []Entry{},
		External:    []Entry{},
	}

	if err := saveManifest(primaryRoot, manifest); err != nil {
		return err
	}
	if err := writeWorkspace(primaryRoot, manifest, cfg.ProjectsDirName); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.Stdout, "initialized %s\n", primaryRoot)
	return nil
}

func (a *App) summon(cwd string, ref string, requested TrustClass) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	primaryRoot, err := resolveWorkspaceRoot(cwd)
	if err != nil {
		return err
	}

	repo, err := findOrCloneRepo(cfg, a.Runner, a.Stdout, ref, requested)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(primaryRoot)
	if err != nil {
		return err
	}

	entry := Entry{
		RepoRef:      repo.DisplayRef(),
		CheckoutPath: repo.CheckoutPath,
		TrustClass:   requested,
	}

	if requested == TrustClassTrusted {
		entry.MountPath = filepath.Join(primaryRoot, cfg.ProjectsDirName, repo.Name)
		if err := ensureTrustedSymlink(entry.MountPath, repo.CheckoutPath); err != nil {
			return err
		}
	}

	manifest.Upsert(entry)
	if err := saveManifest(primaryRoot, manifest); err != nil {
		return err
	}
	if err := writeWorkspace(primaryRoot, manifest, cfg.ProjectsDirName); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(a.Stdout, formatSummonSuccess(requested, repo, entry, primaryRoot))
	return nil
}

func (a *App) Dismiss(cwd string, ref string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	_ = cfg

	primaryRoot, err := resolveWorkspaceRoot(cwd)
	if err != nil {
		return err
	}

	manifest, err := loadManifest(primaryRoot)
	if err != nil {
		return err
	}

	entry, ok, err := resolveManifestEntry(manifest, ref)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s repository %q is not part of the current workspace composition", ansiRedBold+"✗"+ansiReset, ref)
	}

	if entry.TrustClass == TrustClassTrusted && entry.MountPath != "" {
		if err := removeTrustedSymlink(entry.MountPath); err != nil {
			return err
		}
	}

	manifest.Remove(entry)
	if err := saveManifest(primaryRoot, manifest); err != nil {
		return err
	}
	if err := writeWorkspace(primaryRoot, manifest, cfg.ProjectsDirName); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(a.Stdout, formatDismissSuccess(entry))
	return nil
}

func ensureTrustedSymlink(linkPath, target string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("create projects directory: %w", err)
	}

	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			if removable, checkErr := isRemovableMountResidue(linkPath); checkErr != nil {
				return fmt.Errorf("inspect mount residue %s: %w", linkPath, checkErr)
			} else if !removable {
				return fmt.Errorf("mount path %s already exists and is not a symlink", linkPath)
			}
			if err := os.RemoveAll(linkPath); err != nil {
				return fmt.Errorf("remove stale mount residue %s: %w", linkPath, err)
			}
		} else {
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("replace symlink %s: %w", linkPath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat mount path %s: %w", linkPath, err)
	}

	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", linkPath, target, err)
	}
	return nil
}

func formatSummonSuccess(requested TrustClass, repo Repo, entry Entry, primaryRoot string) string {
	check := ansiGreenBold + "✓" + ansiReset
	repoRef := ansiCyanBold + repo.DisplayRef() + ansiReset

	switch requested {
	case TrustClassTrusted:
		mountDisplay := entry.MountPath
		if rel, err := filepath.Rel(primaryRoot, entry.MountPath); err == nil && rel != "" {
			mountDisplay = rel
		}
		mountPath := ansiYellowBold + mountDisplay + ansiReset
		return fmt.Sprintf("%s Trusted repository attached: %s at %s", check, repoRef, mountPath)
	case TrustClassExternal:
		return fmt.Sprintf("%s External repository added: %s", check, repoRef)
	default:
		return fmt.Sprintf("%s Repository added: %s", check, repoRef)
	}
}

func formatDismissSuccess(entry Entry) string {
	icon := ansiRedBold + "-" + ansiReset
	repoRef := ansiCyanBold + entry.RepoRef + ansiReset

	switch entry.TrustClass {
	case TrustClassTrusted:
		return fmt.Sprintf("%s Trusted repository removed: %s", icon, repoRef)
	case TrustClassExternal:
		return fmt.Sprintf("%s External repository removed: %s", icon, repoRef)
	default:
		return fmt.Sprintf("%s Repository removed: %s", icon, repoRef)
	}
}

func removeTrustedSymlink(linkPath string) error {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat symlink %s: %w", linkPath, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove symlink %s: %w", linkPath, err)
		}
		return nil
	}

	removable, err := isRemovableMountResidue(linkPath)
	if err != nil {
		return fmt.Errorf("inspect mount residue %s: %w", linkPath, err)
	}
	if !removable {
		return fmt.Errorf("mount path %s exists but is not a removable symlink residue", linkPath)
	}
	if err := os.RemoveAll(linkPath); err != nil {
		return fmt.Errorf("remove stale mount residue %s: %w", linkPath, err)
	}
	return nil
}

func isRemovableMountResidue(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}

	expected := []string{
		".git",
		filepath.Join(".git", "gk"),
		filepath.Join(".git", "gk", "config"),
	}

	for _, rel := range expected {
		info, err := os.Lstat(filepath.Join(path, rel))
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		if rel == filepath.Join(".git", "gk", "config") {
			if info.IsDir() {
				return false, nil
			}
		} else if !info.IsDir() {
			return false, nil
		}
	}

	allowed := map[string]struct{}{
		".git":                                {},
		filepath.Join(".git", "gk"):           {},
		filepath.Join(".git", "gk", "config"): {},
	}

	valid := true
	err = filepath.WalkDir(path, func(current string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == path {
			return nil
		}
		rel, relErr := filepath.Rel(path, current)
		if relErr != nil {
			return relErr
		}
		if _, ok := allowed[rel]; !ok {
			valid = false
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, nil
	}
	return valid, nil
}
