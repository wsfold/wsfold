package wsfold

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func (a *App) summon(cwd string, ref string, requested TrustClass) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	primaryRoot, err := ensurePrimaryWorkspaceRoot(a.Runner, cwd)
	if err != nil {
		return err
	}

	index, err := DiscoverRepositories(cfg, a.Runner)
	if err != nil {
		return err
	}

	repo, err := findOrCloneRepo(cfg, a.Runner, index, ref, requested)
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
		entry.MountPath = filepath.Join(primaryRoot, "refs", repo.Name)
		if err := ensureTrustedSymlink(entry.MountPath, repo.CheckoutPath); err != nil {
			return err
		}
	}

	manifest.Upsert(entry)
	if err := saveManifest(primaryRoot, manifest); err != nil {
		return err
	}
	if err := writeWorkspace(primaryRoot, manifest); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(a.Stdout, "%s %s\n", requested, repo.DisplayRef())
	return nil
}

func (a *App) Dismiss(cwd string, ref string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	_ = cfg

	primaryRoot, err := ensurePrimaryWorkspaceRoot(a.Runner, cwd)
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
		return nil
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
	if err := writeWorkspace(primaryRoot, manifest); err != nil {
		return err
	}

	return nil
}

func ensureTrustedSymlink(linkPath, target string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("create refs directory: %w", err)
	}

	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("mount path %s already exists and is not a symlink", linkPath)
		}
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("replace symlink %s: %w", linkPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat mount path %s: %w", linkPath, err)
	}

	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", linkPath, target, err)
	}
	return nil
}

func removeTrustedSymlink(linkPath string) error {
	if _, err := os.Lstat(linkPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat symlink %s: %w", linkPath, err)
	}
	if err := os.Remove(linkPath); err != nil {
		return fmt.Errorf("remove symlink %s: %w", linkPath, err)
	}
	return nil
}
