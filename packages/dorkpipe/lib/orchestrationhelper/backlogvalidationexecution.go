package orchestrationhelper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	backlogValidationExecutionContract = "dorkpipe.validation-execution/v1"
	backlogValidationCommandLimit      = 16
	backlogValidationCommandTimeout    = 4 * time.Minute
)

var (
	backlogValidationMkdirTemp  = os.MkdirTemp
	backlogValidationRemoveAll  = os.RemoveAll
	backlogValidationRunCommand = func(ctx context.Context, root string, argv, environment []string) (int, bool, error) {
		command := exec.CommandContext(ctx, argv[0], argv[1:]...)
		command.Dir = root
		command.Env = environment
		command.Stdin = nil
		var output bytes.Buffer
		command.Stdout = &output
		command.Stderr = &output
		err := command.Run()
		if err == nil {
			return 0, true, nil
		}
		if ctx.Err() != nil {
			return 0, command.ProcessState != nil, ctx.Err()
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			_, _ = os.Stderr.Write(output.Bytes())
			return exitError.ExitCode(), true, nil
		}
		return 0, false, err
	}
)

func executeBacklogValidation(consumerRoot, artifactRoot string) error {
	boundaryPath, err := requireBacklogRegularArtifact(artifactRoot, "patch-boundary.json")
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	applicationPath, err := requireBacklogRegularArtifact(artifactRoot, "patch-application.json")
	if err != nil {
		return rejectBacklog("validation_execution_application_invalid", "%v", err)
	}
	if err := verifyBacklogPatchBoundary(artifactRoot); err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	boundary, err := readStrictJSONMap(boundaryPath)
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	application, applicationFingerprint, err := loadAndVerifyBacklogPatchApplication(artifactRoot, applicationPath, request, boundary)
	if err != nil {
		return rejectBacklog("validation_execution_application_invalid", "%v", err)
	}
	requiredValidation, requiredValidationFingerprint, commands, err := backlogValidationCommands(request)
	if err != nil {
		return rejectBacklog("validation_execution_command_invalid", "%v", err)
	}

	executionPath, err := backlogArtifactPath(artifactRoot, "validation-execution.json")
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(executionPath); statErr == nil {
		if backlogFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
			return rejectBacklog("validation_execution_artifact_invalid", "existing validation-execution.json is not a regular non-link file")
		}
		existing, readErr := readStrictJSONMap(executionPath)
		if readErr != nil || validateBacklogValidationExecution(existing, request, application, applicationFingerprint, requiredValidation, requiredValidationFingerprint, commands) != nil {
			return rejectBacklog("validation_execution_artifact_invalid", "existing validation-execution.json is malformed, tampered, or does not match the immutable chain")
		}
		return nil
	} else if !os.IsNotExist(statErr) {
		return rejectBacklog("validation_execution_artifact_invalid", "%v", statErr)
	}

	consumer, err := validateBacklogConsumerRoot(consumerRoot)
	if err != nil {
		return rejectBacklog("validation_execution_source_invalid", "%v", err)
	}
	validationInputs, validationInputsFingerprint, err := backlogValidationInputs(request)
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	changedPaths, err := strictBacklogStringArray(application["changed_paths"])
	if err != nil || len(changedPaths) == 0 || !sort.StringsAreSorted(changedPaths) {
		return rejectBacklog("validation_execution_application_invalid", "patch application changed_paths are missing, malformed, or unsorted")
	}

	workspaceSources := make(map[string][]byte, len(validationInputs)+len(changedPaths))
	remainingBytes := backlogValidationInputMaxBytes
	for _, rawEntry := range validationInputs {
		entry := mapValue(rawEntry)
		path := stringValue(entry["path"])
		content, readErr := readBacklogValidationInput(consumer, path, remainingBytes)
		if readErr != nil {
			return rejectBacklog("validation_execution_source_invalid", "validation input %q cannot be read safely: %v", path, readErr)
		}
		expectedBytes, _ := backlogJSONInt(entry["bytes"])
		if len(content) != expectedBytes || sha256String(content) != stringValue(entry["sha256"]) {
			return rejectBacklog("validation_execution_source_invalid", "validation input %q does not match its immutable size and SHA-256", path)
		}
		workspaceSources[path] = content
		remainingBytes -= len(content)
	}
	for _, path := range changedPaths {
		if _, present := workspaceSources[path]; present {
			continue
		}
		content, readErr := readBacklogValidationInput(consumer, path, remainingBytes)
		if readErr != nil {
			return rejectBacklog("validation_execution_source_invalid", "changed-path overlay %q cannot be read safely: %v", path, readErr)
		}
		workspaceSources[path] = content
		remainingBytes -= len(content)
	}

	preimageFiles := make([]any, 0, len(changedPaths))
	for _, path := range changedPaths {
		preimageFiles = append(preimageFiles, backlogApplicationManifestEntry(path, workspaceSources[path]))
	}
	preimageManifest, err := backlogApplicationManifest(preimageFiles)
	if err != nil || !jsonMapsEqual(preimageManifest, mapValue(application["preimage_manifest"])) {
		return rejectBacklog("validation_execution_source_invalid", "changed-path overlay does not match the accepted patch-application preimage manifest")
	}

	patchRaw, err := os.ReadFile(filepath.Join(artifactRoot, "remote-diff.patch"))
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "accepted patch cannot be read: %v", err)
	}
	parsedPatch, err := parseBacklogApplicationPatch(patchRaw)
	if err != nil {
		return rejectBacklog("validation_execution_chain_invalid", "%v", err)
	}
	expectedWorkspace, expectedPostimage, err := expectedBacklogValidationWorkspace(workspaceSources, parsedPatch)
	if err != nil {
		return err
	}
	if !jsonMapsEqual(expectedPostimage, mapValue(application["postimage_manifest"])) {
		return rejectBacklog("validation_execution_application_invalid", "accepted patch-application postimage manifest does not match the immutable patch and source preimages")
	}

	temporaryRoot, err := backlogValidationMkdirTemp("", "dorkpipe-backlog-validation-execution-*")
	if err != nil {
		return rejectBacklog("validation_execution_workspace_failed", "temporary validation workspace cannot be created: %v", err)
	}
	commandEvidence := []any{}
	aggregateStatus := "passed"
	runErr := func() error {
		postimageFiles, _, applyErr := applyBacklogPatchInTemporaryCopy(temporaryRoot, parsedPatch, workspaceSources)
		if applyErr != nil {
			return rejectBacklog("validation_execution_workspace_failed", "%v", applyErr)
		}
		actualPostimage, manifestErr := backlogApplicationManifest(postimageFiles)
		if manifestErr != nil || !jsonMapsEqual(actualPostimage, expectedPostimage) {
			return rejectBacklog("validation_execution_workspace_failed", "isolated patch application did not reproduce the accepted postimage manifest")
		}
		if verifyErr := verifyBacklogValidationWorkspace(temporaryRoot, expectedWorkspace); verifyErr != nil {
			return rejectBacklog("validation_execution_workspace_failed", "%v", verifyErr)
		}
		environment := backlogValidationCommandEnvironment()
		for index, argv := range commands {
			ctx, cancel := context.WithTimeout(context.Background(), backlogValidationCommandTimeout)
			exitCode, launched, runCommandErr := backlogValidationRunCommand(ctx, temporaryRoot, argv, environment)
			cancel()
			if runCommandErr != nil || !launched {
				return rejectBacklog("validation_execution_launch_failed", "validation command %d could not complete: %v", index+1, runCommandErr)
			}
			status := "passed"
			if exitCode != 0 {
				status = "failed"
				aggregateStatus = "failed"
			}
			commandEvidence = append(commandEvidence, map[string]any{
				"ordinal": index + 1, "argv": anyStrings(argv), "status": status, "exit_code": exitCode,
			})
			if verifyErr := verifyBacklogValidationWorkspace(temporaryRoot, expectedWorkspace); verifyErr != nil {
				return rejectBacklog("validation_execution_workspace_failed", "validation command %d changed the bounded workspace: %v", index+1, verifyErr)
			}
			if exitCode != 0 {
				break
			}
		}
		return nil
	}()
	cleanupErr := backlogValidationRemoveAll(temporaryRoot)
	if cleanupErr == nil {
		if _, statErr := os.Lstat(temporaryRoot); !os.IsNotExist(statErr) {
			cleanupErr = errors.New("temporary validation workspace still exists after cleanup")
		}
	}
	if cleanupErr != nil {
		return rejectBacklog("validation_execution_cleanup_failed", "temporary validation workspace cleanup failed: %v", cleanupErr)
	}
	if runErr != nil {
		return runErr
	}

	payload, err := backlogValidationExecutionPayload(
		request, application, applicationFingerprint, validationInputsFingerprint,
		requiredValidation, requiredValidationFingerprint, changedPaths, len(workspaceSources), commandEvidence, aggregateStatus,
	)
	if err != nil {
		return err
	}
	return writeJSONFileAtomic(executionPath, payload)
}

