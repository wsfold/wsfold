package wsfold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRoundTripMatchesGolden(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manifest := Manifest{
		Version:     manifestVersion,
		PrimaryRoot: root,
		Trusted: []Entry{
			{
				RepoRef:      "acme/service",
				CheckoutPath: "/trusted/acme/service",
				TrustClass:   TrustClassTrusted,
				MountPath:    filepath.Join(root, "service"),
			},
		},
		External: []Entry{
			{
				RepoRef:      "legacy/tool",
				CheckoutPath: "/external/legacy/tool",
				TrustClass:   TrustClassExternal,
			},
		},
	}

	if err := saveManifest(root, manifest); err != nil {
		t.Fatalf("saveManifest returned error: %v", err)
	}

	got, err := os.ReadFile(manifestPath(root))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	want, err := os.ReadFile("testdata/manifest.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	expected := string(want)
	expected = strings.ReplaceAll(expected, "{{PRIMARY_ROOT}}", root)
	if string(got) != expected {
		t.Fatalf("manifest mismatch\nwant:\n%s\ngot:\n%s", expected, string(got))
	}

	loaded, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest returned error: %v", err)
	}
	if len(loaded.Trusted) != 1 || len(loaded.External) != 1 {
		t.Fatalf("unexpected loaded manifest: %#v", loaded)
	}
}

func TestResolveManifestEntryReturnsAmbiguityErrorWithFullRepoGuidance(t *testing.T) {
	manifest := Manifest{
		Trusted: []Entry{
			{RepoRef: "acme/service", CheckoutPath: "/trusted/service", TrustClass: TrustClassTrusted},
		},
		External: []Entry{
			{RepoRef: "other/service", CheckoutPath: "/external/service", TrustClass: TrustClassExternal},
		},
	}

	_, ok, err := resolveManifestEntry(manifest, "service")
	if ok {
		t.Fatal("did not expect ambiguous short ref to resolve")
	}
	if err == nil {
		t.Fatal("expected ambiguity error for duplicate short ref")
	}
	if !strings.Contains(err.Error(), `repository ref "service" is ambiguous; use the full repo name, for example acme/service`) {
		t.Fatalf("unexpected ambiguity error: %v", err)
	}
}

func TestResolveManifestEntryAcceptsFullRepoNameWhenShortNameIsAmbiguous(t *testing.T) {
	manifest := Manifest{
		Trusted: []Entry{
			{RepoRef: "acme/service", CheckoutPath: "/trusted/service", TrustClass: TrustClassTrusted},
		},
		External: []Entry{
			{RepoRef: "other/service", CheckoutPath: "/external/service", TrustClass: TrustClassExternal},
		},
	}

	entry, ok, err := resolveManifestEntry(manifest, "other/service")
	if err != nil {
		t.Fatalf("resolveManifestEntry returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected exact repo ref to resolve")
	}
	if entry.RepoRef != "other/service" || entry.TrustClass != TrustClassExternal {
		t.Fatalf("unexpected resolved entry: %#v", entry)
	}
}
