//go:build linux

package guest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"

	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/recovery"
)

const (
	harnessCommandSchema    = "dockpipe.sqlite-vm-harness-command.v1"
	harnessCheckpointSchema = "dockpipe.sqlite-vm-checkpoint-evidence.v1"
	harnessRecoverySchema   = "dockpipe.sqlite-vm-recovery-evidence.v1"
	harnessCheckpointRole   = "DORKPIPE_SQLITE_VM_HARNESS_ROLE=checkpoint"
	harnessRecoveryRole     = "DORKPIPE_SQLITE_VM_HARNESS_ROLE=recovery"
	harnessLookupPath       = "PATH=/usr/bin:/bin"
	harnessOutputLimit      = 32 * 1024
)

var harnessIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)

var harnessBoundaries = map[string]int64{
	"after-stage-before-commit":        1,
	"inside-commit-hook-before-phase1": 1,
	"after-commit-before-reload":       2,
	"after-validation-before-ack":      2,
}

type linuxHarnessAdapter struct {
	binaryPath string
	binarySHA  string
	root       string
	mu         sync.Mutex
	held       *exec.Cmd
}

type harnessCommand struct {
	Schema   string `json:"schema"`
	CohortID string `json:"cohort_id"`
	TrialID  string `json:"trial_id"`
	Boundary string `json:"boundary"`
	Attempt  int    `json:"attempt"`
	Root     string `json:"root"`
}

func NewLinuxHarnessAdapter(binaryPath, binarySHA, root string) (HarnessAdapter, error) {
	adapter := &linuxHarnessAdapter{binaryPath: binaryPath, binarySHA: binarySHA, root: root}
	if !filepath.IsAbs(binaryPath) || !filepath.IsAbs(root) || filepath.Clean(root) != root || root != "/var/lib/dockpipe-qualification" || !shaPattern.MatchString(binarySHA) {
		return nil, fmt.Errorf("qualification harness paths or hash are invalid")
	}
	if err := adapter.requireBinary(); err != nil {
		return nil, err
	}
	for _, directory := range []string{filepath.Join(root, "cohorts"), filepath.Join(root, "tickets"), filepath.Join(root, "results")} {
		if err := requirePrivateHarnessDirectory(directory); err != nil {
			return nil, err
		}
	}
	return adapter, nil
}

func (a *linuxHarnessAdapter) Checkpoint(request HarnessRequest) (any, error) {
	command, identity, err := a.validateRequest(request, request.BootID)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.held != nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("one checkpoint harness is already active")
	}
	a.mu.Unlock()
	state, err := recovery.New(recovery.FileStore{Root: filepath.Join(a.root, "tickets")}, identity)
	if err != nil {
		return nil, err
	}
	ticket := recovery.Ticket{Identity: identity, Status: "pending"}
	if err := state.AcceptPending(ticket); err != nil {
		return nil, err
	}
	ticketJSON, _ := json.Marshal(ticket)
	ticketSHA256 := hashHarnessBytes(ticketJSON)
	if request.observeCheckpoint != nil {
		if err := request.observeCheckpoint(checkpointStagePendingAccepted, ticketSHA256, ""); err != nil {
			return nil, err
		}
	}
	evidence, process, wait, err := a.runHarness(command, harnessCheckpointRole, true)
	if err != nil {
		return nil, err
	}
	select {
	case err := <-wait:
		return nil, fmt.Errorf("checkpoint harness exited instead of holding: %w", err)
	default:
	}
	a.mu.Lock()
	a.held = process
	a.mu.Unlock()
	evidenceSHA256 := hashHarnessBytes(evidence)
	if request.observeCheckpoint != nil {
		if err := request.observeCheckpoint(checkpointStageHarnessEmitted, ticketSHA256, evidenceSHA256); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"ticket_sha256": ticketSHA256, "checkpoint_boot_id": request.BootID,
		"harness_sha256": a.binarySHA, "evidence": json.RawMessage(evidence),
	}, nil
}

