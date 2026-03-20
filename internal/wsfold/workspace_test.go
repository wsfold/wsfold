package wsfold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWorkspaceMatchesGoldenAndIsDeterministic(t *testing.T) {
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

	first, err := renderWorkspace(manifest)
	if err != nil {
		t.Fatalf("renderWorkspace returned error: %v", err)
	}
	second, err := renderWorkspace(manifest)
	if err != nil {
		t.Fatalf("renderWorkspace returned error: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("workspace rendering is not deterministic")
	}

	want, err := os.ReadFile("testdata/workspace.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	expected := strings.ReplaceAll(string(want), "{{PRIMARY_ROOT}}", root)
	expected = strings.ReplaceAll(expected, "{{PRIMARY_NAME}}", filepath.Base(root))
	if string(first) != expected {
		t.Fatalf("workspace mismatch\nwant:\n%s\ngot:\n%s", expected, string(first))
	}
}