func requireBacklogRegularArtifact(artifactRoot, name string) (string, error) {
	path, err := backlogArtifactPath(artifactRoot, name)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("%s is required: %w", name, err)
	}
	if backlogFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s must be a regular non-link file", name)
	}
	return path, nil
}

func loadAndVerifyBacklogPatchApplication(artifactRoot, applicationPath string, request, boundary map[string]any) (map[string]any, string, error) {
	application, err := readStrictJSONMap(applicationPath)
	if err != nil {
		return nil, "", err
	}
	if stringValue(application["contract_version"]) != backlogPatchApplicationContract || stringValue(application["state"]) != "completion_candidate" {
		return nil, "", errors.New("patch application has an unsupported contract or lifecycle state")
	}
	changedPaths, err := strictBacklogStringArray(boundary["changed_paths"])
	if err != nil || len(changedPaths) == 0 || !sort.StringsAreSorted(changedPaths) {
		return nil, "", errors.New("patch boundary changed_paths are missing, malformed, or unsorted")
	}
	preimageManifest, err := validateBacklogApplicationManifest(application["preimage_manifest"], changedPaths)
	if err != nil {
		return nil, "", fmt.Errorf("patch application preimage manifest is invalid: %w", err)
	}
	postimageManifest, err := validateBacklogApplicationManifest(application["postimage_manifest"], changedPaths)
	if err != nil {
		return nil, "", fmt.Errorf("patch application postimage manifest is invalid: %w", err)
	}
	boundaryFingerprint, err := backlogJSONFingerprint(boundary)
	if err != nil {
		return nil, "", err
	}
	patchRaw, err := os.ReadFile(filepath.Join(artifactRoot, "remote-diff.patch"))
	if err != nil {
		return nil, "", err
	}
	parsed, err := parseBacklogApplicationPatch(patchRaw)
	if err != nil {
		return nil, "", err
	}
	hunkCount := 0
	for _, file := range parsed.Files {
		hunkCount += len(file.Hunks)
	}
	expected := backlogPatchApplicationPayload(request, boundary, boundaryFingerprint, changedPaths, preimageManifest, postimageManifest, len(parsed.Files), hunkCount)
	if !jsonMapsEqual(application, expected) {
		return nil, "", errors.New("patch application is malformed, tampered, or does not match the immutable boundary")
	}
	fingerprint, err := backlogJSONFingerprint(application)
	return application, fingerprint, err
}

