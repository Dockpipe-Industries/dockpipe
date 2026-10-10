//go:build linux

package executor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/guest"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
)

func TestDeterministicGoNoCloudISO(t *testing.T) {
	c := executorFixture(t)
	first, err := buildNoCloudISO(c.NoCloud)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildNoCloudISO(c.NoCloud)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("NoCloud ISO is not deterministic")
	}
	if string(first[16*isoSector+1:16*isoSector+6]) != "CD001" || !bytes.Contains(first, []byte("META-DATA;1")) || !bytes.Contains(first, []byte("NETWORK-CONFIG;1")) || !bytes.Contains(first, []byte("USER-DATA;1")) {
		t.Fatal("NoCloud ISO layout changed")
	}
}

func TestLinuxRunnerCreatesExclusiveSparseDiskAndSeedWithoutExternalTools(t *testing.T) {
	c := executorFixture(t)
	if err := os.MkdirAll(filepath.Dir(c.DataDisk.Target), 0o700); err != nil {
		t.Fatal(err)
	}
	r := &LinuxRunner{}
	if err := r.CreateSparseRawDisk(context.Background(), c.DataDisk); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(c.DataDisk.Target)
	if err != nil || info.Size() != manifest.DataDiskBytes || info.Mode().Perm() != 0o600 {
		t.Fatalf("sparse disk mismatch: %v %v", info, err)
	}
	if err := r.CreateNoCloudSeed(context.Background(), c.NoCloud); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.NoCloud.Target); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateSparseRawDisk(context.Background(), c.DataDisk); err == nil {
		t.Fatal("expected exclusive data-disk rejection")
	}
}

func TestLinuxRunnerUsesOnlyExactPinnedQEMUImageArgv(t *testing.T) {
	execution := executorFixture(t)
	targetDir := filepath.Dir(execution.OSClone.Target)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(execution.Guest.Evidence), 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source.img")
	sourceBytes := []byte("pinned-source")
	if err := os.WriteFile(source, sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(t.TempDir(), "qemu-img")
	script := []byte("#!/bin/sh\nset -eu\n[ \"$1\" = create ]\n[ \"$2\" = -f ]\n[ \"$6\" = -b ]\n: > \"$8\"\n")
	if err := os.WriteFile(tool, script, 0o500); err != nil {
		t.Fatal(err)
	}
	sourceSum := sha256.Sum256(sourceBytes)
	toolSum := sha256.Sum256(script)
	execution.OSClone.Source = source
	execution.OSClone.SourceSHA256 = hex.EncodeToString(sourceSum[:])
	execution.OSClone.Command.Binary = tool
	execution.OSClone.Command.BinarySHA256 = hex.EncodeToString(toolSum[:])
	execution.OSClone.Command.Args = []string{"create", "-f", "qcow2", "-F", "qcow2", "-b", source, execution.OSClone.Target}
	r := &LinuxRunner{config: RunnerConfig{Contract: provisioning.Contract{SourceImage: provisioning.SourceImage{Path: source, SHA256: execution.OSClone.SourceSHA256, Bytes: int64(len(sourceBytes))}}, Execution: execution}}
	if err := r.CreateOSClone(context.Background(), execution.OSClone); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(execution.OSClone.Target)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("exact clone mismatch: %v %v", info, err)
	}
}

