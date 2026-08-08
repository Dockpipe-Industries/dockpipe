//go:build linux

package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/provisioning"
)

func TestBoundedConsoleCaptureRetainsExactPrefixAndFailsClosedOnOverflow(t *testing.T) {
	for _, test := range []struct {
		name     string
		bytes    int
		overflow bool
	}{
		{name: "exact cap", bytes: int(provisioning.FirstBootObservationMaxBytes)},
		{name: "one byte overflow", bytes: int(provisioning.FirstBootObservationMaxBytes) + 1, overflow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "console.log")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			reader, writer := io.Pipe()
			capture, err := startBoundedConsoleCapture(reader, file, provisioning.FirstBootObservationMaxBytes)
			if err != nil {
				t.Fatal(err)
			}
			writeDone := make(chan error, 1)
			go func() {
				_, writeErr := writer.Write(bytesOf(test.bytes))
				_ = writer.Close()
				writeDone <- writeErr
			}()
			select {
			case <-capture.done:
			case <-time.After(5 * time.Second):
				t.Fatal("bounded capture goroutine did not terminate")
			}
			session := &observationSession{policy: provisioning.FirstBootObservationPlan{EvidencePath: path}, capture: capture, file: file, syncDir: syncDirectory}
			err = session.stopAndSync()
			if test.overflow != errors.Is(err, errFirstBootObservationOverflow) {
				t.Fatalf("overflow result mismatch: %v", err)
			}
			if writeErr := <-writeDone; writeErr != nil {
				t.Fatal(writeErr)
			}
			info, statErr := os.Stat(path)
			if statErr != nil || info.Size() != provisioning.FirstBootObservationMaxBytes {
				t.Fatalf("retained prefix mismatch: info=%v err=%v", info, statErr)
			}
		})
	}
}