func validateBacklogApplicationManifest(value any, expectedPaths []string) (map[string]any, error) {
	manifest, ok := value.(map[string]any)
	if !ok || len(manifest) != 2 {
		return nil, errors.New("manifest must contain only files and fingerprint")
	}
	files, ok := manifest["files"].([]any)
	if !ok || len(files) != len(expectedPaths) {
		return nil, errors.New("manifest files do not match changed paths")
	}
	for index, raw := range files {
		entry, ok := raw.(map[string]any)
		bytesValue, bytesOK := backlogJSONInt(entry["bytes"])
		if !ok || len(entry) != 3 || stringValue(entry["path"]) != expectedPaths[index] || !backlogFingerprint.MatchString(stringValue(entry["sha256"])) || !bytesOK || bytesValue < 0 {
			return nil, errors.New("manifest contains a malformed or unsorted file entry")
		}
	}
	expected, err := backlogApplicationManifest(files)
	if err != nil || !jsonMapsEqual(manifest, expected) {
		return nil, errors.New("manifest fingerprint does not bind its exact file entries")
	}
	return manifest, nil
}

func backlogValidationCommands(request map[string]any) ([]string, string, [][]string, error) {
	declarations, err := strictBacklogStringArray(request["required_validation"])
	if err != nil || len(declarations) == 0 || len(declarations) > backlogValidationCommandLimit {
		return nil, "", nil, fmt.Errorf("required_validation must contain between 1 and %d command declarations", backlogValidationCommandLimit)
	}
	fingerprint, err := backlogRequiredValidationFingerprint(request)
	if err != nil {
		return nil, "", nil, err
	}
	commands := make([][]string, 0, len(declarations))
	for index, declaration := range declarations {
		argv, parseErr := parseBacklogValidationCommand(declaration)
		if parseErr != nil {
			return nil, "", nil, fmt.Errorf("required_validation command %d is unsupported: %w", index+1, parseErr)
		}
		commands = append(commands, argv)
	}
	return declarations, fingerprint, commands, nil
}

