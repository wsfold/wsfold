package wsfold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			Name: filepath.Base(path),
			Path: relativePath,
		})
	}
	for _, entry := range manifest.External {
		relativePath, err := workspaceRelativePath(manifest.PrimaryRoot, entry.CheckoutPath)
		if err != nil {
			return nil, err
		}
		folders = append(folders, workspaceFolder{
			Name: filepath.Base(entry.CheckoutPath),
			Path: relativePath,
		})
	}

	file := workspaceFile{
		Folders:  folders,
		Settings: workspaceSettings(manifest, projectsDirName),
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workspace: %w", err)
	}
	return append(data, '\n'), nil
}

func trustedMountPath(primaryRoot string, projectsDirName string, repoName string) string {
	if projectsDirName == "." {
		return filepath.Join(primaryRoot, repoName)
	}
	return filepath.Join(primaryRoot, projectsDirName, repoName)
}

func workspaceSettings(manifest Manifest, projectsDirName string) map[string]any {
	excludes := workspaceExcludes(manifest, projectsDirName)
	return map[string]any{
		"files.exclude":        excludes,
		"files.watcherExclude": excludes,
		"search.exclude":       excludes,
	}
}

func workspaceExcludes(manifest Manifest, projectsDirName string) map[string]bool {
	excludes := map[string]bool{}
	if projectsDirName == "." {
		names := map[string]struct{}{}
		for _, entry := range manifest.Trusted {
			if entry.MountPath == "" {
				continue
			}
			names[filepath.Base(entry.MountPath)] = struct{}{}
		}
		keys := make([]string, 0, len(names))
		for name := range names {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			excludes[name] = true
		}
		return excludes
	}

	excludes[projectsDirName] = true
	return excludes
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