func (a *linuxHarnessAdapter) Recovery(request HarnessRequest) (any, error) {
	if request.CheckpointBootID == "" || request.CheckpointBootID == request.BootID {
		return nil, fmt.Errorf("recovery requires the distinct authenticated checkpoint boot ID")
	}
	command, identity, err := a.validateRequest(request, request.CheckpointBootID)
	if err != nil {
		return nil, err
	}
	state, err := recovery.New(recovery.FileStore{Root: filepath.Join(a.root, "tickets")}, identity)
	if err != nil {
		return nil, err
	}
	ticket, exists, err := (recovery.FileStore{Root: filepath.Join(a.root, "tickets")}).Load(request.TrialID)
	if err != nil || !exists || ticket.Status != "pending" || ticket.Identity != identity {
		return nil, fmt.Errorf("exact pending recovery ticket is unavailable")
	}
	ticketJSON, _ := json.Marshal(ticket)
	evidence, _, _, err := a.runHarness(command, harnessRecoveryRole, false)
	if err != nil {
		return nil, err
	}
	evidenceSHA := hashHarnessBytes(evidence)
	resultPath := filepath.Join(a.root, "results", request.TrialID+".json")
	if err := durableHarnessFile(resultPath, evidence); err != nil {
		return nil, err
	}
	outcome := "old"
	if harnessBoundaries[request.Boundary] == 2 {
		outcome = "new"
	}
	if _, err := state.ConsumeRecovery(recovery.Result{Identity: identity, Outcome: outcome, Evidence: evidenceSHA}); err != nil {
		return nil, err
	}
	return map[string]any{
		"ticket_sha256": hashHarnessBytes(ticketJSON), "checkpoint_boot_id": request.CheckpointBootID,
		"recovery_boot_id": request.BootID, "harness_sha256": a.binarySHA,
		"outcome": outcome, "evidence_sha256": evidenceSHA, "evidence": json.RawMessage(evidence),
	}, nil
}

func (a *linuxHarnessAdapter) validateRequest(request HarnessRequest, ticketBootID string) (harnessCommand, recovery.Identity, error) {
	var command harnessCommand
	if err := a.requireBinary(); err != nil {
		return command, recovery.Identity{}, err
	}
	if !harnessIDPattern.MatchString(request.RunID) || !harnessIDPattern.MatchString(request.CohortID) || !harnessIDPattern.MatchString(request.TrialID) || request.Attempt < 1 || request.Attempt > 3 || !shaPattern.MatchString(request.TicketNonce) || request.HarnessSHA256 != a.binarySHA {
		return command, recovery.Identity{}, fmt.Errorf("harness request identity or pin is invalid")
	}
	if _, ok := harnessBoundaries[request.Boundary]; !ok {
		return command, recovery.Identity{}, fmt.Errorf("durability boundary is not reviewed")
	}
	wantTrial := fmt.Sprintf("%s-%d", request.Boundary, request.Attempt)
	if request.TrialID != wantTrial {
		return command, recovery.Identity{}, fmt.Errorf("trial ID does not match boundary and attempt")
	}
	root := filepath.Join(a.root, "cohorts", request.CohortID, request.Boundary, fmt.Sprintf("attempt-%d", request.Attempt))
	command = harnessCommand{Schema: harnessCommandSchema, CohortID: request.CohortID, TrialID: request.TrialID, Boundary: request.Boundary, Attempt: request.Attempt, Root: root}
	identity := recovery.Identity{
		MachineUUID: request.MachineUUID, DiskSerial: request.DiskSerial, BootID: ticketBootID,
		RunID: request.RunID, CohortID: request.CohortID, TrialID: request.TrialID,
		Scenario: request.Scenario, DurabilityBoundary: request.Boundary,
		Nonce: request.TicketNonce, HarnessSHA256: request.HarnessSHA256,
	}
	return command, identity, nil
}

func (a *linuxHarnessAdapter) runHarness(command harnessCommand, role string, hold bool) ([]byte, *exec.Cmd, <-chan error, error) {
	input, err := json.Marshal(command)
	if err != nil {
		return nil, nil, nil, err
	}
	process := exec.Command(a.binaryPath)
	process.Env = []string{role, harnessLookupPath}
	process.Dir = filepath.Dir(a.binaryPath)
	process.Stdin = bytes.NewReader(input)
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	var stderr bytes.Buffer
	process.Stderr = &stderr
	if err := process.Start(); err != nil {
		return nil, nil, nil, err
	}
	wait := make(chan error, 1)
	go func() { wait <- process.Wait() }()
	decoder := json.NewDecoder(io.LimitReader(stdout, harnessOutputLimit+1))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		_ = process.Process.Kill()
		<-wait
		return nil, nil, nil, fmt.Errorf("decode pinned harness evidence: %w; stderr=%s", err, boundedHarnessText(stderr.String()))
	}
	if len(raw) == 0 || len(raw) > harnessOutputLimit {
		_ = process.Process.Kill()
		<-wait
		return nil, nil, nil, fmt.Errorf("pinned harness evidence exceeds bounds")
	}
	canonical, err := protocol.Canonicalize(raw)
	if err != nil {
		_ = process.Process.Kill()
		<-wait
		return nil, nil, nil, fmt.Errorf("pinned harness evidence is not strict canonical JSON: %w", err)
	}
	if err := validateHarnessEvidence(canonical, command, role); err != nil {
		_ = process.Process.Kill()
		<-wait
		return nil, nil, nil, err
	}
	if !hold {
		if err := <-wait; err != nil {
			return nil, nil, nil, fmt.Errorf("recovery harness failed: %w; stderr=%s", err, boundedHarnessText(stderr.String()))
		}
	}
	return canonical, process, wait, nil
}

