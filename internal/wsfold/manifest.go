package wsfold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const manifestVersion = 1

type Manifest struct {
	Version     int     `yaml:"version"`
	PrimaryRoot string  `yaml:"primary_root"`
	Trusted     []Entry `yaml:"trusted"`
	External    []Entry `yaml:"external"`
}

func manifestPath(primaryRoot string) string {
	return filepath.Join(primaryRoot, ".wsfold", "manifest.yaml")
}

func loadManifest(primaryRoot string) (Manifest, error) {
	path := manifestPath(primaryRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{
				Version:     manifestVersion,
				PrimaryRoot: primaryRoot,
				Trusted:     []Entry{},
				External:    []Entry{},
			}, nil
		}
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.Version == 0 {
		manifest.Version = manifestVersion
	}
	if manifest.PrimaryRoot == "" {
		manifest.PrimaryRoot = primaryRoot
	}
	sortEntries(manifest.Trusted)
	sortEntries(manifest.External)
	return manifest, nil
}

func saveManifest(primaryRoot string, manifest Manifest) error {
	manifest.Version = manifestVersion
	manifest.PrimaryRoot = primaryRoot
	sortEntries(manifest.Trusted)
	sortEntries(manifest.External)

	if err := os.MkdirAll(filepath.Dir(manifestPath(primaryRoot)), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	return os.WriteFile(manifestPath(primaryRoot), data, 0o644)
}

func (m *Manifest) Upsert(entry Entry) {
	target := &m.External
	if entry.TrustClass == TrustClassTrusted {
		target = &m.Trusted
	}

	replaced := false
	for i := range *target {
		if (*target)[i].CheckoutPath == entry.CheckoutPath || (*target)[i].RepoRef == entry.RepoRef {
			(*target)[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		*target = append(*target, entry)
	}
	sortEntries(*target)
}

func (m *Manifest) Remove(entry Entry) {
	if entry.TrustClass == TrustClassTrusted {
		m.Trusted = removeEntry(m.Trusted, entry)
		return
	}
	m.External = removeEntry(m.External, entry)
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RepoRef != entries[j].RepoRef {
			return entries[i].RepoRef < entries[j].RepoRef
		}
		return entries[i].CheckoutPath < entries[j].CheckoutPath
	})
}

func removeEntry(entries []Entry, target Entry) []Entry {
	filtered := entries[:0]
	for _, entry := range entries {
		if entry.CheckoutPath == target.CheckoutPath || entry.RepoRef == target.RepoRef {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func resolveManifestEntry(manifest Manifest, ref string) (Entry, bool, error) {
	ref = normalizeRepoRef(ref)
	all := append(append([]Entry{}, manifest.Trusted...), manifest.External...)

	var exact []Entry
	var short []Entry
	var local []Entry
	shortName := repoNameFromRef(ref)
	for _, entry := range all {
		if normalizeRepoRef(entry.RepoRef) == ref {
			exact = append(exact, entry)
		}
		if repoNameFromRef(entry.RepoRef) == shortName {
			short = append(short, entry)
		}
		if strings.EqualFold(completionFolderName(entry.CheckoutPath), ref) {
			local = append(local, entry)
		}
	}

	if len(exact) == 1 {
		return exact[0], true, nil
	}
	if len(exact) > 1 {
		return Entry{}, false, fmt.Errorf("repo ref %q is ambiguous in manifest", ref)
	}
	if len(short) == 1 {
		return short[0], true, nil
	}
	if len(short) > 1 {
		return Entry{}, false, fmt.Errorf("repo ref %q is ambiguous in manifest", ref)
	}
	if len(local) == 1 {
		return local[0], true, nil
	}
	if len(local) > 1 {
		return Entry{}, false, fmt.Errorf("repo ref %q is ambiguous in manifest", ref)
	}

	return Entry{}, false, nil
}
