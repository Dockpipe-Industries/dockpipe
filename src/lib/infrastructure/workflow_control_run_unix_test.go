//go:build linux || darwin

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"golang.org/x/sys/unix"
)

func shellControlFrame(writer *controlWireWriter) string {
	var output bytes.Buffer
	_ = writer.send(&output)
	var encoded strings.Builder
	for _, value := range output.Bytes() {
		fmt.Fprintf(&encoded, "\\%03o", value)
	}
	return "printf '%b' '" + encoded.String() + "'\n"
}
func controlCall(sequence uint64, id string) *controlWireWriter {
	frame := &controlWireWriter{}
	frame.byte(1)
	frame.number(sequence, 8)
	frame.text(id)
	frame.number(0, 4)
	return frame
}
func controlDone(outcome byte, message string) *controlWireWriter {
	frame := &controlWireWriter{}
	frame.byte(3)
	frame.byte(outcome)
	frame.text(message)
	return frame
}

func TestWorkflowControlRunsDeclaredCallbacksAndKeepsEarlierError(t *testing.T) {
	definition := controlTestWorkflow(t)
	cleanup := definition.Steps[0]
	cleanup.ID = "cleanup"
	definition.Steps = append(definition.Steps, cleanup)
	expected := &controlTestExitError{23}
	script := "#!/bin/sh\n/bin/dd bs=1 count=13 of=/dev/null 2>/dev/null\n"
	script += shellControlFrame(controlCall(1, "test"))
	script += fmt.Sprintf("/bin/dd bs=1 count=%d of=/dev/null 2>/dev/null\n", 18+len(expected.Error()))
	script += shellControlFrame(controlCall(2, "cleanup"))
	script += "/bin/dd bs=1 count=18 of=/dev/null 2>/dev/null\n"
	script += shellControlFrame(controlDone(2, expected.Error()))
	root := t.TempDir()
	publishControlTestArtifact(t, root, definition, []byte(script))
	var calls []string
	err := RunWorkflowControl(WorkflowControlRunOptions{Workdir: root, Workflow: definition, Invoke: func(_ context.Context, id string, _ map[string]string) error {
		calls = append(calls, id)
		if id == "test" {
			return expected
		}
		return nil
	}})
	if !errors.Is(err, expected) || strings.Join(calls, ",") != "test,cleanup" {
		t.Fatalf("lost callback outcomes: %v %v", calls, err)
	}
}

type controlTestExitError struct{ Code int }

func (err *controlTestExitError) Error() string { return fmt.Sprintf("exit code %d", err.Code) }

func TestWorkflowControlRejectsFIFOBeforeOpening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.json")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readControlMetadata(path, 256); err == nil {
		t.Fatal("FIFO metadata accepted")
	}
	if _, _, err := controlFileDigest(path, 256); err == nil {
		t.Fatal("FIFO binary accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("FIFO rejection did not return promptly")
	}
}

func TestWorkflowControlCancellationJoinsProcess(t *testing.T) {
	root := t.TempDir()
	definition := controlTestWorkflow(t)
	publishControlTestArtifact(t, root, definition, []byte("#!/bin/sh\n/bin/dd bs=1 count=13 of=/dev/null 2>/dev/null\n/bin/sleep 30\n"))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	started := time.Now()
	err := RunWorkflowControl(WorkflowControlRunOptions{Workdir: root, Workflow: definition, Context: ctx, Invoke: func(context.Context, string, map[string]string) error { t.Error("unexpected callback"); return nil }})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("controller cancellation was not joined promptly")
	}
}

func TestWorkflowControlCancellationJoinsActiveCallback(t *testing.T) {
	root := t.TempDir()
	definition := controlTestWorkflow(t)
	script := "#!/bin/sh\n/bin/dd bs=1 count=13 of=/dev/null 2>/dev/null\n"
	script += shellControlFrame(controlCall(1, "test"))
	script += "/bin/sleep 30\n"
	publishControlTestArtifact(t, root, definition, []byte(script))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	callbackStarted := make(chan struct{})
	callbackJoined := make(chan struct{})
	cancellationJoined := make(chan struct{})
	go func() {
		defer close(cancellationJoined)
		select {
		case <-callbackStarted:
			cancel()
		case <-ctx.Done():
		}
	}()
	defer func() {
		cancel()
		<-cancellationJoined
	}()

	err := RunWorkflowControl(WorkflowControlRunOptions{
		Workdir: root, Workflow: definition, Context: ctx,
		Invoke: func(callbackContext context.Context, _ string, _ map[string]string) error {
			close(callbackStarted)
			<-callbackContext.Done()
			close(callbackJoined)
			return callbackContext.Err()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("active callback lost controller cancellation: %v", err)
	}
	select {
	case <-callbackJoined:
	default:
		t.Fatal("controller returned before joining its active callback")
	}
}

func TestWorkflowControlUsesOriginalArtifactRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	runRoot := t.TempDir()
	definition := controlTestWorkflow(t)
	script := "#!/bin/sh\n/bin/dd bs=1 count=13 of=/dev/null 2>/dev/null\n" + shellControlFrame(controlDone(0, ""))
	publishControlTestArtifact(t, artifactRoot, definition, []byte(script))
	if err := RunWorkflowControl(WorkflowControlRunOptions{Workdir: runRoot, ArtifactWorkdir: artifactRoot, Workflow: definition, Invoke: func(context.Context, string, map[string]string) error { return nil }}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowControlArtifactAdmissionCanDenyBeforeSpawn(t *testing.T) {
	root := t.TempDir()
	definition := controlTestWorkflow(t)
	publishControlTestArtifact(t, root, definition, []byte("#!/bin/sh\nexit 99\n"))
	denied := errors.New("controller denied")
	called := false
	err := RunWorkflowControl(WorkflowControlRunOptions{Workdir: root, Workflow: definition,
		Admission: NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
			called = true
			if binding.Contract.Capability != "workflow.control" {
				t.Errorf("wrong capability: %v", binding.Contract)
			}
			return denied
		}}, Invoke: func(context.Context, string, map[string]string) error {
			t.Error("callback after denied admission")
			return nil
		}})
	if !called || !errors.Is(err, denied) {
		t.Fatalf("admission bypassed: %v", err)
	}
}
