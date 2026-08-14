package guest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
)

type oneShotRW struct {
	reader *bytes.Reader
	writer bytes.Buffer
}

type failingObservationWriter struct{}

func (failingObservationWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type fakeHarnessAdapter struct {
	checkpoint HarnessRequest
	recovery   HarnessRequest
}

func (f *fakeHarnessAdapter) Checkpoint(request HarnessRequest) (any, error) {
	f.checkpoint = request
	if request.observeCheckpoint != nil {
		if err := request.observeCheckpoint(checkpointStagePendingAccepted, strings.Repeat("e", 64), ""); err != nil {
			return nil, err
		}
		if err := request.observeCheckpoint(checkpointStageHarnessEmitted, strings.Repeat("e", 64), strings.Repeat("f", 64)); err != nil {
			return nil, err
		}
	}
	return map[string]any{"accepted": true}, nil
}
func (f *fakeHarnessAdapter) Recovery(request HarnessRequest) (any, error) {
	f.recovery = request
	return map[string]any{"recovered": true}, nil
}

func (rw *oneShotRW) Read(p []byte) (int, error)  { return rw.reader.Read(p) }
func (rw *oneShotRW) Write(p []byte) (int, error) { return rw.writer.Write(p) }

func serviceFixture(t *testing.T) (*Service, ed25519.PrivateKey, ed25519.PublicKey, time.Time) {
	t.Helper()
	controllerPub, controllerPriv, _ := ed25519.GenerateKey(rand.Reader)
	guestPub, guestPriv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1_800_000_000, 0)
	expected := protocol.Context{MachineUUID: "11111111-1111-4111-8111-111111111111", DiskSerial: "dockpipe-data-000001", BootID: "22222222-2222-4222-8222-222222222222", RunID: "run-001", Scenario: "sqlite-wal", DurabilityBoundary: "after-fsync"}
	bootstrapNonce := strings.Repeat("b", 64)
	payload := protocol.IdentityBootstrapPayload{BootIDSource: manifest.KernelBootIDSource, ControllerPublicKeySHA256: hash(controllerPub), GuestPublicKeySHA256: hash(guestPub), ControllerBinarySHA256: strings.Repeat("c", 64), GuestAgentBinarySHA256: strings.Repeat("a", 64)}
	service := &Service{ControllerPublic: controllerPub, GuestPrivate: guestPriv, Expected: expected, AgentSHA256: strings.Repeat("a", 64), ControllerSHA256: strings.Repeat("c", 64), BootstrapNonce: bootstrapNonce, BootIDSource: manifest.KernelBootIDSource, BootstrapPayload: payload, Observability: io.Discard, Now: func() time.Time { return now }}
	service.Replay = protocol.NewReplayGuardAfterBootstrap(expected, bootstrapNonce)
	return service, controllerPriv, guestPub, now
}

