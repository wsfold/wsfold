package wsfold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tailscale/hujson"
)

const (
	workspaceTemplate         = "{\n  \"folders\": [],\n  \"settings\": {}\n}\n"
	topLevelMemberIndent      = "\n  "
	topLevelClosingIndent     = "\n"
	arrayElementIndent        = "\n    "
	arrayClosingIndent        = "\n  "
	nestedObjectMemberIndent  = "\n      "
	nestedObjectClosingIndent = "\n    "
	settingsMemberIndent      = "\n    "
	settingsClosingIndent     = "\n  "
	excludeMemberIndent       = "\n      "
	excludeClosingIndent      = "\n    "
	valueLeadingSpace         = " "
)

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
	value, _, err := loadWorkspaceValue(workspacePath(primaryRoot))
	if err != nil {
		return nil, err
	}
	if err := mergeWorkspaceValue(&value, previous, manifest, projectsDirName); err != nil {
		return nil, err
	}
	return value.Pack(), nil
}

func loadWorkspaceValue(path string) (hujson.Value, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			value, parseErr := hujson.Parse([]byte(workspaceTemplate))
			if parseErr != nil {
				return hujson.Value{}, true, fmt.Errorf("parse workspace template: %w", parseErr)
			}
			return value, true, nil
		}
		return hujson.Value{}, false, fmt.Errorf("read workspace: %w", err)
	}

	value, err := hujson.Parse(data)
	if err != nil {
		return hujson.Value{}, false, fmt.Errorf("parse workspace as JSONC: %w", err)
	}
	return value, false, nil
}

func mergeWorkspaceValue(value *hujson.Value, previous Manifest, manifest Manifest, projectsDirName string) error {
	root, ok := value.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf("workspace root must be a JSON object")
	}

	currentFolders, err := workspaceFolders(manifest)
	if err != nil {
		return err
	}
	previousFolders, err := workspaceFolders(previous)
	if err != nil {
		return err
	}

	foldersArray := ensureTopLevelArrayMember(root, "folders")
	mergeFoldersArray(foldersArray, previousFolders, currentFolders)

	settingsObject := ensureTopLevelObjectMember(root, "settings")
	previousExcludes := workspaceExcludes(previous, projectsDirName)
	currentExcludes := workspaceExcludes(manifest, projectsDirName)
	for _, key := range []string{"files.exclude", "files.watcherExclude", "search.exclude"} {
		excludeObject := ensureNestedObjectMember(settingsObject, key, settingsMemberIndent, excludeClosingIndent)
		mergeExcludeObject(excludeObject, previousExcludes, currentExcludes)
	}

	return nil
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

func mergeFoldersArray(arr *hujson.Array, previous []workspaceFolder, current []workspaceFolder) {
	currentByPath := make(map[string]workspaceFolder, len(current))
	for _, folder := range current {
		currentByPath[folder.Path] = folder
	}

	previousPaths := make(map[string]struct{}, len(previous))
	for _, folder := range previous {
		previousPaths[folder.Path] = struct{}{}
	}

	hadTrailingComma := arrayHasTrailingComma(arr)
	used := map[string]struct{}{}
	result := make([]hujson.Value, 0, len(arr.Elements)+len(current))
	for _, element := range arr.Elements {
		path, isFolderObject := folderElementPath(element)
		_, wasManaged := previousPaths[path]
		currentFolder, isManaged := currentByPath[path]
		if path == "." || wasManaged || isManaged {
			if !isManaged {
				continue
			}
			if _, seen := used[path]; seen {
				continue
			}
			if isFolderObject {
				updateFolderElement(&element, currentFolder)
			} else {
				replacement := newFolderElement(currentFolder)
				replacement.BeforeExtra = copyExtra(element.BeforeExtra)
				replacement.AfterExtra = copyExtra(element.AfterExtra)
				element = replacement
			}
			result = append(result, element)
			used[path] = struct{}{}
			continue
		}

		result = append(result, element)
	}

	for _, folder := range current {
		if _, ok := used[folder.Path]; ok {
			continue
		}
		result = append(result, newFolderElement(folder))
	}

	arr.Elements = result
	setArrayTrailingComma(arr, hadTrailingComma)
}

