package resultcmd

import (
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestRunValidatesRequiredFields(t *testing.T) {
	if err := Run([]string{"--status", infrastructure.OperationStatusDone}); err == nil || !strings.Contains(err.Error(), "--unit is required") {
		t.Fatalf("expected missing unit error, got %v", err)
	}
	if err := Run([]string{"--unit", "x", "--status", "ok"}); err == nil || !strings.Contains(err.Error(), "--status must be") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if err := Run([]string{"--unit", "x", "--status", infrastructure.OperationStatusDone, "--id", "nope"}); err == nil || !strings.Contains(err.Error(), "--id requires key=value") {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}