func request(t *testing.T, private ed25519.PrivateKey, now time.Time, ctx protocol.Context, capability string, payload any) []byte {
	t.Helper()
	b, err := protocol.Sign(protocol.RequestKind, capability, ctx, payload, now, now.Add(time.Minute), private)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func requestContext(service *Service, sequence uint64, nonceByte string) protocol.Context {
	ctx := service.Expected
	ctx.Sequence = sequence
	ctx.Nonce = strings.Repeat(nonceByte, 64)
	ctx.Phase = "verification"
	return ctx
}

func TestServiceModeFramesVerifiesAndSignsReviewedCapabilities(t *testing.T) {
	service, controllerPriv, guestPub, now := serviceFixture(t)
	ctx := requestContext(service, protocol.FirstRequestSequence, "1")
	req := request(t, controllerPriv, now, ctx, "identity/v1", struct{}{})
	var framed bytes.Buffer
	if err := protocol.WriteFramed(&framed, req); err != nil {
		t.Fatal(err)
	}
	rw := &oneShotRW{reader: bytes.NewReader(framed.Bytes())}
	if err := service.Serve(rw); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := protocol.ReadFramed(&rw.writer)
	if err != nil {
		t.Fatal(err)
	}
	expected := service.Expected
	expected.BootID = ""
	if _, err := protocol.VerifyIdentityBootstrap(bootstrap, guestPub, now, expected, service.BootstrapNonce, service.BootstrapPayload); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.ReadFramed(&rw.writer)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := protocol.Verify(response, guestPub, now)
	if err != nil || frame.Kind != protocol.ResultKind || frame.Capability != "identity/v1" || frame.Context != ctx {
		t.Fatalf("signed response mismatch: %+v %v", frame, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(frame.Payload, &payload); err != nil || payload["machine_uuid"] != ctx.MachineUUID {
		t.Fatalf("identity response mismatch: %s", frame.Payload)
	}
}

func TestServiceModeRejectsSignatureReplaySequenceIdentityAndCapabilityInputs(t *testing.T) {
	service, controllerPriv, _, now := serviceFixture(t)
	ctx := requestContext(service, protocol.FirstRequestSequence, "1")
	valid := request(t, controllerPriv, now, ctx, "health/v1", struct{}{})
	otherService, _, _, _ := serviceFixture(t)
	if _, err := otherService.Handle(valid); err == nil {
		t.Fatal("expected signature pin rejection")
	}
	if _, err := service.Handle(valid); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Handle(valid); err == nil {
		t.Fatal("expected replay rejection")
	}
	service, controllerPriv, _, now = serviceFixture(t)
	ctx = requestContext(service, protocol.FirstRequestSequence, "2")
	nonRequest, err := protocol.Sign(protocol.ResultKind, "health/v1", ctx, struct{}{}, now, now.Add(time.Minute), controllerPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Handle(nonRequest); err == nil {
		t.Fatal("expected non-request frame rejection")
	}
	ctx = requestContext(service, protocol.FirstRequestSequence+2, "2")
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "health/v1", struct{}{})); err == nil {
		t.Fatal("expected sequence rejection")
	}
	service, controllerPriv, _, now = serviceFixture(t)
	ctx = requestContext(service, protocol.FirstRequestSequence, "3")
	ctx.MachineUUID = "33333333-3333-4333-8333-333333333333"
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "health/v1", struct{}{})); err == nil {
		t.Fatal("expected identity substitution rejection")
	}
	service, controllerPriv, _, now = serviceFixture(t)
	ctx = requestContext(service, protocol.FirstRequestSequence, "4")
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "launch-hash-pinned/v1", map[string]string{"controller_binary_sha256": strings.Repeat("c", 64), "guest_agent_binary_sha256": strings.Repeat("b", 64)})); err == nil {
		t.Fatal("expected binary hash substitution rejection")
	}
	if _, err := protocol.Sign("request", "exec/v1", ctx, map[string]string{"command": "id"}, now, now.Add(time.Minute), controllerPriv); err == nil {
		t.Fatal("expected arbitrary execution capability rejection")
	}
}

func TestServiceModeRejectsMalformedFraming(t *testing.T) {
	service, _, _, _ := serviceFixture(t)
	rw := &oneShotRW{reader: bytes.NewReader([]byte{0, 1, 0, 1})}
	if err := service.Serve(rw); err == nil || err == io.EOF {
		t.Fatal("expected oversized framing rejection")
	}
}

func TestServiceModeKeepsCheckpointAndRecoveryHarnessFailClosed(t *testing.T) {
	for _, capability := range []string{"checkpoint/v1", "recovery/v1"} {
		t.Run(capability, func(t *testing.T) {
			service, controllerPriv, _, now := serviceFixture(t)
			ctx := requestContext(service, protocol.FirstRequestSequence, "5")
			payload := map[string]any{"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1, "boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64), "harness_sha256": strings.Repeat("e", 64)}
			if capability == "recovery/v1" {
				payload["checkpoint_boot_id"] = "33333333-3333-4333-8333-333333333333"
			}
			if _, err := service.Handle(request(t, controllerPriv, now, ctx, capability, payload)); err == nil {
				t.Fatal("expected unowned harness capability to fail closed")
			}
		})
	}
}

func TestServiceModeRoutesExactSignedHarnessRequests(t *testing.T) {
	service, controllerPriv, guestPub, now := serviceFixture(t)
	adapter := &fakeHarnessAdapter{}
	service.Harness = adapter
	checkpointContext := requestContext(service, protocol.FirstRequestSequence, "6")
	checkpointPayload := map[string]any{"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1, "boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64), "harness_sha256": strings.Repeat("e", 64)}
	response, err := service.Handle(request(t, controllerPriv, now, checkpointContext, "checkpoint/v1", checkpointPayload))
	if err != nil {
		t.Fatal(err)
	}
	frame, err := protocol.Verify(response, guestPub, now)
	if err != nil || frame.Capability != "checkpoint/v1" || adapter.checkpoint.TrialID != "after-stage-before-commit-1" || adapter.checkpoint.BootID != service.Expected.BootID {
		t.Fatalf("checkpoint adapter routing changed: frame=%+v request=%+v err=%v", frame, adapter.checkpoint, err)
	}
	recoveryContext := requestContext(service, protocol.FirstRequestSequence+1, "7")
	recoveryPayload := map[string]any{"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1, "boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64), "checkpoint_boot_id": "33333333-3333-4333-8333-333333333333", "harness_sha256": strings.Repeat("e", 64)}
	if _, err := service.Handle(request(t, controllerPriv, now, recoveryContext, "recovery/v1", recoveryPayload)); err != nil {
		t.Fatal(err)
	}
	if adapter.recovery.CheckpointBootID != "33333333-3333-4333-8333-333333333333" || adapter.recovery.TrialID != adapter.checkpoint.TrialID {
		t.Fatalf("recovery adapter routing changed: %+v", adapter.recovery)
	}
}

