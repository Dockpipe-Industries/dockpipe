package remote

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validDesktopOpen() DesktopOpen {
	request := DesktopOpen{Schema: DesktopVersion, Target: validTerminalIntent().Target, Epoch: strings.Repeat("c", 64),
		Provider: DesktopProvider{ID: "fixture.native", Version: "1", Artifact: strings.Repeat("e", 64)}, Mode: "control", Nonce: strings.Repeat("d", 64), Deadline: time.Now().UTC().Add(time.Minute)}
	request.ID, _ = request.Digest()
	return request
}

func TestDesktopExactIntentAndIndependentScopes(t *testing.T) {
	request := validDesktopOpen()
	if request.Validate() != nil {
		t.Fatal("valid desktop intent rejected")
	}
	for _, change := range []func(*DesktopOpen){
		func(value *DesktopOpen) { value.Target.Generation = strings.Repeat("f", 64) },
		func(value *DesktopOpen) { value.Target.Fingerprint = strings.Repeat("f", 64) },
		func(value *DesktopOpen) { value.Epoch = strings.Repeat("f", 64) },
		func(value *DesktopOpen) { value.Provider.Artifact = strings.Repeat("f", 64) },
		func(value *DesktopOpen) { value.Mode = "view" },
		func(value *DesktopOpen) { value.Deadline = value.Deadline.Add(time.Second) },
	} {
		changed := request
		change(&changed)
		if changed.Validate() == nil {
			t.Fatal("changed desktop selection retained its previous identity")
		}
	}
	if OperatorScope("/v1/desktop-open") != "remote.desktop.session" || OperatorScope("/v1/desktop-status") != "remote.desktop.view" ||
		OperatorScope("/v1/desktop-worker") != "" || OperatorScope("/v1/desktop-input") != "" || FeatureAccess("desktop.open") != "" {
		t.Fatal("desktop gained execution, worker or arbitrary-input authority")
	}
}

func TestDesktopSignalingRejectsMediaAndUnboundedParameters(t *testing.T) {
	reference := validDesktopOpen().Reference()
	for _, signal := range []DesktopSignal{
		{Reference: reference, Sequence: 1, Type: "offer", SDP: "v=0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"},
		{Reference: reference, Sequence: 1, Type: "parameters", Parameters: json.RawMessage(`{"temporary":"fixture"}`)},
		{Reference: reference, Sequence: 2, Type: "ice", Candidate: "candidate:fixture"},
		{Reference: reference, Sequence: 3, Type: "ice-complete"},
	} {
		if err := signal.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, signal := range []DesktopSignal{
		{Reference: reference, Sequence: 1, Type: "input"},
		{Reference: reference, Sequence: 1, Type: "frame"},
		{Reference: reference, Sequence: 1, Type: "clipboard"},
		{Reference: reference, Sequence: 1, Type: "offer", SDP: "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 0\r\n"},
		{Reference: reference, Sequence: 1, Type: "offer", SDP: "v=0\r\n" + strings.Repeat("a", MaxDesktopSDP)},
		{Reference: reference, Sequence: 1, Type: "parameters", Parameters: json.RawMessage(`[]`)},
		{Reference: reference, Sequence: 1, Type: "parameters", Parameters: json.RawMessage(`{"key":"` + strings.Repeat("a", MaxDesktopParameters) + `"}`)},
		{Reference: reference, Sequence: 1, Type: "ice-complete", Parameters: json.RawMessage(`{}`)},
	} {
		if signal.Validate() == nil {
			t.Fatal("unsupported or unbounded desktop content accepted", signal.Type)
		}
	}
}
