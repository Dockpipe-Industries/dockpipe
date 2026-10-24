package telemetry

import (
	"context"
	"testing"
)

func TestNativeInspectionRejectsExitAndPIDReuse(t *testing.T) {
	process := Process{ProcessIdentity: ProcessIdentity{PID: 42, StartToken: "creation-1"}, ParentPID: 7, Name: "worker"}
	for _, identity := range []ProcessIdentity{
		{PID: 42, StartToken: "creation-2"},
		{PID: 43, StartToken: "creation-1"},
		{PID: 42},
	} {
		if _, err := finishNativeInspection(context.Background(), process, identity, true); err == nil {
			t.Fatal("changed kernel process identity admitted")
		}
	}
	if _, err := finishNativeInspection(context.Background(), process, process.ProcessIdentity, false); err == nil {
		t.Fatal("exited process admitted")
	}
	result, err := finishNativeInspection(context.Background(), process, process.ProcessIdentity, true)
	if err != nil || result.ParentPID != 7 || result.Name != "worker" {
		t.Fatal("stable native process sample was changed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := finishNativeInspection(ctx, process, process.ProcessIdentity, true); err != context.Canceled {
		t.Fatal("native inspection cancellation was lost")
	}
}
