package remotecmd

import (
	"bytes"
	"encoding/json"
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
