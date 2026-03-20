package wsfold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type workspaceFile struct {
	Folders  []workspaceFolder `json:"folders"`
	Settings map[string]any    `json:"settings"`
}

type workspaceFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func workspacePath(primaryRoot string) string {
	return filepath.Join(primaryRoot, filepath.Base(primaryRoot)+".code-workspace")
}

func writeWorkspace(primaryRoot string, manifest Manifest, projectsDirName string) error {
	data, err := renderWorkspace(manifest, projectsDirName)
	if err != nil {
		return err
	}
	return os.WriteFile(workspacePath(primaryRoot), data, 0o644)
}

func renderWorkspace(manifest Manifest, projectsDirName string) ([]byte, error) {
	folders := []workspaceFolder{
		{Name: filepath.Base(manifest.PrimaryRoot), Path: "."},
	}
	for _, entry := range manifest.Trusted {
		path := entry.MountPath
		if path == "" {
			path = entry.CheckoutPath
		}
		relativePath, err := workspaceRelativePath(manifest.PrimaryRoot, path)
		if err != nil {
			return nil, err
		}
		folders = append(folders, workspaceFolder{
			Name: repoNameFromRef(entry.RepoRef),
			Path: relativePath,
		})
	}
	for _, entry := range manifest.External {
		relativePath, err := workspaceRelativePath(manifest.PrimaryRoot, entry.CheckoutPath)
		if err != nil {
			return nil, err
		}
		folders = append(folders, workspaceFolder{
			Name: repoNameFromRef(entry.RepoRef),
			Path: relativePath,
		})
	}

	file := workspaceFile{
		Folders: folders,
		Settings: map[string]any{
			"files.exclude": map[string]bool{
				projectsDirName: true,
			},
			"files.watcherExclude": map[string]bool{
				projectsDirName: true,
			},
			"search.exclude": map[string]bool{
				projectsDirName: true,
			},
		},
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workspace: %w", err)
	}
	return append(data, '\n'), nil
}

func workspaceRelativePath(primaryRoot string, targetPath string) (string, error) {
	relativePath, err := filepath.Rel(primaryRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("compute relative workspace path for %s: %w", targetPath, err)
	}
	if relativePath == "." {
		return ".", nil
	}
	return filepath.ToSlash(relativePath), nil
}
