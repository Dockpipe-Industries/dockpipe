package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func testContext() Context {
	return Context{MachineUUID: "11111111-1111-4111-8111-111111111111", DiskSerial: "dockpipe-data-000001", BootID: "22222222-2222-4222-8222-222222222222", Sequence: FirstRequestSequence, RunID: "run-001", Nonce: strings.Repeat("a", 64), Scenario: "wal-checkpoint", DurabilityBoundary: "after-fsync", Phase: "checkpoint"}
}

func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestSignVerifyAndFrameRoundTrip(t *testing.T) {
	pub, priv := keypair(t)
	now := time.Unix(1_800_000_000, 0)
	data, err := Sign(RequestKind, "checkpoint/v1", testContext(), map[string]any{"boundary": "after-fsync"}, now, now.Add(time.Minute), priv)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := Verify(data, pub, now)
	if err != nil || frame.Context.RunID != "run-001" {
		t.Fatalf("verify: %+v, %v", frame, err)
	}
	var wire bytes.Buffer
	if err := WriteFramed(&wire, data); err != nil {
		t.Fatal(err)
	}
	if err := WriteFramed(&wire, data); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := ReadFramed(&wire)
	if err != nil || !bytes.Equal(data, roundTrip) {
		t.Fatalf("framed round trip failed: %v", err)
	}
	second, err := ReadFramed(&wire)
	if err != nil || !bytes.Equal(data, second) {
		t.Fatalf("second framed message was over-read: %v", err)
	}
}

func TestProtocolRejectsMalformedAndUntrustedFrames(t *testing.T) {
	pub, priv := keypair(t)
	otherPub, _ := keypair(t)
	now := time.Unix(1_800_000_000, 0)
	data, err := Sign(ResultKind, "health/v1", testContext(), map[string]any{"healthy": true}, now, now.Add(time.Minute), priv)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"unknown field": bytes.Replace(data, []byte(`"version":`), []byte(`"extra":true,"version":`), 1),
		"duplicate":     bytes.Replace(data, []byte(`"version":`), []byte(`"kind":"again","version":`), 1),
		"noncanonical":  append([]byte(" "), data...),
		"bad signature": bytes.Replace(data, []byte(`"signature":"`), []byte(`"signature":"AAAA`), 1),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Verify(candidate, pub, now); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	if _, err := Verify(data, otherPub, now); err == nil {
		t.Fatal("expected pinned-key substitution rejection")
	}
	if _, err := Verify(data, pub, now.Add(10*time.Minute)); err == nil {
		t.Fatal("expected stale frame rejection")
	}
}

func TestProtocolRejectsOversizeAndProhibitedCapability(t *testing.T) {
	_, priv := keypair(t)
	now := time.Unix(1_800_000_000, 0)
	if _, err := Sign(RequestKind, "exec/v1", testContext(), map[string]string{"command": "id"}, now, now.Add(time.Minute), priv); err == nil {
		t.Fatal("expected arbitrary exec rejection")
	}
	if _, err := Sign(RequestKind, "health/v1", testContext(), map[string]string{"data": strings.Repeat("x", MaxPayloadBytes+1)}, now, now.Add(time.Minute), priv); err == nil {
		t.Fatal("expected oversized payload rejection")
	}
	missing := testContext()
	missing.BootID = ""
	if _, err := Sign(RequestKind, "health/v1", missing, map[string]bool{"healthy": true}, now, now.Add(time.Minute), priv); err == nil {
		t.Fatal("expected missing authenticated identity rejection")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], MaxFrameBytes+1)
	if _, err := ReadFramed(bytes.NewReader(prefix[:])); err == nil {
		t.Fatal("expected oversized frame rejection")
	}
	if _, err := Canonicalize([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected duplicate key rejection")
	}
	if err := RequireCanonical([]byte(`{"b":1,"a":2}`)); err == nil {
		t.Fatal("expected noncanonical key order rejection")
	}
}

func TestGuestFirstIdentityBootstrapAuthenticatesKernelBootID(t *testing.T) {
	guestPub, guestPriv := keypair(t)
	now := time.Unix(1_800_000_000, 0)
	ctx := testContext()
	ctx.Sequence = BootstrapSequence
	ctx.Nonce = strings.Repeat("b", 64)
	ctx.Phase = BootstrapPhase
	payload := IdentityBootstrapPayload{
		BootIDSource: "/proc/sys/kernel/random/boot_id", ControllerPublicKeySHA256: strings.Repeat("c", 64),
		GuestPublicKeySHA256: func() string { sum := sha256.Sum256(guestPub); return hex.EncodeToString(sum[:]) }(), ControllerBinarySHA256: strings.Repeat("e", 64), GuestAgentBinarySHA256: strings.Repeat("f", 64),
	}
	data, err := Sign(BootstrapKind, "identity/v1", ctx, payload, now, now.Add(time.Minute), guestPriv)
	if err != nil {
		t.Fatal(err)
	}
	expected := ctx
	expected.BootID = ""
	if _, err := VerifyIdentityBootstrap(data, guestPub, now, expected, ctx.Nonce, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentityBootstrap(data, guestPub, now, expected, strings.Repeat("9", 64), payload); err == nil {
		t.Fatal("expected launch bootstrap nonce substitution rejection")
	}
	wrongPayload := payload
	wrongPayload.GuestPublicKeySHA256 = strings.Repeat("0", 64)
	if _, err := VerifyIdentityBootstrap(data, guestPub, now, expected, ctx.Nonce, wrongPayload); err == nil {
		t.Fatal("expected bootstrap pin substitution rejection")
	}
}

func TestReplayGuardRejectsReplayOrderAndIdentitySubstitution(t *testing.T) {
	ctx := testContext()
	guard := NewReplayGuard(ctx)
	frame := SignedFrame{Context: ctx}
	if err := guard.Accept(frame); err != nil {
		t.Fatal(err)
	}
	if err := guard.Accept(frame); err == nil {
		t.Fatal("expected replay rejection")
	}
	ctx.Sequence = 4
	ctx.Nonce = strings.Repeat("b", 64)
	if err := guard.Accept(SignedFrame{Context: ctx}); err == nil {
		t.Fatal("expected out-of-order rejection")
	}
	ctx.Sequence = 3
	ctx.MachineUUID = "33333333-3333-4333-8333-333333333333"
	if err := guard.Accept(SignedFrame{Context: ctx}); err == nil {
		t.Fatal("expected identity substitution rejection")
	}
}