func validateHarnessEvidence(data []byte, command harnessCommand, role string) error {
	var evidence struct {
		Schema               string `json:"schema"`
		CohortID             string `json:"cohort_id"`
		TrialID              string `json:"trial_id"`
		Boundary             string `json:"boundary"`
		Attempt              int    `json:"attempt"`
		Root                 string `json:"root"`
		RootIdentity         string `json:"root_identity"`
		Database             string `json:"database"`
		ExpectedRevision     int64  `json:"expected_revision"`
		ObservedRevision     int64  `json:"observed_revision,omitempty"`
		ObservedDigest       string `json:"observed_digest,omitempty"`
		PreMetadataSHA256    string `json:"pre_metadata_sha256"`
		PostMetadataSHA256   string `json:"post_metadata_sha256,omitempty"`
		CompileOptionsSHA256 string `json:"compile_options_sha256"`
		SQLiteVersion        string `json:"sqlite_version"`
		SQLiteSourceID       string `json:"sqlite_source_id"`
		VFS                  string `json:"vfs"`
		QuickCheck           string `json:"quick_check,omitempty"`
		Retries              int    `json:"retries"`
		Replays              int    `json:"replays"`
		Repairs              int    `json:"repairs"`
		Fallbacks            int    `json:"fallbacks"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return fmt.Errorf("decode pinned harness evidence contract: %w", err)
	}
	wantSchema := harnessCheckpointSchema
	if role == harnessRecoveryRole {
		wantSchema = harnessRecoverySchema
	}
	wantRevision := harnessBoundaries[command.Boundary]
	if evidence.Schema != wantSchema || evidence.CohortID != command.CohortID || evidence.TrialID != command.TrialID || evidence.Boundary != command.Boundary || evidence.Attempt != command.Attempt || evidence.Root != command.Root || evidence.ExpectedRevision != wantRevision || evidence.RootIdentity == "" || evidence.Database != filepath.Join(command.Root, "sqlite", "main", "aggregate.sqlite") || !shaPattern.MatchString(evidence.PreMetadataSHA256) || !shaPattern.MatchString(evidence.CompileOptionsSHA256) || evidence.SQLiteVersion != "3.53.3" || evidence.SQLiteSourceID != "2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62" || evidence.VFS != "unix" || evidence.Retries != 0 || evidence.Replays != 0 || evidence.Repairs != 0 || evidence.Fallbacks != 0 {
		return fmt.Errorf("pinned harness evidence identity or invariant changed")
	}
	if role == harnessRecoveryRole && (evidence.ObservedRevision != wantRevision || !shaPattern.MatchString(evidence.ObservedDigest) || !shaPattern.MatchString(evidence.PostMetadataSHA256) || evidence.QuickCheck != "ok") {
		return fmt.Errorf("pinned recovery evidence is incomplete")
	}
	if role == harnessCheckpointRole && (evidence.ObservedRevision != 0 || evidence.ObservedDigest != "" || evidence.PostMetadataSHA256 != "" || evidence.QuickCheck != "") {
		return fmt.Errorf("checkpoint evidence contains a fabricated recovery result")
	}
	return nil
}

func (a *linuxHarnessAdapter) requireBinary() error {
	info, err := os.Lstat(a.binaryPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o755 {
		return fmt.Errorf("qualification harness must be the exact regular mode-0755 binary")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != 0 || stat.Gid != 0 {
		return fmt.Errorf("qualification harness must be root-owned")
	}
	data, err := os.ReadFile(a.binaryPath)
	if err != nil || hashHarnessBytes(data) != a.binarySHA {
		return fmt.Errorf("qualification harness SHA-256 mismatch")
	}
	return nil
}

func requirePrivateHarnessDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("qualification harness directory %s must be a private directory", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() || int(stat.Gid) != os.Getegid() {
		return fmt.Errorf("qualification harness directory %s ownership mismatch", path)
	}
	return nil
}

func durableHarnessFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func hashHarnessBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func boundedHarnessText(value string) string {
	if len(value) > 1024 {
		return value[:1024]
	}
	return value
}