func TestLinuxRunnerPerformsGuestFirstSignedVerificationAndDurableEvidence(t *testing.T) {
	execution := executorFixture(t)
	if err := os.MkdirAll(filepath.Dir(execution.Guest.Evidence), 0o700); err != nil {
		t.Fatal(err)
	}
	controllerPublic, controllerPrivate, _ := ed25519.GenerateKey(rand.Reader)
	guestPublic, guestPrivate, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1_800_000_000, 0)
	m := manifest.Manifest{MachineUUID: "11111111-1111-4111-8111-111111111111", RunID: "run-001", Scenario: "sqlite-wal", DurabilityBoundary: "after-fsync", BootIDSource: manifest.KernelBootIDSource, DataDisk: manifest.Disk{Serial: "dockpipe-data-000001"}}
	contract := provisioning.Contract{RunID: "run-001", CohortID: "cohort-001", BootstrapNonce: execution.Guest.Bootstrap.BootstrapNonce, Artifacts: provisioning.Artifacts{ControllerPublicKeySHA256: hashBytes(controllerPublic), GuestPublicKeySHA256: hashBytes(guestPublic), ControllerBinarySHA256: execution.NoCloud.Files[0].SHA256, GuestAgentBinarySHA256: execution.NoCloud.Files[1].SHA256}}
	keys := provisioning.KeyMaterial{ControllerPublic: controllerPublic, ControllerPrivate: controllerPrivate, GuestPublic: guestPublic, GuestPrivate: guestPrivate}
	payload := protocol.IdentityBootstrapPayload{BootIDSource: manifest.KernelBootIDSource, ControllerPublicKeySHA256: contract.Artifacts.ControllerPublicKeySHA256, GuestPublicKeySHA256: contract.Artifacts.GuestPublicKeySHA256, ControllerBinarySHA256: contract.Artifacts.ControllerBinarySHA256, GuestAgentBinarySHA256: contract.Artifacts.GuestAgentBinarySHA256}
	expected := protocol.Context{MachineUUID: m.MachineUUID, DiskSerial: m.DataDisk.Serial, BootID: "22222222-2222-4222-8222-222222222222", RunID: m.RunID, Scenario: m.Scenario, DurabilityBoundary: m.DurabilityBoundary}
	service := &guest.Service{ControllerPublic: controllerPublic, GuestPrivate: guestPrivate, Expected: expected, AgentSHA256: contract.Artifacts.GuestAgentBinarySHA256, ControllerSHA256: contract.Artifacts.ControllerBinarySHA256, BootstrapNonce: contract.BootstrapNonce, BootIDSource: manifest.KernelBootIDSource, BootstrapPayload: payload, Now: func() time.Time { return now }}
	service.Replay = protocol.NewReplayGuardAfterBootstrap(expected, contract.BootstrapNonce)
	clientConn, serverConn := net.Pipe()
	serverErr := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		serverErr <- service.Serve(serverConn)
	}()
	runner, err := NewLinuxRunner(RunnerConfig{Contract: contract, Manifest: m, Keys: keys, Execution: execution, Now: func() time.Time { return now }, Dial: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }})
	if err != nil {
		t.Fatal(err)
	}
	consoleReader, consoleWriter := net.Pipe()
	consoleFile, err := os.OpenFile(execution.FirstBootObservation.EvidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	runner.observation = &observationSession{policy: *execution.FirstBootObservation, conn: consoleReader, file: consoleFile, syncDir: syncDirectory}
	go func() {
		_, _ = consoleWriter.Write([]byte("first-boot fixture\n"))
		_ = consoleWriter.Close()
	}()
	if err := runner.VerifyGuest(context.Background(), execution.Guest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(execution.Guest.Bootstrap.ExclusiveEvidencePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(execution.Guest.Evidence); err != nil {
		t.Fatal(err)
	}
	clientConn.Close()
	select {
	case <-serverErr:
	case <-time.After(time.Second):
		t.Fatal("guest fixture did not terminate")
	}
}

func TestGuestVerificationTimeoutEvidenceIsExclusiveDurableAndNonSecret(t *testing.T) {
	execution := executorFixture(t)
	if err := os.MkdirAll(filepath.Dir(execution.Guest.FailureEvidence), 0o700); err != nil {
		t.Fatal(err)
	}
	progress := guestVerificationProgress{BootstrapVerified: true, CompletedCapabilities: []string{"identity/v1"}}
	if err := persistGuestVerificationTimeout(execution.Guest, progress); err != nil {
		t.Fatal(err)
	}
	evidence, err := os.ReadFile(execution.Guest.FailureEvidence)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"dockpipe.vm.guest-verification-failure.v1","operation":"verify-guest","reason":"timeout","timeout_seconds":300,"bootstrap_verified":true,"completed_capabilities":["identity/v1"]}`
	if string(evidence) != want {
		t.Fatalf("unexpected timeout evidence: %s", evidence)
	}
	for _, forbidden := range []string{"run_id", "cohort_id", "boot_id", "nonce", "frame", "payload", "key", "path", "timestamp"} {
		if bytes.Contains(evidence, []byte(forbidden)) {
			t.Fatalf("timeout evidence disclosed forbidden field %q: %s", forbidden, evidence)
		}
	}
	info, err := os.Lstat(execution.Guest.FailureEvidence)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("timeout evidence metadata changed: info=%v err=%v", info, err)
	}
	if err := persistGuestVerificationTimeout(execution.Guest, progress); err == nil {
		t.Fatal("timeout evidence must be exclusively created")
	}
}

func TestLinuxRunnerPersistsTimeoutEvidenceWhenBootstrapReadExpires(t *testing.T) {
	execution := executorFixture(t)
	if err := os.MkdirAll(filepath.Dir(execution.Guest.FailureEvidence), 0o700); err != nil {
		t.Fatal(err)
	}
	verificationClient, verificationServer := net.Pipe()
	defer verificationServer.Close()
	runner := &LinuxRunner{config: RunnerConfig{Dial: func(context.Context, string, string) (net.Conn, error) {
		return verificationClient, nil
	}}}
	consoleReader, consoleWriter := net.Pipe()
	consoleFile, err := os.OpenFile(execution.FirstBootObservation.EvidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	runner.observation = &observationSession{policy: *execution.FirstBootObservation, conn: consoleReader, file: consoleFile, syncDir: syncDirectory}
	go func() {
		_, _ = consoleWriter.Write([]byte("first-boot progress fixture\n"))
		_ = consoleWriter.Close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := runner.VerifyGuest(ctx, execution.Guest); err == nil {
		t.Fatal("expected bootstrap read timeout")
	}
	evidence, err := os.ReadFile(execution.Guest.FailureEvidence)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"dockpipe.vm.guest-verification-failure.v1","operation":"verify-guest","reason":"timeout","timeout_seconds":300,"bootstrap_verified":false,"completed_capabilities":[]}`
	if string(evidence) != want {
		t.Fatalf("unexpected bootstrap timeout evidence: %s", evidence)
	}
}
