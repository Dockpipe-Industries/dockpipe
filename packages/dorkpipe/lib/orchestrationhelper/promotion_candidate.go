package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type softwareDevPromotionIdentity struct {
	TaskPackPath string
	StepID       string
	SiblingRoles string
}

func evaluateSoftwareDevPromotionArtifacts(repoRoot, taskPackPath, taskPackStepID, artifactRoot string) error {
	repoTaskPack, identity, err := loadSoftwareDevPromotionIdentity(repoRoot, taskPackPath, taskPackStepID)
	if err != nil {
		return err
	}
	rootPath, err := softwareDevArtifactRoot(artifactRoot)
	if err != nil {
		return err
	}
	proposalDir := filepath.Join(rootPath, "proposal")
	if err := requirePromotionDirectory(rootPath, proposalDir, "proposal artifact directory"); err != nil {
		return err
	}
	if err := requirePromotionDirectory(rootPath, filepath.Join(rootPath, "verify"), "verification artifact directory"); err != nil {
		return err
	}
	candidatePath := filepath.Join(proposalDir, "promotion-candidate.json")

	rawProposal, rawRelativePath, err := readPromotionRawProposal(rootPath)
	if err != nil {
		return err
	}
	parsed, err := parsePlannerProposal(rawProposal)
	if err != nil {
		return fmt.Errorf("promotion source proposal is invalid: %w", err)
	}
	normalized, err := readPromotionJSONMap(rootPath, "proposal/normalized.json")
	if err != nil {
		return err
	}
	metadata, err := readPromotionJSONMap(rootPath, "proposal/metadata.json")
	if err != nil {
		return err
	}
	verification, err := readPromotionJSONMap(rootPath, "verify/result.json")
	if err != nil {
		return err
	}
	if mustJSON(parsed.Declaration, nil) != mustJSON(normalized, nil) {
		return errors.New("promotion proposal raw and normalized artifacts are inconsistent")
	}
	if err := validatePromotionMetadata(parsed, metadata, identity); err != nil {
		return err
	}
	if err := validatePromotionVerificationArtifact(verification); err != nil {
		return err
	}

	candidate := evaluateSoftwareDevPromotion(repoTaskPack, normalized, metadata, verification, identity, rawRelativePath)
	if err := addPromotionCandidateTargetDigests(candidate, repoRoot, identity); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(candidatePath, candidate); err != nil {
		return fmt.Errorf("write promotion candidate: %w", err)
	}
	return nil
}

