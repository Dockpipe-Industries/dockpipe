package infrastructure

import (
	"errors"
	"os/exec"
	"strings"
)

// bindSessionRepositoryIsEmpty distinguishes a fresh repository from an orphan
// branch in an existing repository. Unexpected Git errors are never absence.
func bindSessionRepositoryIsEmpty(workdir string) (bool, error) {
	_, err := gitOutput(workdir, "rev-parse", "--verify", "--quiet", "HEAD")
	if err == nil {
		return false, nil
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		return false, err
	}
	branches, err := gitOutput(workdir, "for-each-ref", "--count=1", "--format=%(refname)", "refs/heads")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(branches) == "", nil
}
