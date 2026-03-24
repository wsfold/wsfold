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

	first, err := renderWorkspace(manifest, ".")
	if err != nil {
		t.Fatalf("renderWorkspace returned error: %v", err)
	}
	second, err := renderWorkspace(manifest, ".")
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
	externalRelativePath, err := filepath.Rel(root, "/external/legacy/tool")
	if err != nil {
		t.Fatalf("compute relative external path: %v", err)
	}
	expected = strings.ReplaceAll(expected, "{{EXTERNAL_RELATIVE_PATH}}", filepath.ToSlash(externalRelativePath))
	if string(first) != expected {
		t.Fatalf("workspace mismatch\nwant:\n%s\ngot:\n%s", expected, string(first))
	}
}

func TestRenderWorkspaceCustomProjectsDirKeepsSubdirExcludes(t *testing.T) {
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
				MountPath:    filepath.Join(root, "_ctx", "service"),
			},
		},
	}

	data, err := renderWorkspace(manifest, "_ctx")
	if err != nil {
		t.Fatalf("renderWorkspace returned error: %v", err)
	}
	if !strings.Contains(string(data), `"_ctx/service"`) {
		t.Fatalf("expected trusted root under custom projects dir:\n%s", string(data))
	}
	if !strings.Contains(string(data), `"_ctx": true`) {
		t.Fatalf("expected custom projects dir exclude:\n%s", string(data))
	}
}
