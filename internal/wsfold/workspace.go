package wsfold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const workspaceFileName = "wsfold.code-workspace"

type workspaceFile struct {
	Folders  []workspaceFolder `json:"folders"`
	Settings map[string]any    `json:"settings"`
}

type workspaceFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func workspacePath(primaryRoot string) string {
	return filepath.Join(primaryRoot, workspaceFileName)
}

func writeWorkspace(primaryRoot string, manifest Manifest) error {
	data, err := renderWorkspace(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(workspacePath(primaryRoot), data, 0o644)
}

func renderWorkspace(manifest Manifest) ([]byte, error) {
	folders := []workspaceFolder{
		{Name: filepath.Base(manifest.PrimaryRoot), Path: manifest.PrimaryRoot},
	}
	for _, entry := range manifest.Trusted {
		folders = append(folders, workspaceFolder{
			Name: repoNameFromRef(entry.RepoRef),
			Path: entry.CheckoutPath,
		})
	}
	for _, entry := range manifest.External {
		folders = append(folders, workspaceFolder{
			Name: repoNameFromRef(entry.RepoRef),
			Path: entry.CheckoutPath,
		})
	}

	file := workspaceFile{
		Folders: folders,
		Settings: map[string]any{
			"files.exclude": map[string]bool{
				"refs": true,
			},
			"search.exclude": map[string]bool{
				"refs": true,
			},
		},
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workspace: %w", err)
	}
	return append(data, '\n'), nil
}
