package wsfold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func writeWorkspace(primaryRoot string, previous Manifest, manifest Manifest, projectsDirName string) error {
	data, err := renderWorkspace(primaryRoot, previous, manifest, projectsDirName)
	if err != nil {
		return err
	}
	return os.WriteFile(workspacePath(primaryRoot), data, 0o644)
}

func renderWorkspace(primaryRoot string, previous Manifest, manifest Manifest, projectsDirName string) ([]byte, error) {
	existing, err := loadWorkspaceFile(workspacePath(primaryRoot))
	if err != nil {
		return nil, err
	}

	merged, err := mergeWorkspaceFile(existing, previous, manifest, projectsDirName)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workspace: %w", err)
	}
	return append(data, '\n'), nil
}

func loadWorkspaceFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read workspace: %w", err)
	}

	normalized, err := normalizeWorkspaceJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse workspace: %w", err)
	}

	var file map[string]any
	if err := json.Unmarshal(normalized, &file); err != nil {
		return nil, fmt.Errorf("unmarshal workspace: %w", err)
	}
	if file == nil {
		return map[string]any{}, nil
	}
	return file, nil
}

func mergeWorkspaceFile(existing map[string]any, previous Manifest, manifest Manifest, projectsDirName string) (map[string]any, error) {
	merged := cloneMap(existing)
	merged["folders"] = mergeWorkspaceFolders(existing["folders"], previous, manifest)
	merged["settings"] = mergeWorkspaceSettings(existing["settings"], previous, manifest, projectsDirName)
	return merged, nil
}

func workspaceFolders(manifest Manifest) ([]workspaceFolder, error) {
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

	return folders, nil
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

func mergeWorkspaceFolders(existing any, previous Manifest, manifest Manifest) []any {
	currentFolders, err := workspaceFolders(manifest)
	if err != nil {
		currentFolders = []workspaceFolder{{Name: filepath.Base(manifest.PrimaryRoot), Path: "."}}
	}
	previousFolders, err := workspaceFolders(previous)
	if err != nil {
		previousFolders = []workspaceFolder{}
	}

	currentByPath := make(map[string]workspaceFolder, len(currentFolders))
	for _, folder := range currentFolders {
		currentByPath[folder.Path] = folder
	}
	previousPaths := make(map[string]struct{}, len(previousFolders))
	for _, folder := range previousFolders {
		previousPaths[folder.Path] = struct{}{}
	}

	items, ok := existing.([]any)
	if !ok {
		items = nil
	}

	used := map[string]struct{}{}
	result := make([]any, 0, len(items)+len(currentFolders))
	for _, item := range items {
		folder, ok := item.(map[string]any)
		if !ok {
			result = append(result, item)
			continue
		}

		path, _ := folder["path"].(string)
		_, wasManaged := previousPaths[path]
		current, isManaged := currentByPath[path]
		if path == "." || wasManaged || isManaged {
			if !isManaged {
				continue
			}
			if _, seen := used[path]; seen {
				continue
			}
			updated := cloneMap(folder)
			updated["name"] = current.Name
			updated["path"] = current.Path
			result = append(result, updated)
			used[path] = struct{}{}
			continue
		}

		result = append(result, item)
	}

	var missing []any
	for _, folder := range currentFolders {
		if _, ok := used[folder.Path]; ok {
			continue
		}
		missing = append(missing, map[string]any{
			"name": folder.Name,
			"path": folder.Path,
		})
	}

	if len(missing) == 0 {
		return result
	}
	if currentFolders[0].Path == "." {
		if _, ok := used["."]; !ok {
			return append(missing[:1], append(result, missing[1:]...)...)
		}
	}
	return append(result, missing...)
}

func mergeWorkspaceSettings(existing any, previous Manifest, manifest Manifest, projectsDirName string) map[string]any {
	settings, ok := existing.(map[string]any)
	if !ok {
		settings = map[string]any{}
	}
	merged := cloneMap(settings)

	previousExcludes := workspaceExcludes(previous, projectsDirName)
	currentExcludes := workspaceExcludes(manifest, projectsDirName)
	for _, key := range []string{"files.exclude", "files.watcherExclude", "search.exclude"} {
		merged[key] = mergeExcludeSetting(settings[key], previousExcludes, currentExcludes)
	}

	return merged
}

func mergeExcludeSetting(existing any, previous map[string]bool, current map[string]bool) map[string]any {
	section, ok := existing.(map[string]any)
	if !ok {
		section = map[string]any{}
	}
	merged := cloneMap(section)
	for key := range previous {
		if _, stillManaged := current[key]; stillManaged {
			continue
		}
		delete(merged, key)
	}
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		merged[key] = true
	}
	return merged
}

func normalizeWorkspaceJSON(data []byte) ([]byte, error) {
	withoutComments, err := stripJSONCComments(data)
	if err != nil {
		return nil, err
	}
	return stripJSONCTrailingCommas(withoutComments), nil
}

func stripJSONCComments(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				if i < len(data) {
					out = append(out, data[i])
				}
				continue
			case '*':
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				if i+1 >= len(data) {
					return nil, fmt.Errorf("unterminated block comment")
				}
				i++
				continue
			}
		}

		out = append(out, c)
	}

	return out, nil
}

func stripJSONCTrailingCommas(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == ',' {
			next := i + 1
			for next < len(data) && strings.ContainsRune(" \t\r\n", rune(data[next])) {
				next++
			}
			if next < len(data) && (data[next] == ']' || data[next] == '}') {
				continue
			}
		}

		out = append(out, c)
	}

	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
