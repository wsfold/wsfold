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
				MountPath:    filepath.Join(root, "refs", "service"),
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