func parseBacklogValidationCommand(declaration string) ([]string, error) {
	if declaration == "" || len(declaration) > 1024 || strings.TrimSpace(declaration) != declaration {
		return nil, errors.New("command must be a non-empty bounded canonical line")
	}
	for _, value := range declaration {
		if value < 0x21 || value > 0x7e {
			if value == ' ' {
				continue
			}
			return nil, errors.New("command contains whitespace or non-ASCII syntax outside the supported grammar")
		}
	}
	if strings.ContainsAny(declaration, "\"'\\|&;<>`$(){}[]!?*~") {
		return nil, errors.New("command contains quoting, shell metacharacters, redirection, pipelines, or wildcard syntax")
	}
	argv := strings.Fields(declaration)
	if len(argv) < 3 || strings.Join(argv, " ") != declaration {
		return nil, errors.New("command spacing or argv shape is ambiguous")
	}
	if argv[0] != "go" || argv[1] != "test" {
		return nil, errors.New("only direct go test package validation is supported")
	}
	for _, argument := range argv[2:] {
		if strings.HasPrefix(argument, "-") || !strings.HasPrefix(argument, "./") || strings.Contains(argument, "...") {
			return nil, errors.New("go test arguments must be exact repository-relative package paths without flags or recursive patterns")
		}
		path := strings.TrimPrefix(argument, "./")
		if path == "" || strings.Contains(path, ":") || filepath.IsAbs(path) {
			return nil, errors.New("go test package path is absolute or empty")
		}
		for _, component := range strings.Split(path, "/") {
			if component == "" || component == "." || component == ".." {
				return nil, errors.New("go test package path contains an empty or traversal component")
			}
		}
	}
	return argv, nil
}