func mergeExcludeObject(obj *hujson.Object, previous map[string]bool, current map[string]bool) {
	hadTrailingComma := objectHasTrailingComma(obj)
	used := map[string]struct{}{}
	result := make([]hujson.ObjectMember, 0, len(obj.Members)+len(current))

	for _, member := range obj.Members {
		name, ok := memberName(member)
		if !ok {
			result = append(result, member)
			continue
		}

		_, wasManaged := previous[name]
		_, isManaged := current[name]
		if wasManaged || isManaged {
			if !isManaged {
				continue
			}
			if _, seen := used[name]; seen {
				continue
			}
			member.Value.Value = hujson.Bool(true)
			result = append(result, member)
			used[name] = struct{}{}
			continue
		}

		result = append(result, member)
	}

	keys := sortedTrueKeys(current)
	for _, key := range keys {
		if _, ok := used[key]; ok {
			continue
		}
		result = append(result, newBoolMember(key, true, excludeMemberIndent))
	}

	obj.Members = result
	setObjectTrailingComma(obj, hadTrailingComma)
}

func ensureTopLevelArrayMember(root *hujson.Object, name string) *hujson.Array {
	member := ensureObjectMember(root, name, topLevelMemberIndent, newArray(arrayClosingIndent))
	array, ok := member.Value.Value.(*hujson.Array)
	if ok {
		if len(array.Elements) == 0 && array.AfterExtra == nil {
			array.AfterExtra = hujson.Extra(arrayClosingIndent)
		}
		return array
	}

	before := copyExtra(member.Value.BeforeExtra)
	after := copyExtra(member.Value.AfterExtra)
	member.Value = hujson.Value{
		BeforeExtra: preserveValueBeforeExtra(before),
		Value:       newArray(arrayClosingIndent),
		AfterExtra:  after,
	}
	return member.Value.Value.(*hujson.Array)
}

func ensureTopLevelObjectMember(root *hujson.Object, name string) *hujson.Object {
	member := ensureObjectMember(root, name, topLevelMemberIndent, newObject(settingsClosingIndent))
	object, ok := member.Value.Value.(*hujson.Object)
	if ok {
		if len(object.Members) == 0 && object.AfterExtra == nil {
			object.AfterExtra = hujson.Extra(settingsClosingIndent)
		}
		return object
	}

	before := copyExtra(member.Value.BeforeExtra)
	after := copyExtra(member.Value.AfterExtra)
	member.Value = hujson.Value{
		BeforeExtra: preserveValueBeforeExtra(before),
		Value:       newObject(settingsClosingIndent),
		AfterExtra:  after,
	}
	return member.Value.Value.(*hujson.Object)
}

func ensureNestedObjectMember(obj *hujson.Object, name string, memberIndent string, closingIndent string) *hujson.Object {
	member := ensureObjectMember(obj, name, memberIndent, newObject(closingIndent))
	nested, ok := member.Value.Value.(*hujson.Object)
	if ok {
		if len(nested.Members) == 0 && nested.AfterExtra == nil {
			nested.AfterExtra = hujson.Extra(closingIndent)
		}
		return nested
	}

	before := copyExtra(member.Value.BeforeExtra)
	after := copyExtra(member.Value.AfterExtra)
	member.Value = hujson.Value{
		BeforeExtra: preserveValueBeforeExtra(before),
		Value:       newObject(closingIndent),
		AfterExtra:  after,
	}
	return member.Value.Value.(*hujson.Object)
}

func ensureObjectMember(obj *hujson.Object, name string, memberIndent string, value hujson.ValueTrimmed) *hujson.ObjectMember {
	if _, member := findObjectMember(obj, name); member != nil {
		return member
	}

	member := hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: hujson.Extra(memberIndent),
			Value:       hujson.String(name),
		},
		Value: hujson.Value{
			BeforeExtra: hujson.Extra(valueLeadingSpace),
			Value:       value,
		},
	}
	obj.Members = append(obj.Members, member)
	return &obj.Members[len(obj.Members)-1]
}

func updateFolderElement(element *hujson.Value, folder workspaceFolder) {
	obj, ok := element.Value.(*hujson.Object)
	if !ok {
		return
	}
	ensureStringMember(obj, "name", folder.Name, nestedObjectMemberIndent)
	ensureStringMember(obj, "path", folder.Path, nestedObjectMemberIndent)
}

func ensureStringMember(obj *hujson.Object, name string, value string, memberIndent string) {
	if _, member := findObjectMember(obj, name); member != nil {
		member.Value.Value = hujson.String(value)
		return
	}

	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: hujson.Extra(memberIndent),
			Value:       hujson.String(name),
		},
		Value: hujson.Value{
			BeforeExtra: hujson.Extra(valueLeadingSpace),
			Value:       hujson.String(value),
		},
	})
}