func loadSoftwareDevPromotionIdentity(repoRoot, taskPackPath, taskPackStepID string) (map[string]any, softwareDevPromotionIdentity, error) {
	displayPath := strings.TrimSpace(taskPackPath)
	canonicalPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(displayPath)))
	if displayPath == "" {
		return nil, softwareDevPromotionIdentity{}, errors.New("promotion task pack path is required")
	}
	if filepath.IsAbs(displayPath) || filepath.VolumeName(displayPath) != "" {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q must be relative to the consumer repo", displayPath)
	}
	if canonicalPath == "." || displayPath != canonicalPath {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q is not an exact canonical repo-relative identity", displayPath)
	}
	stepID := strings.TrimSpace(taskPackStepID)
	if stepID == "" || stepID != taskPackStepID {
		return nil, softwareDevPromotionIdentity{}, errors.New("promotion task pack step id must be an exact non-empty identity")
	}
	rootPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion consumer repo root is invalid: %w", err)
	}
	candidatePath, err := filepath.Abs(filepath.Join(rootPath, filepath.FromSlash(canonicalPath)))
	if err != nil || !withinRoot(rootPath, candidatePath) {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q escapes the consumer repo", displayPath)
	}
	if err := rejectSymlinkPath(rootPath, candidatePath, "promotion task pack"); err != nil {
		return nil, softwareDevPromotionIdentity{}, err
	}
	info, err := os.Stat(candidatePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion task pack path %q is not a readable regular file", displayPath)
	}
	repoTaskPack, err := loadTaskPack(rootPath, canonicalPath, stepID)
	if err != nil {
		return nil, softwareDevPromotionIdentity{}, err
	}

	siblingRelative := ""
	siblingPath := filepath.Join(filepath.Dir(candidatePath), "agents.yml")
	if siblingInfo, statErr := os.Lstat(siblingPath); statErr == nil {
		if siblingInfo.Mode()&os.ModeSymlink != 0 || !siblingInfo.Mode().IsRegular() {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q is not a repo-owned regular file", displayPath)
		}
		if err := rejectSymlinkPath(rootPath, siblingPath, "promotion sibling agents file"); err != nil {
			return nil, softwareDevPromotionIdentity{}, err
		}
		sibling := readYAMLMap(siblingPath)
		if _, ok := mapDeclaration(sibling["agents"]); !ok {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q has no agents mapping", displayPath)
		}
		relative, relErr := filepath.Rel(rootPath, siblingPath)
		if relErr != nil || strings.HasPrefix(relative, "..") {
			return nil, softwareDevPromotionIdentity{}, fmt.Errorf("promotion sibling agents path for %q escapes the consumer repo", displayPath)
		}
		siblingRelative = filepath.ToSlash(relative)
	} else if !os.IsNotExist(statErr) {
		return nil, softwareDevPromotionIdentity{}, fmt.Errorf("inspect promotion sibling agents path: %w", statErr)
	}

	return repoTaskPack, softwareDevPromotionIdentity{
		TaskPackPath: canonicalPath,
		StepID:       stepID,
		SiblingRoles: siblingRelative,
	}, nil
}

func rejectSymlinkPath(rootPath, targetPath, label string) error {
	relative, err := filepath.Rel(rootPath, targetPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s escapes the consumer repo", label)
	}
	current := rootPath
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return fmt.Errorf("%s cannot be inspected: %w", label, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s cannot use a symlinked identity: %s", label, filepath.ToSlash(relative))
		}
	}
	return nil
}

func requirePromotionDirectory(rootPath, directory, label string) error {
	if !withinRoot(rootPath, directory) {
		return fmt.Errorf("%s escapes the run artifact root", label)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("%s is missing: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a regular directory", label)
	}
	return nil
}

func readPromotionRawProposal(rootPath string) ([]byte, string, error) {
	type candidate struct {
		path     string
		relative string
	}
	found := []candidate{}
	for _, relative := range []string{"proposal/raw.yaml", "proposal/raw.json"} {
		path := filepath.Join(rootPath, filepath.FromSlash(relative))
		if _, err := os.Lstat(path); err == nil {
			found = append(found, candidate{path: path, relative: relative})
		} else if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("inspect %s: %w", relative, err)
		}
	}
	if len(found) != 1 {
		return nil, "", fmt.Errorf("promotion requires exactly one raw proposal artifact; found %d", len(found))
	}
	raw, err := readPromotionRegularFile(rootPath, found[0].path, found[0].relative)
	return raw, found[0].relative, err
}

func readPromotionJSONMap(rootPath, relative string) (map[string]any, error) {
	path := filepath.Join(rootPath, filepath.FromSlash(relative))
	raw, err := readPromotionRegularFile(rootPath, path, relative)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("promotion artifact %s is malformed JSON: %w", relative, err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("promotion artifact %s must be a non-empty JSON object", relative)
	}
	return payload, nil
}

func readPromotionRegularFile(rootPath, path, relative string) ([]byte, error) {
	if !withinRoot(rootPath, path) {
		return nil, fmt.Errorf("promotion artifact %s escapes the run artifact root", relative)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("promotion artifact %s is missing: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("promotion artifact %s is not a regular file", relative)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promotion artifact %s cannot be read: %w", relative, err)
	}
	return raw, nil
}