func expectedBacklogValidationWorkspace(sources map[string][]byte, patch backlogApplicationPatch) (map[string][]byte, map[string]any, error) {
	expected := make(map[string][]byte, len(sources))
	for path, raw := range sources {
		expected[path] = append([]byte(nil), raw...)
	}
	postimageFiles := make([]any, 0, len(patch.Files))
	for _, file := range patch.Files {
		postimage, err := applyBacklogApplicationFile(expected[file.Path], file)
		if err != nil {
			return nil, nil, err
		}
		expected[file.Path] = postimage
		postimageFiles = append(postimageFiles, backlogApplicationManifestEntry(file.Path, postimage))
	}
	sort.Slice(postimageFiles, func(i, j int) bool {
		return stringValue(mapValue(postimageFiles[i])["path"]) < stringValue(mapValue(postimageFiles[j])["path"])
	})
	manifest, err := backlogApplicationManifest(postimageFiles)
	return expected, manifest, err
}

func verifyBacklogValidationWorkspace(root string, expected map[string][]byte) error {
	actualPaths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if backlogFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("workspace contains a link or reparse point")
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("workspace contains a non-regular file")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		expectedRaw, ok := expected[rel]
		if !ok {
			return fmt.Errorf("workspace contains undeclared file %q", rel)
		}
		actualRaw, err := os.ReadFile(path)
		if err != nil || !bytesEqual(actualRaw, expectedRaw) {
			return fmt.Errorf("workspace file %q does not match its exact expected bytes", rel)
		}
		actualPaths = append(actualPaths, rel)
		return nil
	})
	if err != nil {
		return err
	}
	if len(actualPaths) != len(expected) {
		return fmt.Errorf("workspace contains %d files; expected exact union of %d files", len(actualPaths), len(expected))
	}
	return nil
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func backlogValidationCommandEnvironment() []string {
	overrides := map[string]string{
		"CGO_ENABLED": "0", "GO111MODULE": "on", "GOENV": "off", "GOFLAGS": "-mod=readonly -count=1",
		"GONOSUMDB": "*", "GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOVCS": "off",
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		key := entry
		if index := strings.IndexByte(entry, '='); index >= 0 {
			key = entry[:index]
		}
		upperKey := strings.ToUpper(key)
		if _, replaced := overrides[upperKey]; !replaced && upperKey != "GOTMPDIR" && upperKey != "GOWORK" {
			environment = append(environment, entry)
		}
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
}

func backlogValidationExecutionPayload(request, application map[string]any, applicationFingerprint, validationInputsFingerprint string, requiredValidation []string, requiredValidationFingerprint string, changedPaths []string, workspaceFileCount int, commandEvidence []any, aggregateStatus string) (map[string]any, error) {
	selection := mapValue(request["selection"])
	binding := mapValue(application["binding"])
	payload := map[string]any{
		"contract_version": backlogValidationExecutionContract,
		"state":            "completion_candidate",
		"patch_application": map[string]any{
			"fingerprint": applicationFingerprint,
		},
		"patch_boundary":       mapValue(application["patch_boundary"]),
		"validation_receipt":   mapValue(application["validation_receipt"]),
		"remote_result":        mapValue(application["remote_result"]),
		"remote_diff":          mapValue(application["remote_diff"]),
		"remote_status":        mapValue(application["remote_status"]),
		"completion_candidate": mapValue(application["completion_candidate"]),
		"binding": map[string]any{
			"request_fingerprint": stringValue(binding["request_fingerprint"]), "compatibility_fingerprint": stringValue(binding["compatibility_fingerprint"]),
			"validation_inputs_fingerprint": validationInputsFingerprint, "dispatch_fingerprint": stringValue(binding["dispatch_fingerprint"]),
			"task_id": stringValue(selection["task_id"]), "remote_task_id": stringValue(binding["remote_task_id"]),
			"adapter_identity": stringValue(binding["adapter_identity"]), "environment_ref": stringValue(binding["environment_ref"]),
			"branch_ref": stringValue(binding["branch_ref"]), "baseline_commit": stringValue(selection["baseline_commit"]),
			"baseline_commit_git_verified": false,
		},
		"validation_input_manifest": mapValue(request["validation_input_manifest"]),
		"required_validation": map[string]any{
			"fingerprint": requiredValidationFingerprint, "declaration": anyStrings(requiredValidation),
		},
		"accepted_patch": mapValue(application["accepted_patch"]),
		"changed_paths":  anyStrings(changedPaths),
		"workspace": map[string]any{
			"authority": "exact_validation_inputs_plus_changed_path_overlay", "validation_input_files": len(listValue(request["validation_input_files"])),
			"changed_path_overlay": anyStrings(changedPaths), "file_count": workspaceFileCount,
			"temporary_workspace_cleanup_succeeded": true, "consumer_checkout_mutated": false,
		},
		"commands": commandEvidence,
		"aggregate": map[string]any{
			"status": aggregateStatus, "commands_declared": len(requiredValidation), "commands_executed": len(commandEvidence),
			"stopped_after_first_failure": aggregateStatus == "failed",
		},
		"actions": map[string]any{
			"semantic_correctness_reviewed": false, "validation_executed": true, "validation_success_authoritative": false,
			"ready_for_review_emitted": false, "apply_to_checkout_performed": false, "commit_performed": false,
			"push_performed": false, "publication_performed": false,
		},
		"lifecycle": map[string]any{
			"ready_for_review": false, "apply": false, "commit": false, "push": false, "publication": false,
		},
	}
	fingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return nil, err
	}
	payload["artifact_fingerprint"] = fingerprint
	return payload, nil
}