func TestObservationSessionCreatesExclusiveOwnerOnlyEvidenceAndControllerListener(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "dpvm-observe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	evidenceRoot := filepath.Join(root, "e")
	runtimeRoot := filepath.Join(root, "r")
	for _, path := range []string{filepath.Join(evidenceRoot, "run-001", "cohort-001"), filepath.Join(runtimeRoot, "run-001", "cohort-001")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := provisioning.PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001", "cohort-001")
	if err != nil {
		t.Fatal(err)
	}
	listener := &fakeConsoleListener{}
	var listenedPath string
	session, err := prepareObservationSessionWithListener(policy, func(path string) (consoleListener, error) {
		listenedPath = path
		return listener, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(policy.EvidencePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("evidence ownership mode mismatch: info=%v err=%v", info, err)
	}
	if listenedPath != policy.SocketPath {
		t.Fatalf("controller listener path mismatch: got %q want %q", listenedPath, policy.SocketPath)
	}
	if _, err := prepareObservationSessionWithListener(policy, func(string) (consoleListener, error) { return &fakeConsoleListener{}, nil }); err == nil {
		t.Fatal("expected exclusive evidence/listener rejection")
	}
	if err := session.stopAndSync(); err != nil {
		t.Fatal(err)
	}
	if !listener.closed {
		t.Fatal("controller listener was not closed")
	}
}

type fakeConsoleListener struct {
	closed   bool
	closeErr error
}

func (f *fakeConsoleListener) Accept() (net.Conn, error) {
	return nil, errors.New("unused fake listener")
}
func (f *fakeConsoleListener) Close() error {
	f.closed = true
	return f.closeErr
}
func (f *fakeConsoleListener) SetDeadline(time.Time) error { return nil }

type failingObservationFile struct {
	writes   []byte
	syncErr  error
	closeErr error
}

func (f *failingObservationFile) Write(data []byte) (int, error) {
	f.writes = append(f.writes, data...)
	return len(data), nil
}
func (f *failingObservationFile) Sync() error  { return f.syncErr }
func (f *failingObservationFile) Close() error { return f.closeErr }

func TestObservationStopJoinsCaptureAndPropagatesFileAndDirectorySyncErrors(t *testing.T) {
	reader, writer := net.Pipe()
	fileSyncErr := errors.New("injected file sync failure")
	fileCloseErr := errors.New("injected file close failure")
	dirSyncErr := errors.New("injected directory sync failure")
	file := &failingObservationFile{syncErr: fileSyncErr, closeErr: fileCloseErr}
	capture, err := startBoundedConsoleCapture(reader, file, provisioning.FirstBootObservationMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = writer.Write([]byte("bounded output"))
		_ = writer.Close()
	}()
	select {
	case <-capture.done:
	case <-time.After(time.Second):
		t.Fatal("capture fixture did not finish")
	}
	session := &observationSession{
		policy:  provisioning.FirstBootObservationPlan{EvidencePath: filepath.Join(t.TempDir(), "console.log")},
		conn:    reader,
		file:    file,
		capture: capture,
		syncDir: func(string) error { return dirSyncErr },
	}
	err = session.stopAndSync()
	if !errors.Is(err, fileSyncErr) || !errors.Is(err, fileCloseErr) || !errors.Is(err, dirSyncErr) {
		t.Fatalf("durability errors were not propagated: %v", err)
	}
	select {
	case <-capture.done:
	default:
		t.Fatal("capture goroutine was not joined")
	}
	if string(file.writes) != "bounded output" {
		t.Fatalf("unexpected retained output: %q", file.writes)
	}
}

func TestObservationSetupFailurePropagatesEveryOwnedResourceError(t *testing.T) {
	listenerCloseErr := errors.New("injected listener close failure")
	fileSyncErr := errors.New("injected setup file sync failure")
	fileCloseErr := errors.New("injected setup file close failure")
	dirSyncErr := errors.New("injected setup directory sync failure")
	listener := &fakeConsoleListener{closeErr: listenerCloseErr}
	file := &failingObservationFile{syncErr: fileSyncErr, closeErr: fileCloseErr}

	err := finishFailedObservationSetup(listener, file, "/unused/evidence", func(string) error { return dirSyncErr })
	for _, want := range []error{listenerCloseErr, fileSyncErr, fileCloseErr, dirSyncErr} {
		if !errors.Is(err, want) {
			t.Fatalf("setup failure did not propagate %v: %v", want, err)
		}
	}
	if !listener.closed {
		t.Fatal("failed setup did not close its listener")
	}
}

func TestObservationCaptureClosesAllOwnedFileDescriptors(t *testing.T) {
	baseline, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("file-descriptor inventory unavailable: %v", err)
	}
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(filepath.Join(root, "console-"+time.Now().Add(time.Duration(i)).Format("150405.000000000")+".log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		capture, err := startBoundedConsoleCapture(reader, file, provisioning.FirstBootObservationMaxBytes)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte("fd lifecycle\n")); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		<-capture.done
		session := &observationSession{policy: provisioning.FirstBootObservationPlan{EvidencePath: filepath.Join(root, "evidence.log")}, file: file, capture: capture, syncDir: func(string) error { return nil }}
		if err := session.stopAndSync(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(baseline)+1 {
		t.Fatalf("owned file descriptors leaked: before=%d after=%d", len(baseline), len(after))
	}
}

func TestPreservedExecutorV2RemainsLoadableForExactCleanupOnly(t *testing.T) {
	legacy := executorFixture(t)
	legacy.Schema = LegacyCleanupSchema
	legacy.FirstBootObservation = nil
	legacy.ProvisioningRoots = nil
	legacy.Guest.TimeoutSeconds = 60
	legacy.Launch.Command.Args = removeConsoleArgs(legacy.Launch.Command.Args)
	legacy.Cleanup.Resources = slices.Delete(legacy.Cleanup.Resources, 7, 8)
	legacy.ExecutionSHA256, _ = legacy.Digest()
	legacyV2Material := struct {
		Schema          string                   `json:"schema"`
		ContractSHA256  string                   `json:"contract_sha256"`
		PlanSHA256      string                   `json:"plan_sha256"`
		ToolchainSHA256 string                   `json:"toolchain_sha256"`
		ExecutionSHA256 string                   `json:"execution_sha256"`
		RunID           string                   `json:"run_id"`
		CohortID        string                   `json:"cohort_id"`
		OSClone         OSCloneRequest           `json:"os_clone"`
		DataDisk        SparseRawDiskRequest     `json:"data_disk"`
		NoCloud         NoCloudSeedRequest       `json:"nocloud"`
		Launch          LaunchRequest            `json:"launch"`
		Guest           GuestVerificationRequest `json:"guest_verification"`
		Shutdown        ShutdownRequest          `json:"shutdown"`
		Preservation    PreservationRequest      `json:"preservation"`
		Cleanup         CleanupRequest           `json:"cleanup"`
	}{
		legacy.Schema, legacy.ContractSHA256, legacy.PlanSHA256, legacy.ToolchainSHA256, "", legacy.RunID, legacy.CohortID,
		legacy.OSClone, legacy.DataDisk, legacy.NoCloud, legacy.Launch, legacy.Guest, legacy.Shutdown, legacy.Preservation, legacy.Cleanup,
	}
	legacyJSON, err := json.Marshal(legacyV2Material)
	if err != nil {
		t.Fatal(err)
	}
	wantLegacyDigest := sha256.Sum256(legacyJSON)
	if legacy.ExecutionSHA256 != hex.EncodeToString(wantLegacyDigest[:]) {
		t.Fatalf("executor-v2 historical digest shape changed: got %s want %s", legacy.ExecutionSHA256, hex.EncodeToString(wantLegacyDigest[:]))
	}
	path := filepath.Join(t.TempDir(), "executor-v2.json")
	if err := Store(path, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("preserved executor-v2 cleanup contract became unloadable: %v", err)
	}
	if _, err := Execute(context.Background(), loaded, &fakeRunner{}); err == nil {
		t.Fatal("legacy cleanup compatibility must not restore qualification execution")
	}
	now := time.Unix(1_800_000_000, 0)
	authorization := CleanupAuthorization{Schema: CleanupSchema, Approved: true, ContractSHA256: loaded.ContractSHA256, PlanSHA256: loaded.PlanSHA256, ExecutionSHA256: loaded.ExecutionSHA256, RunID: loaded.RunID, CohortID: loaded.CohortID, Resources: slices.Clone(loaded.Cleanup.Resources), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	runner := &fakeRunner{}
	if _, err := ExecuteCleanup(context.Background(), loaded, authorization, now, runner); err != nil {
		t.Fatalf("preserved executor-v2 exact cleanup compatibility failed: %v", err)
	}
}

func TestPreservedExecutorV3RemainsLoadableForExactCleanupOnly(t *testing.T) {
	legacy := executorFixture(t)
	legacy.Schema = LegacyObservationCleanupSchema
	legacy.Guest.TimeoutSeconds = 60
	legacy.ExecutionSHA256, _ = legacy.Digest()
	path := filepath.Join(t.TempDir(), "executor-v3.json")
	if err := Store(path, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("preserved executor-v3 cleanup contract became unloadable: %v", err)
	}
	if _, err := Execute(context.Background(), loaded, &fakeRunner{}); err == nil {
		t.Fatal("legacy observation executor must not regain qualification execution")
	}
	now := time.Unix(1_800_000_000, 0)
	authorization := CleanupAuthorization{Schema: CleanupSchema, Approved: true, ContractSHA256: loaded.ContractSHA256, PlanSHA256: loaded.PlanSHA256, ExecutionSHA256: loaded.ExecutionSHA256, RunID: loaded.RunID, CohortID: loaded.CohortID, Resources: slices.Clone(loaded.Cleanup.Resources), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	runner := &fakeRunner{}
	if _, err := ExecuteCleanup(context.Background(), loaded, authorization, now, runner); err != nil {
		t.Fatalf("preserved executor-v3 exact cleanup compatibility failed: %v", err)
	}
}

func TestPreservedExecutorV4RemainsLoadableForExactCleanupOnly(t *testing.T) {
	legacy := executorFixture(t)
	legacy.Schema = LegacyDeadlineCleanupSchema
	legacy.Guest.TimeoutSeconds = 180
	legacy.ExecutionSHA256, _ = legacy.Digest()
	path := filepath.Join(t.TempDir(), "executor-v4.json")
	if err := Store(path, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("preserved executor-v4 cleanup contract became unloadable: %v", err)
	}
	if _, err := Execute(context.Background(), loaded, &fakeRunner{}); err == nil {
		t.Fatal("legacy deadline executor must not regain qualification execution")
	}
	now := time.Unix(1_800_000_000, 0)
	authorization := CleanupAuthorization{Schema: CleanupSchema, Approved: true, ContractSHA256: loaded.ContractSHA256, PlanSHA256: loaded.PlanSHA256, ExecutionSHA256: loaded.ExecutionSHA256, RunID: loaded.RunID, CohortID: loaded.CohortID, Resources: slices.Clone(loaded.Cleanup.Resources), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	runner := &fakeRunner{}
	if _, err := ExecuteCleanup(context.Background(), loaded, authorization, now, runner); err != nil {
		t.Fatalf("preserved executor-v4 exact cleanup compatibility failed: %v", err)
	}
}

func TestPreservedExecutorV5RemainsLoadableForExactCleanupOnly(t *testing.T) {
	legacy := executorFixture(t)
	legacy.Schema = LegacyUserCreationCleanupSchema
	legacy.Guest.TimeoutSeconds = 240
	legacy.ExecutionSHA256, _ = legacy.Digest()
	path := filepath.Join(t.TempDir(), "executor-v5.json")
	if err := Store(path, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("preserved executor-v5 cleanup contract became unloadable: %v", err)
	}
	if _, err := Execute(context.Background(), loaded, &fakeRunner{}); err == nil {
		t.Fatal("legacy user-creation executor must not regain qualification execution")
	}
	now := time.Unix(1_800_000_000, 0)
	authorization := CleanupAuthorization{Schema: CleanupSchema, Approved: true, ContractSHA256: loaded.ContractSHA256, PlanSHA256: loaded.PlanSHA256, ExecutionSHA256: loaded.ExecutionSHA256, RunID: loaded.RunID, CohortID: loaded.CohortID, Resources: slices.Clone(loaded.Cleanup.Resources), ExpiresAtUnix: now.Add(time.Minute).Unix()}
	runner := &fakeRunner{}
	if _, err := ExecuteCleanup(context.Background(), loaded, authorization, now, runner); err != nil {
		t.Fatalf("preserved executor-v5 exact cleanup compatibility failed: %v", err)
	}
}

func removeConsoleArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) && (args[i] == "-chardev" && strings.HasPrefix(args[i+1], "socket,id=dockpipe-first-boot-console,") || args[i] == "-serial" && args[i+1] == "chardev:dockpipe-first-boot-console") {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func bytesOf(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}
