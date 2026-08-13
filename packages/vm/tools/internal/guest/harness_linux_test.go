//go:build linux

package guest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHarnessProvidesOnlyReviewedRoleAndLookupPath(t *testing.T) {
	t.Setenv("DOCKPIPE_UNREVIEWED", "present-in-parent")
	command := harnessCommand{
		Schema:   harnessCommandSchema,
		CohortID: "cohort-001",
		TrialID:  "after-stage-before-commit-1",
		Boundary: "after-stage-before-commit",
		Attempt:  1,
		Root:     "/var/lib/dockpipe-qualification/cohorts/cohort-001/after-stage-before-commit/attempt-1",
	}
	hash := strings.Repeat("a", 64)
	evidence := fmt.Sprintf(
		`{"schema":"%s","cohort_id":"%s","trial_id":"%s","boundary":"%s","attempt":%d,"root":"%s","root_identity":"fixture-root","database":"%s","expected_revision":1,"observed_revision":1,"observed_digest":"%s","pre_metadata_sha256":"%s","post_metadata_sha256":"%s","compile_options_sha256":"%s","sqlite_version":"3.53.3","sqlite_source_id":"2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62","vfs":"unix","quick_check":"ok","retries":0,"replays":0,"repairs":0,"fallbacks":0}`,
		harnessRecoverySchema, command.CohortID, command.TrialID, command.Boundary, command.Attempt, command.Root,
		filepath.Join(command.Root, "sqlite", "main", "aggregate.sqlite"), hash, hash, hash, hash,
	)
	script := fmt.Sprintf(`#!/bin/sh
set -eu
[ "$DORKPIPE_SQLITE_VM_HARNESS_ROLE" = "recovery" ]
[ "$PATH" = "/usr/bin:/bin" ]
[ -z "${DOCKPIPE_UNREVIEWED+x}" ]
[ "$(command -v systemd-detect-virt)" = "/usr/bin/systemd-detect-virt" ]
cat >/dev/null
printf '%%s\n' '%s'
`, evidence)
	binaryPath := filepath.Join(t.TempDir(), "pinned-harness")
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := &linuxHarnessAdapter{binaryPath: binaryPath}
	if _, _, _, err := adapter.runHarness(command, harnessRecoveryRole, false); err != nil {
		t.Fatalf("run harness with reviewed environment: %v", err)
	}
}
