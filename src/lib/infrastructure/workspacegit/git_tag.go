package workspacegit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// WorkspaceGitTagPublication records an exact remote observation, not a local tag.
type WorkspaceGitTagPublication struct {
	Name             string `json:"name"`
	Commit           string `json:"commit"`
	Remote           string `json:"remote"`
	RemoteIdentity   string `json:"remote_identity"`
	DestinationRef   string `json:"destination_ref"`
	ObservedCommit   string `json:"observed_commit"`
	AlreadyPublished bool   `json:"already_published"`
	Recovered        bool   `json:"recovered"`
}

type workspaceGitTagCommand func(context.Context, string, ...string) ([]byte, error)

func publishWorkspaceGitTag(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest) (*WorkspaceGitSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return publishWorkspaceGitTagWithCommand(ctx, current, request, workspaceGitCommand)
}

func publishWorkspaceGitTagWithCommand(ctx context.Context, current *WorkspaceGitSnapshot, request WorkspaceGitRequest, run workspaceGitTagCommand) (*WorkspaceGitSnapshot, error) {
	if !request.Confirm {
		return nil, errors.New("tag publication requires explicit confirmation")
	}
	if request.Snapshot == "" || request.Snapshot != current.Snapshot {
		return nil, errors.New("the project changed since review; refresh before continuing")
	}
	if len(current.Changes) != 0 || current.Operation != nil {
		return nil, errors.New("tag publication requires a clean project with no active Git operation")
	}
	if len(request.Commit) != len(current.Head) || (len(request.Commit) != 40 && len(request.Commit) != 64) {
		return nil, errors.New("tag publication requires an exact full commit object ID matching the reviewed repository format")
	}
	if err := workspaceGitExactCommit(ctx, current.Root, request.Commit); err != nil {
		return nil, err
	}
	if current.Head == "" {
		return nil, errors.New("tag publication requires a reviewed HEAD commit")
	}
	if _, err := run(ctx, current.Root, "merge-base", "--is-ancestor", request.Commit, current.Head); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("tag publication commit must be reachable from the reviewed HEAD")
	}
	name := request.Name
	if name == "" || len(name) > 256 || name != strings.TrimSpace(name) || strings.HasPrefix(name, "-") ||
		strings.HasPrefix(name, "refs/") || strings.ContainsAny(name, "\x00\r\n") {
		return nil, errors.New("select one valid tag name without a refs prefix")
	}
	ref := "refs/tags/" + name
	if _, err := run(ctx, current.Root, "check-ref-format", ref); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("select one valid tag name")
	}
	destination, err := workspaceGitTagDestination(ctx, current.Root, run)
	if err != nil {
		return nil, err
	}
	if err := validateWorkspaceGitTagLiteralDestination(ctx, current.Root, destination, run); err != nil {
		return nil, err
	}
	observed, err := observeWorkspaceGitTag(ctx, current.Root, destination, ref, len(request.Commit), run)
	if err != nil {
		return nil, err
	}
	if observed != "" && observed != request.Commit {
		return nil, errors.New("remote tag already identifies a different object; it will not be overwritten")
	}
	if err := revalidateWorkspaceGitTagProject(ctx, current); err != nil {
		return nil, err
	}
	confirmedDestination, err := workspaceGitTagDestination(ctx, current.Root, run)
	if err != nil {
		return nil, err
	}
	if confirmedDestination != destination {
		return nil, errors.New("remote push destination changed during review; refresh before continuing")
	}

	if err := validateWorkspaceGitTagLiteralDestination(ctx, current.Root, destination, run); err != nil {
		return nil, err
	}

	alreadyPublished := observed == request.Commit
	recovered := false
	if !alreadyPublished {
		_, pushErr := run(ctx, current.Root, "push", "--no-follow-tags", "--porcelain", "--recurse-submodules=no", "--", destination, request.Commit+":"+ref)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		observed, err = observeWorkspaceGitTag(ctx, current.Root, destination, ref, len(request.Commit), run)
		if err != nil {
			return nil, fmt.Errorf("tag publication outcome could not be verified; refresh before retrying: %w", err)
		}
		if observed != request.Commit {
			if observed != "" {
				return nil, errors.New("remote tag identifies a different object; it was not overwritten")
			}
			if pushErr != nil {
				return nil, errors.New("tag publication push failed; remote tag remains absent")
			}
			return nil, errors.New("tag publication push returned success but remote tag is absent; refresh before retrying")
		}
		recovered = pushErr != nil
	}
	updated, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return nil, fmt.Errorf("remote tag verified but project refresh failed: %w", err)
	}
	if updated.Snapshot != current.Snapshot {
		return nil, errors.New("remote tag verified but project changed during publication; refresh before continuing")
	}
	identity := sha256.Sum256([]byte(destination))
	updated.TagPublication = &WorkspaceGitTagPublication{
		Name: name, Commit: request.Commit, Remote: "origin", RemoteIdentity: hex.EncodeToString(identity[:]),
		DestinationRef: ref, ObservedCommit: observed, AlreadyPublished: alreadyPublished, Recovered: recovered,
	}
	updated.Notice = "remote tag publication verified"
	return updated, nil
}