func validateBacklogValidationExecution(payload, request, application map[string]any, applicationFingerprint string, requiredValidation []string, requiredValidationFingerprint string, commands [][]string) error {
	if stringValue(payload["contract_version"]) != backlogValidationExecutionContract || stringValue(payload["state"]) != "completion_candidate" {
		return errors.New("unsupported validation execution contract or lifecycle state")
	}
	changedPaths, err := strictBacklogStringArray(application["changed_paths"])
	if err != nil {
		return err
	}
	evidence, ok := payload["commands"].([]any)
	if !ok || len(evidence) == 0 || len(evidence) > len(commands) {
		return errors.New("validation command evidence is incomplete or malformed")
	}
	aggregateStatus := "passed"
	for index, raw := range evidence {
		entry, ok := raw.(map[string]any)
		exitCode, exitOK := backlogJSONInt(entry["exit_code"])
		argv, argvErr := strictBacklogStringArray(entry["argv"])
		status := stringValue(entry["status"])
		if !ok || len(entry) != 4 || intFromAny(entry["ordinal"]) != index+1 || argvErr != nil || !stringSlicesEqual(argv, commands[index]) || !exitOK || exitCode < 0 {
			return errors.New("validation command evidence contains a malformed entry")
		}
		if index < len(evidence)-1 && (status != "passed" || exitCode != 0) {
			return errors.New("validation command evidence did not stop on the first failure")
		}
		if status == "failed" {
			if exitCode == 0 || index != len(evidence)-1 {
				return errors.New("failed validation command evidence has an invalid exit code or ordering")
			}
			aggregateStatus = "failed"
		} else if status != "passed" || exitCode != 0 {
			return errors.New("passed validation command evidence has an invalid status or exit code")
		}
	}
	if aggregateStatus == "passed" && len(evidence) != len(commands) {
		return errors.New("passed validation evidence did not execute every declared command")
	}
	validationInputs, validationInputsFingerprint, err := backlogValidationInputs(request)
	if err != nil {
		return err
	}
	union := map[string]bool{}
	for _, raw := range validationInputs {
		union[stringValue(mapValue(raw)["path"])] = true
	}
	for _, path := range changedPaths {
		union[path] = true
	}
	expected, err := backlogValidationExecutionPayload(request, application, applicationFingerprint, validationInputsFingerprint, requiredValidation, requiredValidationFingerprint, changedPaths, len(union), evidence, aggregateStatus)
	if err != nil || !jsonMapsEqual(payload, expected) {
		return errors.New("validation execution artifact does not match its canonical immutable payload")
	}
	return nil
}
