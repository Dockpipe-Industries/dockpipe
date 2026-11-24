package machinecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"dockpipe/src/lib/infrastructure/telemetry"
)

func TestSnapshotDefaultsToBothCategoriesAndWritesMachineContract(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"snapshot", "--json"}, &output,
		func(context.Context, bool, bool) (telemetry.MachineSnapshot, error) {
			return telemetry.MachineSnapshot{Schema: telemetry.MachineSchema, Node: "host", State: "live"}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot telemetry.MachineSnapshot
	if err := json.Unmarshal(output.Bytes(), &snapshot); err != nil || snapshot.Schema != telemetry.MachineSchema || snapshot.Node != "host" {
		t.Fatal("invalid machine response", err)
	}
}

func TestSnapshotCategorySelectionAndInvalidArguments(t *testing.T) {
	for _, selection := range []struct {
		args           []string
		system, docker bool
	}{
		{[]string{"snapshot"}, true, true},
		{[]string{"snapshot", "--system"}, true, false},
		{[]string{"snapshot", "--docker"}, false, true},
	} {
		err := run(context.Background(), selection.args, &bytes.Buffer{},
			func(_ context.Context, system, docker bool) (telemetry.MachineSnapshot, error) {
				if system != selection.system || docker != selection.docker {
					t.Fatal("wrong inspection category")
				}
				return telemetry.MachineSnapshot{}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	err := run(context.Background(), []string{"snapshot", "--command", "whoami"}, &bytes.Buffer{},
		func(context.Context, bool, bool) (telemetry.MachineSnapshot, error) {
			t.Fatal("invalid request reached collection")
			return telemetry.MachineSnapshot{}, nil
		})
	if err == nil {
		t.Fatal("arbitrary command arguments were accepted")
	}
}

func TestSnapshotCancellationReturnsNoPartialResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var output bytes.Buffer
	err := run(ctx, []string{"snapshot"}, &output,
		func(context.Context, bool, bool) (telemetry.MachineSnapshot, error) {
			cancel()
			return telemetry.MachineSnapshot{State: "live"}, nil
		})
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatal("cancelled collection produced a successful response", err)
	}
}