func TestCheckpointObservationDistinguishesGuestMilestonesWithoutSecrets(t *testing.T) {
	service, controllerPriv, _, now := serviceFixture(t)
	service.Harness = &fakeHarnessAdapter{}
	var observation bytes.Buffer
	service.Observability = &observation
	ctx := requestContext(service, protocol.FirstRequestSequence, "8")
	payload := map[string]any{
		"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1,
		"boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64),
		"harness_sha256": strings.Repeat("a", 64),
	}
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "checkpoint/v1", payload)); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(observation.String()), "\n")
	wantStages := []string{checkpointStageRequestReceived, checkpointStagePendingAccepted, checkpointStageHarnessEmitted}
	if len(lines) != len(wantStages) {
		t.Fatalf("checkpoint observations = %q, want %d stages", observation.String(), len(wantStages))
	}
	for index, line := range lines {
		data, ok := strings.CutPrefix(line, checkpointObservationPrefix)
		if !ok {
			t.Fatalf("checkpoint observation %d lacks exact prefix: %q", index, line)
		}
		var got checkpointObservation
		decoder := json.NewDecoder(strings.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if err := protocol.RequireCanonical([]byte(data)); err != nil {
			t.Fatalf("checkpoint observation %d is not canonical: %v", index, err)
		}
		if got.Schema != checkpointObservationSchema || got.Stage != wantStages[index] || got.RunID != ctx.RunID || got.CohortID != "cohort-001" || got.TrialID != "after-stage-before-commit-1" || got.BootID != ctx.BootID {
			t.Fatalf("checkpoint observation %d changed: %+v", index, got)
		}
		if index == 0 && (got.TicketSHA256 != "" || got.EvidenceSHA256 != "") {
			t.Fatalf("request receipt claims later evidence: %+v", got)
		}
		if index == 1 && (got.TicketSHA256 != strings.Repeat("e", 64) || got.EvidenceSHA256 != "") {
			t.Fatalf("pending-ticket observation changed: %+v", got)
		}
		if index == 2 && (got.TicketSHA256 != strings.Repeat("e", 64) || got.EvidenceSHA256 != strings.Repeat("f", 64)) {
			t.Fatalf("harness-evidence observation changed: %+v", got)
		}
	}
	if strings.Contains(observation.String(), strings.Repeat("d", 64)) {
		t.Fatal("checkpoint observations must not emit the private ticket nonce")
	}
}

func TestCheckpointObservationFailureStopsBeforeHarness(t *testing.T) {
	service, controllerPriv, _, now := serviceFixture(t)
	adapter := &fakeHarnessAdapter{}
	service.Harness = adapter
	service.Observability = failingObservationWriter{}
	ctx := requestContext(service, protocol.FirstRequestSequence, "9")
	payload := map[string]any{
		"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1,
		"boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64),
		"harness_sha256": strings.Repeat("a", 64),
	}
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "checkpoint/v1", payload)); err == nil {
		t.Fatal("expected checkpoint observation failure to stop the request")
	}
	if adapter.checkpoint.TrialID != "" {
		t.Fatalf("harness ran after observation failure: %+v", adapter.checkpoint)
	}
}

func TestCheckpointObservationRequiresSinkBeforeHarness(t *testing.T) {
	service, controllerPriv, _, now := serviceFixture(t)
	adapter := &fakeHarnessAdapter{}
	service.Harness = adapter
	service.Observability = nil
	ctx := requestContext(service, protocol.FirstRequestSequence, "a")
	payload := map[string]any{
		"cohort_id": "cohort-001", "trial_id": "after-stage-before-commit-1", "attempt": 1,
		"boundary": "after-stage-before-commit", "ticket_nonce": strings.Repeat("d", 64),
		"harness_sha256": strings.Repeat("a", 64),
	}
	if _, err := service.Handle(request(t, controllerPriv, now, ctx, "checkpoint/v1", payload)); err == nil {
		t.Fatal("expected missing checkpoint observation sink to stop the request")
	}
	if adapter.checkpoint.TrialID != "" {
		t.Fatalf("harness ran without a checkpoint observation sink: %+v", adapter.checkpoint)
	}
}
