package orchestrationhelper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const backlogPatchApplicationContract = "dorkpipe.patch-application/v2"

var (
	backlogApplicationHunk      = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?$`)
	backlogApplicationMkdirTemp = os.MkdirTemp
	backlogApplicationRemoveAll = os.RemoveAll
)

type backlogApplicationPatch struct {
	Files []backlogApplicationFile
}

type backlogApplicationFile struct {
	Path  string
	Hunks []backlogApplicationHunkData
}

type backlogApplicationHunkData struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []backlogApplicationLine
}

type backlogApplicationLine struct {
	Kind byte
	Text string
}

func applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot string) error {
	boundaryPath, err := backlogArtifactPath(artifactRoot, "patch-boundary.json")
	if err != nil {
		return rejectBacklog("patch_application_boundary_invalid", "%v", err)
	}
	if info, statErr := os.Lstat(boundaryPath); statErr != nil {
		return rejectBacklog("patch_application_boundary_invalid", "patch-boundary.json is required: %v", statErr)
	} else if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return rejectBacklog("patch_application_boundary_invalid", "patch-boundary.json must be a regular non-symlink file")
	}
	if err := verifyBacklogPatchBoundary(artifactRoot); err != nil {
		return rejectBacklog("patch_application_boundary_invalid", "%v", err)
	}
	boundary, err := readStrictJSONMap(boundaryPath)
	if err != nil {
		return rejectBacklog("patch_application_boundary_invalid", "%v", err)
	}
	boundaryFingerprint, err := backlogJSONFingerprint(boundary)
	if err != nil {
		return rejectBacklog("patch_application_boundary_invalid", "%v", err)
	}
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return rejectBacklog("patch_application_request_invalid", "%v", err)
	}

	applicationPath, err := backlogArtifactPath(artifactRoot, "patch-application.json")
	if err != nil {
		return err
	}
	var existing map[string]any
	if info, statErr := os.Lstat(applicationPath); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return rejectBacklog("patch_application_artifact_invalid", "existing patch-application.json is not a regular non-symlink file")
		}
		existing, err = readStrictJSONMap(applicationPath)
		if err != nil || stringValue(existing["contract_version"]) != backlogPatchApplicationContract || stringValue(existing["state"]) != "completion_candidate" {
			return rejectBacklog("patch_application_artifact_invalid", "existing patch-application.json is malformed or tampered")
		}
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("patch_application_artifact_invalid", "%v", statErr)
	}

	changedPaths, err := strictBacklogStringArray(boundary["changed_paths"])
	if err != nil || len(changedPaths) == 0 || !sort.StringsAreSorted(changedPaths) {
		return rejectBacklog("patch_application_boundary_invalid", "verified changed_paths are missing, malformed, or unsorted")
	}
	patchPath, err := backlogArtifactPath(artifactRoot, "remote-diff.patch")
	if err != nil {
		return err
	}
	patchRaw, err := os.ReadFile(patchPath)
	if err != nil {
		return rejectBacklog("patch_application_boundary_invalid", "accepted patch cannot be read: %v", err)
	}
	parsed, err := parseBacklogApplicationPatch(patchRaw)
	if err != nil {
		return rejectBacklog("patch_application_patch_unsupported", "%v", err)
	}
	parsedPaths := make([]string, 0, len(parsed.Files))
	for _, file := range parsed.Files {
		parsedPaths = append(parsedPaths, file.Path)
	}
	sort.Strings(parsedPaths)
	if !stringSlicesEqual(parsedPaths, changedPaths) {
		return rejectBacklog("patch_application_boundary_invalid", "application section paths differ from verified changed_paths")
	}

	root, err := validateBacklogConsumerRoot(consumerRoot)
	if err != nil {
		return rejectBacklog("patch_application_source_invalid", "%v", err)
	}
	preimages := make(map[string][]byte, len(changedPaths))
	preimageFiles := make([]any, 0, len(changedPaths))
	for _, changedPath := range changedPaths {
		raw, readErr := readBacklogConsumerSource(root, changedPath)
		if readErr != nil {
			return rejectBacklog("patch_application_source_invalid", "%v", readErr)
		}
		preimages[changedPath] = raw
		preimageFiles = append(preimageFiles, backlogApplicationManifestEntry(changedPath, raw))
	}
	preimageManifest, err := backlogApplicationManifest(preimageFiles)
	if err != nil {
		return err
	}

	temporaryRoot, err := backlogApplicationMkdirTemp("", "dorkpipe-backlog-patch-application-*")
	if err != nil {
		return rejectBacklog("patch_application_temporary_copy_failed", "temporary workspace cannot be created: %v", err)
	}
	postimageFiles, hunkCount, applicationErr := applyBacklogPatchInTemporaryCopy(temporaryRoot, parsed, preimages)
	cleanupErr := backlogApplicationRemoveAll(temporaryRoot)
	if cleanupErr == nil {
		if _, statErr := os.Lstat(temporaryRoot); !os.IsNotExist(statErr) {
			cleanupErr = fmt.Errorf("temporary workspace still exists after cleanup")
		}
	}
	if cleanupErr != nil {
		return rejectBacklog("patch_application_cleanup_failed", "temporary workspace cleanup failed: %v", cleanupErr)
	}
	if applicationErr != nil {
		return applicationErr
	}
	postimageManifest, err := backlogApplicationManifest(postimageFiles)
	if err != nil {
		return err
	}
	payload := backlogPatchApplicationPayload(request, boundary, boundaryFingerprint, changedPaths, preimageManifest, postimageManifest, len(parsed.Files), hunkCount)
	if existing != nil {
		if !jsonMapsEqual(existing, payload) {
			return rejectBacklog("patch_application_artifact_invalid", "existing patch-application.json is tampered or does not match the immutable chain and source preimages")
		}
		return nil
	}
	return writeJSONFileAtomic(applicationPath, payload)
}

func validateBacklogConsumerRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("consumer root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", fmt.Errorf("consumer root cannot be inspected: %w", err)
	}
	if backlogFileInfoIsLinkOrReparse(info) || !info.IsDir() {
		return "", errors.New("consumer root must be a directory and must not be a symlink")
	}
	return abs, nil
}

func readBacklogConsumerSource(root, rel string) ([]byte, error) {
	if err := validateBacklogPatchPath(rel); err != nil {
		return nil, fmt.Errorf("changed source path %q is invalid: %w", rel, err)
	}
	candidate := filepath.Join(root, filepath.FromSlash(rel))
	if !withinRoot(root, candidate) {
		return nil, fmt.Errorf("changed source path %q escapes the consumer root", rel)
	}
	current := root
	parts := strings.Split(rel, "/")
	var finalInfo os.FileInfo
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("changed source path %q cannot be inspected: %w", rel, err)
		}
		if backlogFileInfoIsLinkOrReparse(info) {
			return nil, fmt.Errorf("changed source path %q contains a filesystem link or reparse point", rel)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("changed source path %q has a non-directory ancestor", rel)
		}
		if index == len(parts)-1 {
			finalInfo = info
		}
	}
	if finalInfo == nil || !finalInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("changed source path %q is not a regular file", rel)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil || !withinRoot(resolvedRoot, resolvedCandidate) {
		return nil, fmt.Errorf("changed source path %q escapes the consumer root through a filesystem link", rel)
	}
	file, err := os.Open(candidate)
	if err != nil {
		return nil, fmt.Errorf("changed source path %q cannot be opened: %w", rel, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(finalInfo, openedInfo) {
		return nil, fmt.Errorf("changed source path %q changed while being opened", rel)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("changed source path %q cannot be read: %w", rel, err)
	}
	return raw, nil
}

func parseBacklogApplicationPatch(raw []byte) (backlogApplicationPatch, error) {
	if _, err := parseBacklogPatchChangedPaths(raw); err != nil {
		return backlogApplicationPatch{}, err
	}
	lines := strings.Split(string(raw[:len(raw)-1]), "\n")
	parsed := backlogApplicationPatch{}
	for index := 0; index < len(lines); {
		parts := strings.Split(lines[index], " ")
		file := backlogApplicationFile{Path: strings.TrimPrefix(parts[2], "a/")}
		index += 4
		for index < len(lines) && strings.HasPrefix(lines[index], "@@ ") {
			matches := backlogApplicationHunk.FindStringSubmatch(lines[index])
			if matches == nil {
				return backlogApplicationPatch{}, fmt.Errorf("line %d has an unsupported application hunk header", index+1)
			}
			hunk, err := parseBacklogApplicationHunkCoordinates(matches)
			if err != nil {
				return backlogApplicationPatch{}, fmt.Errorf("line %d has invalid hunk coordinates: %w", index+1, err)
			}
			index++
			oldLines, newLines := 0, 0
			for index < len(lines) && !strings.HasPrefix(lines[index], "@@ ") && !strings.HasPrefix(lines[index], "diff --git ") {
				line := lines[index]
				if line == "\\ No newline at end of file" {
					return backlogApplicationPatch{}, fmt.Errorf("line %d uses an unsupported no-newline marker", index+1)
				}
				entry := backlogApplicationLine{Kind: line[0], Text: line[1:]}
				switch entry.Kind {
				case ' ':
					oldLines++
					newLines++
				case '-':
					oldLines++
				case '+':
					newLines++
				default:
					return backlogApplicationPatch{}, fmt.Errorf("line %d has an unsupported hunk line", index+1)
				}
				hunk.Lines = append(hunk.Lines, entry)
				index++
			}
			if oldLines != hunk.OldCount || newLines != hunk.NewCount {
				return backlogApplicationPatch{}, fmt.Errorf("hunk for %q declares old/new counts %d/%d but contains %d/%d lines", file.Path, hunk.OldCount, hunk.NewCount, oldLines, newLines)
			}
			file.Hunks = append(file.Hunks, hunk)
		}
		parsed.Files = append(parsed.Files, file)
	}
	return parsed, nil
}

func parseBacklogApplicationHunkCoordinates(matches []string) (backlogApplicationHunkData, error) {
	parse := func(value string, defaultValue int) (int, error) {
		if value == "" {
			return defaultValue, nil
		}
		parsed, err := strconv.ParseUint(value, 10, 31)
		return int(parsed), err
	}
	oldStart, err := parse(matches[1], 0)
	if err != nil {
		return backlogApplicationHunkData{}, err
	}
	oldCount, err := parse(matches[2], 1)
	if err != nil {
		return backlogApplicationHunkData{}, err
	}
	newStart, err := parse(matches[3], 0)
	if err != nil {
		return backlogApplicationHunkData{}, err
	}
	newCount, err := parse(matches[4], 1)
	if err != nil {
		return backlogApplicationHunkData{}, err
	}
	if (oldCount > 0 && oldStart == 0) || (newCount > 0 && newStart == 0) {
		return backlogApplicationHunkData{}, errors.New("non-empty hunk ranges must use one-based coordinates")
	}
	return backlogApplicationHunkData{OldStart: oldStart, OldCount: oldCount, NewStart: newStart, NewCount: newCount}, nil
}

func applyBacklogPatchInTemporaryCopy(root string, patch backlogApplicationPatch, preimages map[string][]byte) ([]any, int, error) {
	for path, raw := range preimages {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, 0, rejectBacklog("patch_application_temporary_copy_failed", "%v", err)
		}
		if err := os.WriteFile(target, raw, 0o600); err != nil {
			return nil, 0, rejectBacklog("patch_application_temporary_copy_failed", "%v", err)
		}
	}
	hunkCount := 0
	postimages := map[string][]byte{}
	for _, file := range patch.Files {
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		raw, err := os.ReadFile(target)
		if err != nil {
			return nil, 0, rejectBacklog("patch_application_temporary_copy_failed", "%v", err)
		}
		postimage, err := applyBacklogApplicationFile(raw, file)
		if err != nil {
			return nil, 0, err
		}
		if err := os.WriteFile(target, postimage, 0o600); err != nil {
			return nil, 0, rejectBacklog("patch_application_temporary_copy_failed", "%v", err)
		}
		postimages[file.Path] = postimage
		hunkCount += len(file.Hunks)
	}
	paths := make([]string, 0, len(postimages))
	for path := range postimages {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	manifest := make([]any, 0, len(paths))
	for _, path := range paths {
		manifest = append(manifest, backlogApplicationManifestEntry(path, postimages[path]))
	}
	return manifest, hunkCount, nil
}

func applyBacklogApplicationFile(raw []byte, file backlogApplicationFile) ([]byte, error) {
	lines, err := backlogApplicationTextLines(raw)
	if err != nil {
		return nil, rejectBacklog("patch_application_source_invalid", "source %q is unsupported application text: %v", file.Path, err)
	}
	output := make([]string, 0, len(lines))
	cursor := 0
	for hunkIndex, hunk := range file.Hunks {
		oldIndex := hunk.OldStart
		if hunk.OldCount > 0 {
			oldIndex--
		}
		newIndex := hunk.NewStart
		if hunk.NewCount > 0 {
			newIndex--
		}
		if oldIndex < cursor || oldIndex > len(lines) || newIndex != len(output)+(oldIndex-cursor) {
			return nil, rejectBacklog("patch_application_patch_unsupported", "hunk %d for %q has invalid, overlapping, or inconsistent coordinates", hunkIndex+1, file.Path)
		}
		output = append(output, lines[cursor:oldIndex]...)
		cursor = oldIndex
		for _, line := range hunk.Lines {
			switch line.Kind {
			case ' ', '-':
				if cursor >= len(lines) || lines[cursor] != line.Text {
					kind := "context"
					if line.Kind == '-' {
						kind = "removed line"
					}
					return nil, rejectBacklog("patch_application_preimage_mismatch", "hunk %d %s does not exactly match source %q at line %d", hunkIndex+1, kind, file.Path, cursor+1)
				}
				if line.Kind == ' ' {
					output = append(output, line.Text)
				}
				cursor++
			case '+':
				output = append(output, line.Text)
			}
		}
	}
	output = append(output, lines[cursor:]...)
	if len(output) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(output, "\n") + "\n"), nil
}

func backlogApplicationTextLines(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 || bytes.Contains(raw, []byte{'\r'}) || raw[len(raw)-1] != '\n' {
		return nil, errors.New("source must be LF-terminated UTF-8 text without NUL or CR bytes")
	}
	return strings.Split(string(raw[:len(raw)-1]), "\n"), nil
}

func backlogApplicationManifestEntry(path string, raw []byte) map[string]any {
	return map[string]any{"path": path, "sha256": sha256String(raw), "bytes": len(raw)}
}

func backlogApplicationManifest(files []any) (map[string]any, error) {
	fingerprint, err := backlogJSONFingerprint(map[string]any{"files": files})
	if err != nil {
		return nil, err
	}
	return map[string]any{"files": files, "fingerprint": fingerprint}, nil
}

func backlogPatchApplicationPayload(request, boundary map[string]any, boundaryFingerprint string, changedPaths []string, preimageManifest, postimageManifest map[string]any, fileCount, hunkCount int) map[string]any {
	selection := mapValue(request["selection"])
	binding := mapValue(boundary["binding"])
	remoteDiff := mapValue(boundary["remote_diff"])
	return map[string]any{
		"contract_version":     backlogPatchApplicationContract,
		"state":                "completion_candidate",
		"patch_boundary":       map[string]any{"fingerprint": boundaryFingerprint},
		"validation_receipt":   mapValue(boundary["validation_receipt"]),
		"remote_result":        mapValue(boundary["remote_result"]),
		"remote_diff":          remoteDiff,
		"remote_status":        mapValue(boundary["remote_status"]),
		"completion_candidate": mapValue(boundary["completion_candidate"]),
		"binding": map[string]any{
			"request_fingerprint": stringValue(binding["request_fingerprint"]), "compatibility_fingerprint": stringValue(binding["compatibility_fingerprint"]),
			"validation_inputs_fingerprint": stringValue(binding["validation_inputs_fingerprint"]),
			"dispatch_fingerprint":          stringValue(binding["dispatch_fingerprint"]), "remote_task_id": stringValue(binding["remote_task_id"]),
			"adapter_identity": stringValue(binding["adapter_identity"]), "environment_ref": stringValue(binding["environment_ref"]),
			"branch_ref": stringValue(binding["branch_ref"]), "baseline_commit": stringValue(selection["baseline_commit"]),
			"baseline_commit_git_verified": false,
		},
		"accepted_patch":     map[string]any{"sha256": stringValue(remoteDiff["patch_sha256"]), "bytes": remoteDiff["patch_bytes"]},
		"changed_paths":      anyStrings(changedPaths),
		"preimage_manifest":  preimageManifest,
		"postimage_manifest": postimageManifest,
		"application": map[string]any{
			"application_scope": "temporary_copy_only", "mechanical_application_succeeded": true,
			"temporary_workspace_cleanup_succeeded": true, "files_applied": fileCount, "hunks_applied": hunkCount,
		},
		"actions": map[string]any{
			"semantic_correctness_reviewed": false, "validation_executed": false, "consumer_checkout_mutated": false,
			"ready_for_review_emitted": false, "apply_to_checkout_performed": false, "commit_performed": false,
			"push_performed": false, "publication_performed": false,
		},
		"lifecycle": map[string]any{
			"ready_for_review": false, "validation_execution": false, "apply": false,
			"commit": false, "push": false, "publication": false,
		},
	}
}

func strictBacklogStringArray(value any) ([]string, error) {
	raw, ok := value.([]any)
	if !ok {
		return nil, errors.New("value is not an array")
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, errors.New("array contains a non-string value")
		}
		values = append(values, text)
	}
	return values, nil
}
