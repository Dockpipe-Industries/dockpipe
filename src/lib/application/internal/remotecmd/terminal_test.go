package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestTerminalPrivateNDJSONRejectsAmbiguousOrGenericCommands(t *testing.T) {
	valid := []string{`{"type":"input","sequence":1,"data":"aGk="}`, `{"type":"resize","columns":80,"rows":24}`, `{"type":"close"}`}
	for _, raw := range valid {
		if _, err := decodeTerminalCommand([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
	invalid := []string{
		`{"type":"exec","command":"sh"}`, `{"type":"input","sequence":0,"data":"aGk="}`,
		`{"type":"input","type":"close","sequence":1,"data":"aGk="}`, `{"type":"resize","columns":1001,"rows":24}`,
		`{"type":"close","data":"aGk="}`, `{"type":"input","sequence":1,"data":"aGk=","env":{}}`,
	}
	for _, raw := range invalid {
		if _, err := decodeTerminalCommand([]byte(raw)); err == nil {
			t.Fatal("ambiguous terminal command accepted")
		}
	}
	oversized, _ := json.Marshal(terminalCommand{Type: "input", Sequence: 1, Data: bytes.Repeat([]byte("x"), contract.MaxTerminalInput+1)})
	if _, err := decodeTerminalCommand(oversized); err == nil {
		t.Fatal("oversized terminal input accepted")
	}
}

func TestTerminalIntentCanonicalizesOnlyExistingNonce(t *testing.T) {
	now := time.Now().UTC()
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Root: "owner", Nonce: strings.Repeat("d", 64), Deadline: now.Add(time.Minute), Columns: 80, Rows: 24}
	raw, _ := json.Marshal(request)
	first, err := decodeTerminalIntent(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := decodeTerminalIntent(raw, now)
	if err != nil || first.ID != second.ID || first.Nonce != request.Nonce {
		t.Fatal("decode created a distinct launch")
	}
	request.Nonce = ""
	raw, _ = json.Marshal(request)
	if _, err := decodeTerminalIntent(raw, now); err == nil {
		t.Fatal("CLI invented a launch nonce")
	}
}

func TestContainerTerminalPreparationMustBeAcknowledgedBeforeRemoteOpen(t *testing.T) {
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Nonce: strings.Repeat("d", 64), Deadline: time.Now().Add(time.Second), Container: strings.Repeat("e", 64), Shell: "/bin/sh", Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	reference := request.Reference()
	wrong := reference
	wrong.ID = strings.Repeat("f", 64)
	for _, command := range []terminalCommand{
		{Type: "prepared-ack", Reference: &wrong},
		{Type: "input", Sequence: 1, Data: []byte("never execute")},
	} {
		data, _ := json.Marshal(command)
		output := &terminalPreparationOutput{}
		// A nil client deliberately proves that invalid acknowledgments cannot
		// reach remote open. No socket or authenticated request is constructed.
		err := streamTerminal(context.Background(), nil, request, io.NopCloser(bytes.NewReader(append(data, '\n'))), output)
		if err == nil {
			t.Fatal("unacknowledged container open reached effects")
		}
		var event terminalEvent
		if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event) != nil || event.Type != "prepared" || event.Reference == nil || *event.Reference != reference {
			t.Fatal("canonical original reference was not prepared")
		}
	}
	valid, _ := json.Marshal(terminalCommand{Type: "prepared-ack", Reference: &reference})
	if _, err := decodeTerminalCommand(valid); err != nil {
		t.Fatal(err)
	}
}

func TestHostTerminalOptInPreparationPreventsOpenWithoutExactAcknowledgment(t *testing.T) {
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Root: "project", Nonce: strings.Repeat("d", 64), Deadline: time.Now().Add(time.Second), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	reference := request.Reference()
	wrong := reference
	wrong.Target.Generation = strings.Repeat("f", 64)
	bad, _ := json.Marshal(terminalCommand{Type: "prepared-ack", Reference: &wrong})
	for _, data := range [][]byte{nil, append(bad, '\n')} {
		output := &terminalPreparationOutput{}
		// A nil client proves no authenticated remote-open call is possible.
		if err := streamTerminalPrepared(context.Background(), nil, request, io.NopCloser(bytes.NewReader(data)), output, true); err == nil {
			t.Fatal("host opening reached effects without its exact prepared ACK")
		}
		var event terminalEvent
		if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event) != nil || event.Type != "prepared" || event.Reference == nil || *event.Reference != reference {
			t.Fatal("host preparation did not retain the canonical original reference")
		}
	}
	if err := Run([]string{"nodes", "--prepare"}, nil); err == nil {
		t.Fatal("terminal preparation flag broadened an unrelated command")
	}
}

type terminalPreparationOutput struct{ bytes.Buffer }

func (output *terminalPreparationOutput) Close() error { return nil }
