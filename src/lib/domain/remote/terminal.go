package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const (
	TerminalVersion           = "dockpipe.remote.terminal/v1"
	MaxTerminalSessions       = 8
	MaxTerminalBrokerSessions = 96
	MaxTerminalInput          = 64 * 1024
	MaxTerminalOutput         = 256 * 1024
	MaxTerminalChunk          = 32 * 1024
	MaxTerminalBody           = 1 << 20
)

// The separate ephemeral terminal budget is 32 MiB, in addition to the feature
// queue's 32 MiB. Ninety-six reservations cover a 256 KiB ring, 64 KiB pending
// input and bounded metadata each. No terminal content enters durable receipts.
type TerminalOpen struct {
	Schema    string        `json:"schema"`
	ID        string        `json:"id"`
	Target    FeatureTarget `json:"target"`
	Epoch     string        `json:"epoch"`
	Root      string        `json:"root"`
	Container string        `json:"container,omitempty"`
	Shell     string        `json:"shell,omitempty"`
	Nonce     string        `json:"nonce"`
	Deadline  time.Time     `json:"deadline"`
	Columns   int           `json:"columns"`
	Rows      int           `json:"rows"`
}

func (request TerminalOpen) Digest() (string, error) {
	request.ID = ""
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func (request TerminalOpen) Validate() error {
	digest, err := request.Digest()
	if err != nil || request.Schema != TerminalVersion || request.ID != digest || !featureDigest(request.Epoch) ||
		!featureDigest(request.Nonce) || request.Target.Validate() != nil || request.Deadline.IsZero() ||
		!ValidTerminalSize(request.Columns, request.Rows) {
		return errors.New("invalid terminal open intent")
	}
	if request.Container == "" {
		if !ValidID(request.Root) || request.Shell != "" {
			return errors.New("terminal shell requires an owner root")
		}
	} else if request.Root != "" || !featureDigest(request.Container) || request.Shell != "/bin/sh" && request.Shell != "/bin/bash" {
		return errors.New("container terminal requires an exact container ID and supported shell")
	}
	return nil
}

func ValidTerminalSize(columns, rows int) bool {
	return columns >= 1 && columns <= 1000 && rows >= 1 && rows <= 1000
}

type TerminalReference struct {
	Schema string        `json:"schema"`
	ID     string        `json:"id"`
	Target FeatureTarget `json:"target"`
	Epoch  string        `json:"epoch"`
}

func (reference TerminalReference) Validate() error {
	if reference.Schema != TerminalVersion || !featureDigest(reference.ID) || !featureDigest(reference.Epoch) || reference.Target.Validate() != nil {
		return errors.New("invalid terminal reference")
	}
	return nil
}

func (request TerminalOpen) Reference() TerminalReference {
	return TerminalReference{Schema: TerminalVersion, ID: request.ID, Target: request.Target, Epoch: request.Epoch}
}

type TerminalInput struct {
	Reference TerminalReference `json:"reference"`
	Sequence  uint64            `json:"sequence"`
	Data      []byte            `json:"data"`
}

func (input TerminalInput) Validate() error {
	if input.Reference.Validate() != nil || input.Sequence == 0 || len(input.Data) == 0 || len(input.Data) > MaxTerminalInput {
		return errors.New("invalid terminal input")
	}
	return nil
}

func TerminalInputDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

type TerminalResize struct {
	Reference TerminalReference `json:"reference"`
	Columns   int               `json:"columns"`
	Rows      int               `json:"rows"`
}

type TerminalRead struct {
	Reference TerminalReference `json:"reference"`
	Cursor    uint64            `json:"cursor"`
}

type TerminalView struct {
	Reference   TerminalReference `json:"reference"`
	State       string            `json:"state"`
	AckSequence uint64            `json:"ack_sequence"`
	Cursor      uint64            `json:"cursor"`
	End         uint64            `json:"end"`
	Oldest      uint64            `json:"oldest"`
	Data        []byte            `json:"data,omitempty"`
	Truncated   bool              `json:"truncated"`
	Reason      string            `json:"reason,omitempty"`
}

type TerminalReport struct {
	Reference   TerminalReference `json:"reference"`
	State       string            `json:"state"`
	AckSequence uint64            `json:"ack_sequence"`
	AckDigest   string            `json:"ack_digest,omitempty"`
	OutputStart uint64            `json:"output_start"`
	Data        []byte            `json:"data,omitempty"`
	Truncated   bool              `json:"truncated"`
	Reason      string            `json:"reason,omitempty"`
}

func (report TerminalReport) Validate() error {
	if report.Reference.Validate() != nil || len(report.Data) > MaxTerminalChunk ||
		(report.AckSequence == 0) != (report.AckDigest == "") || report.AckDigest != "" && !featureDigest(report.AckDigest) {
		return errors.New("invalid terminal worker report")
	}
	if report.State != "running" && report.State != "closed" && report.State != "unknown" {
		return errors.New("invalid terminal worker state")
	}
	switch report.Reason {
	case "", "exited", "closed", "authority-lost", "input-uncertain", "launch-failed", "cleanup-unverified":
		return nil
	default:
		return errors.New("invalid terminal worker reason")
	}
}

type TerminalExchange struct {
	Schema  string           `json:"schema"`
	Epoch   string           `json:"epoch,omitempty"`
	Catalog FeatureCatalog   `json:"catalog"`
	Reports []TerminalReport `json:"reports"`
}

type TerminalDirective struct {
	Open        TerminalOpen   `json:"open"`
	Operator    string         `json:"operator"`
	LeaseMillis int64          `json:"lease_millis"`
	Close       bool           `json:"close"`
	Input       *TerminalInput `json:"input,omitempty"`
	Columns     int            `json:"columns"`
	Rows        int            `json:"rows"`
	OutputAck   uint64         `json:"output_ack"`
	Forget      bool           `json:"forget"`
}

type TerminalExchangeResult struct {
	Schema     string              `json:"schema"`
	Epoch      string              `json:"epoch"`
	Directives []TerminalDirective `json:"directives"`
	Forget     []TerminalReference `json:"forget,omitempty"`
}

// ContainerTerminalReference is recovery routing only, never replayable input.
type ContainerTerminalReference struct {
	Reference TerminalReference `json:"reference"`
	Container string            `json:"container"`
	Phase     string            `json:"phase"`
}

const HostTerminalReceiptVersion = "dockpipe.host-terminal-receipt/v1"

type HostTerminalReference struct {
	Reference TerminalReference `json:"reference"`
	Root      string            `json:"root"`
	Phase     string            `json:"phase"`
}

type HostTerminalRecovery struct {
	Schema     string            `json:"schema"`
	Reference  TerminalReference `json:"reference"`
	Root       string            `json:"root"`
	Reconciled bool              `json:"reconciled"`
}

func (receipt HostTerminalRecovery) Validate() error {
	if receipt.Schema != HostTerminalReceiptVersion || receipt.Reference.Validate() != nil || !ValidID(receipt.Root) || !receipt.Reconciled {
		return errors.New("invalid host terminal recovery proof")
	}
	return nil
}