func newFolderElement(folder workspaceFolder) hujson.Value {
	return hujson.Value{
		BeforeExtra: hujson.Extra(arrayElementIndent),
		Value: &hujson.Object{
			Members: []hujson.ObjectMember{
				newStringMember("name", folder.Name, nestedObjectMemberIndent),
				newStringMember("path", folder.Path, nestedObjectMemberIndent),
			},
			AfterExtra: hujson.Extra(nestedObjectClosingIndent),
		},
	}
}

func newStringMember(name string, value string, memberIndent string) hujson.ObjectMember {
	return hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: hujson.Extra(memberIndent),
			Value:       hujson.String(name),
		},
		Value: hujson.Value{
			BeforeExtra: hujson.Extra(valueLeadingSpace),
			Value:       hujson.String(value),
		},
	}
}

func newBoolMember(name string, value bool, memberIndent string) hujson.ObjectMember {
	return hujson.ObjectMember{
		Name: hujson.Value{
			BeforeExtra: hujson.Extra(memberIndent),
			Value:       hujson.String(name),
		},
		Value: hujson.Value{
			BeforeExtra: hujson.Extra(valueLeadingSpace),
			Value:       hujson.Bool(value),
		},
	}
}

func newObject(closingIndent string) *hujson.Object {
	return &hujson.Object{AfterExtra: hujson.Extra(closingIndent)}
}

func newArray(closingIndent string) *hujson.Array {
	return &hujson.Array{AfterExtra: hujson.Extra(closingIndent)}
}

func folderElementPath(element hujson.Value) (string, bool) {
	obj, ok := element.Value.(*hujson.Object)
	if !ok {
		return "", false
	}
	path, ok := objectStringValue(obj, "path")
	return path, ok
}

func objectStringValue(obj *hujson.Object, name string) (string, bool) {
	_, member := findObjectMember(obj, name)
	if member == nil {
		return "", false
	}
	literal, ok := member.Value.Value.(hujson.Literal)
	if !ok || literal.Kind() != '"' {
		return "", false
	}
	return literal.String(), true
}

func findObjectMember(obj *hujson.Object, name string) (int, *hujson.ObjectMember) {
	for i := range obj.Members {
		memberName, ok := memberName(obj.Members[i])
		if ok && memberName == name {
			return i, &obj.Members[i]
		}
	}
	return -1, nil
}

func memberName(member hujson.ObjectMember) (string, bool) {
	literal, ok := member.Name.Value.(hujson.Literal)
	if !ok || literal.Kind() != '"' {
		return "", false
	}
	return literal.String(), true
}

func preserveValueBeforeExtra(extra hujson.Extra) hujson.Extra {
	if extra == nil {
		return hujson.Extra(valueLeadingSpace)
	}
	return extra
}

func copyExtra(extra hujson.Extra) hujson.Extra {
	if extra == nil {
		return nil
	}
	return append(hujson.Extra(nil), extra...)
}

func arrayHasTrailingComma(arr *hujson.Array) bool {
	if len(arr.Elements) == 0 {
		return false
	}
	return arr.Elements[len(arr.Elements)-1].AfterExtra != nil
}

func setArrayTrailingComma(arr *hujson.Array, enabled bool) {
	if len(arr.Elements) == 0 {
		return
	}
	last := &arr.Elements[len(arr.Elements)-1]
	if enabled {
		if last.AfterExtra == nil {
			last.AfterExtra = hujson.Extra{}
		}
		return
	}
	last.AfterExtra = nil
}

func objectHasTrailingComma(obj *hujson.Object) bool {
	if len(obj.Members) == 0 {
		return false
	}
	return obj.Members[len(obj.Members)-1].Value.AfterExtra != nil
}

func setObjectTrailingComma(obj *hujson.Object, enabled bool) {
	if len(obj.Members) == 0 {
		return
	}
	last := &obj.Members[len(obj.Members)-1].Value
	if enabled {
		if last.AfterExtra == nil {
			last.AfterExtra = hujson.Extra{}
		}
		return
	}
	last.AfterExtra = nil
}

func sortedTrueKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key, enabled := range values {
		if enabled {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func decodeWorkspaceJSON(data []byte) (map[string]any, error) {
	standard, err := hujson.Standardize(data)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(standard, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