func workspaceGitTagDestination(ctx context.Context, root string, run workspaceGitTagCommand) (string, error) {
	output, err := run(ctx, root, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("tag publication cannot resolve the configured origin push destination")
	}
	destination := strings.TrimSuffix(string(output), "\n")
	if destination == "" || destination != strings.TrimSpace(destination) || strings.ContainsAny(destination, "\x00\r\n") || strings.HasPrefix(destination, "-") {
		return "", errors.New("tag publication requires exactly one unambiguous origin push destination")
	}
	// A relative local path must remain a path, not be reinterpreted as the
	// name of another configured remote with different fetch/push destinations.
	if !filepath.IsAbs(destination) && !strings.Contains(destination, ":") {
		destination = filepath.Join(root, destination)
	}
	return destination, nil
}

func validateWorkspaceGitTagLiteralDestination(ctx context.Context, root, destination string, run workspaceGitTagCommand) error {
	// Even a literal-looking URL must not select a configured remote whose
	// explicit pushurl could differ from its fetch URL.
	_, err := run(ctx, root, "config", "--get-regexp", "^remote\\."+regexp.QuoteMeta(destination)+"\\.")
	if err == nil {
		return errors.New("resolved tag destination is also a configured remote name; publication refused")
	}
	if !workspaceGitMissingRef(err) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("cannot verify literal tag destination is not a remote alias")
	}
	observedURL, err := run(ctx, root, "ls-remote", "--get-url", "--", destination)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("cannot verify literal tag observation destination")
	}
	if strings.TrimSuffix(string(observedURL), "\n") != destination {
		return errors.New("resolved tag observation destination has additional URL rewrites; publication refused")
	}

	// Literal pushes can apply pushInsteadOf even when origin's explicit
	// pushurl bypassed it. Conservatively reject every rule matching the
	// resolved literal, rather than guessing Git's rewrite selection order.
	rules, err := run(ctx, root, "config", "--null", "--get-regexp", "^url\\.")
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if workspaceGitMissingRef(err) {
			return nil
		}
		return errors.New("cannot inspect tag destination URL rewrite rules")
	}
	matches, err := workspaceGitTagLiteralRewrite(rules, destination)
	if err != nil {
		return err
	}
	if matches {
		return errors.New("resolved tag destination matches an additional URL rewrite; publication refused")
	}
	return nil
}

func workspaceGitTagLiteralRewrite(rules []byte, destination string) (bool, error) {
	// config --null --get-regexp emits key LF value NUL records. Reject
	// multiline keys/values instead of ambiguously interpreting either half.
	for len(rules) != 0 {
		end := bytes.IndexByte(rules, 0)
		if end < 0 {
			return false, errors.New("tag URL rewrite configuration is malformed")
		}
		record := rules[:end]
		rules = rules[end+1:]
		separator := bytes.IndexByte(record, '\n')
		if separator < 0 || separator != bytes.LastIndexByte(record, '\n') {
			return false, errors.New("tag URL rewrite configuration is ambiguous")
		}
		key := strings.ToLower(string(record[:separator]))
		if !strings.HasPrefix(key, "url.") {
			return false, errors.New("tag URL rewrite configuration has an unexpected key")
		}
		if strings.HasSuffix(key, ".insteadof") || strings.HasSuffix(key, ".pushinsteadof") {
			if strings.HasPrefix(destination, string(record[separator+1:])) {
				return true, nil
			}
		}
	}
	return false, nil
}

func observeWorkspaceGitTag(ctx context.Context, root, destination, ref string, oidLength int, run workspaceGitTagCommand) (string, error) {
	output, err := run(ctx, root, "ls-remote", "--refs", "--", destination, ref)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("remote tag observation failed")
	}
	return parseWorkspaceGitTagObservation(output, ref, oidLength)
}

func parseWorkspaceGitTagObservation(output []byte, ref string, oidLength int) (string, error) {
	if len(output) == 0 {
		return "", nil
	}
	// One canonical full OID, TAB, the exact bounded ref, and an optional LF.
	if (oidLength != 40 && oidLength != 64) || len(output) > oidLength+1+len(ref)+1 {
		return "", errors.New("remote tag observation is oversized or ambiguous")
	}
	fields := strings.Split(strings.TrimSuffix(string(output), "\n"), "\t")
	if len(fields) != 2 || len(fields[0]) != oidLength || fields[1] != ref {
		return "", errors.New("remote tag observation does not identify the exact requested ref")
	}
	for _, char := range fields[0] {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return "", errors.New("remote tag observation contains an invalid object ID")
		}
	}
	return fields[0], nil
}

func revalidateWorkspaceGitTagProject(ctx context.Context, current *WorkspaceGitSnapshot) error {
	latest, err := readWorkspaceGitSnapshot(ctx, current.Root)
	if err != nil {
		return err
	}
	if latest.Snapshot != current.Snapshot || len(latest.Changes) != 0 || latest.Operation != nil {
		return errors.New("the project changed since review; refresh before tag publication")
	}
	return nil
}
