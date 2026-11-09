package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const FeatureVersion = "dockpipe.remote.features/v1"

// Files use the 1 MiB editor contract; base64 plus bounded metadata fits 2 MiB.
// Results share the existing 20 MiB transport bound (including JSON escaping).
// Durable metadata uses the broker's existing 32 MiB storage budget separately.
const MaxFeaturePayload = 2 << 20
const MaxFeatureStorage = 32 << 20
const FeatureLease = 10 * time.Second
const FeatureLifetime = 2 * time.Minute
const FeatureCatalogFreshness = 10 * time.Second

type FeatureTarget struct {
	Node        string `json:"node"`
	Fingerprint string `json:"fingerprint"`
	Generation  string `json:"generation"`
}

func (target FeatureTarget) Validate() error {
	if !ValidID(target.Node) || !featureDigest(target.Fingerprint) || !featureDigest(target.Generation) {
		return errors.New("invalid feature machine identity")
	}
	return nil
}

type FeatureRequest struct {
	Schema   string          `json:"schema"`
	ID       string          `json:"id"`
	Target   FeatureTarget   `json:"target"`
	Root     string          `json:"root,omitempty"`
	Action   string          `json:"action"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Nonce    string          `json:"nonce,omitempty"`
	Deadline time.Time       `json:"deadline"`
}

// Digest is also the operation ID. Reclamation after Deadline cannot permit a
// changed payload or a renewed deadline to reuse the same operation identity.
func (request FeatureRequest) Digest() (string, error) {
	request.ID = ""
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (request FeatureRequest) Validate() error {
	if request.Schema != FeatureVersion || request.Target.Validate() != nil || !featureDigest(request.Nonce) ||
		len(request.Payload) == 0 || len(request.Payload) > MaxFeaturePayload || !json.Valid(request.Payload) || request.Deadline.IsZero() {
		return errors.New("invalid remote feature request")
	}
	if FeatureAccess(request.Action) == "" {
		return errors.New("unsupported remote feature action")
	}
	if FeatureUsesRoot(request.Action) {
		if !ValidID(request.Root) {
			return errors.New("feature root identifier required")
		}
	} else if request.Root != "" {
		return errors.New("this feature action does not select a root")
	}
	digest, err := request.Digest()
	if err != nil || request.ID != digest {
		return errors.New("feature ID must bind the exact request digest")
	}
	return nil
}

// FeatureAccess is an operation allowlist, not an arbitrary command bridge.
func FeatureAccess(action string) string {
	switch action {
	case "terminal.host-recover":
		return "write"
	case "files.list", "files.read", "git.status", "git.diff", "git.review-status", "git.compare", "git.catalog", "git.history", "git.commit-review", "git.stash-list", "containers.snapshot", "containers.inspect", "containers.logs":
		return "read"
	case "files.write", "files.create", "files.rename", "files.delete", "files.mkdir",
		"git.commit", "git.pull", "git.stage", "git.unstage", "git.discard", "git.commit-index", "git.branch-create", "git.branch-switch", "git.branch-delete", "git.fetch", "git.merge", "git.stash-save", "git.stash-apply", "git.stash-pop", "git.stash-drop", "git.revert", "git.cherry-pick", "git.operation-continue", "git.operation-skip", "git.operation-abort", "git.resolve-stage", "git.resolve-ours", "git.resolve-theirs", "git.resolve-delete", "containers.start", "containers.stop", "containers.restart", "containers.terminal-recover":
		return "write"
	case "git.push", "git.tag-publish":
		return "publish"
	default:
		return ""
	}
}

func FeatureUsesRoot(action string) bool {
	return strings.HasPrefix(action, "files.") || strings.HasPrefix(action, "git.") || action == "terminal.host-recover"
}

type FeatureRoot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FilesRead  bool   `json:"files_read"`
	FilesWrite bool   `json:"files_write"`
	GitRead    bool   `json:"git_read"`
	GitWrite   bool   `json:"git_write"`
	GitPublish bool   `json:"git_publish"`
	Terminal   bool   `json:"terminal,omitempty"`
}

type FeatureCatalog struct {
	Schema             string                       `json:"schema"`
	Target             FeatureTarget                `json:"target"`
	Roots              []FeatureRoot                `json:"roots"`
	DockerRead         bool                         `json:"docker_read"`
	DockerControl      bool                         `json:"docker_control"`
	DockerExec         bool                         `json:"docker_exec,omitempty"`
	DockerRecovery     bool                         `json:"docker_recovery,omitempty"`
	ReceivedAt         time.Time                    `json:"received_at"`
	TerminalEpoch      string                       `json:"terminal_epoch,omitempty"`
	ProcessRecovery    string                       `json:"process_recovery,omitempty"`
	ContainerTerminals []ContainerTerminalReference `json:"container_terminals,omitempty"`
	HostTerminals      []HostTerminalReference      `json:"host_terminals,omitempty"`
}

func (catalog FeatureCatalog) Validate() error {
	if catalog.ProcessRecovery != "" && catalog.ProcessRecovery != "cgroup-v2" && catalog.ProcessRecovery != "host-reboot" && catalog.ProcessRecovery != "unavailable" {
		return errors.New("invalid worker process recovery mode")
	}
	if catalog.Schema != FeatureVersion || catalog.Target.Validate() != nil || len(catalog.Roots) > 32 || (catalog.DockerExec || catalog.DockerRecovery) && !catalog.DockerRead {
		return errors.New("invalid feature catalog")
	}
	if len(catalog.ContainerTerminals)+len(catalog.HostTerminals) > MaxTerminalSessions {
		return errors.New("container terminal recovery catalog exceeds its session budget")
	}
	for _, receipt := range catalog.HostTerminals {
		if receipt.Reference.Validate() != nil || !ValidID(receipt.Root) || receipt.Reference.Target.Node != catalog.Target.Node || receipt.Reference.Target.Fingerprint != catalog.Target.Fingerprint ||
			receipt.Phase != "planned" && receipt.Phase != "start-requested" && receipt.Phase != "running" {
			return errors.New("host terminal recovery catalog changed identity or phase")
		}
	}
	for _, receipt := range catalog.ContainerTerminals {
		if receipt.Reference.Validate() != nil || !featureDigest(receipt.Container) || receipt.Reference.Target.Node != catalog.Target.Node || receipt.Reference.Target.Fingerprint != catalog.Target.Fingerprint {
			return errors.New("container terminal recovery catalog changed machine identity")
		}
	}
	seen := map[string]bool{}
	terminalRoots := 0
	for _, root := range catalog.Roots {
		if root.Terminal {
			terminalRoots++
		}
		if !ValidID(root.ID) || seen[root.ID] || len(root.Name) == 0 || len(root.Name) > 256 || strings.ContainsAny(root.Name, "\x00\r\n") {
			return errors.New("invalid feature root catalog")
		}
		seen[root.ID] = true
	}
	if terminalRoots > 1 || catalog.TerminalEpoch != "" && !featureDigest(catalog.TerminalEpoch) {
		return errors.New("invalid terminal catalog")
	}
	return nil
}

// Worker publication omits the broker epoch. Only the broker can advertise an
// actionable terminal to an operator, after the terminal worker has connected.
func (catalog FeatureCatalog) ValidateOperatorCatalog() error {
	if err := catalog.Validate(); err != nil {
		return err
	}
	if catalog.DockerExec && !featureDigest(catalog.TerminalEpoch) {
		return errors.New("container terminal catalog is missing its live broker epoch")
	}
	for _, root := range catalog.Roots {
		if root.Terminal && !featureDigest(catalog.TerminalEpoch) {
			return errors.New("terminal catalog is missing its live broker epoch")
		}
	}
	return nil
}

type FeatureReference struct {
	Schema string        `json:"schema"`
	ID     string        `json:"id"`
	Target FeatureTarget `json:"target"`
}

func (reference FeatureReference) Validate() error {
	if reference.Schema != FeatureVersion || !featureDigest(reference.ID) || reference.Target.Validate() != nil {
		return errors.New("invalid remote feature reference")
	}
	return nil
}

func (request FeatureRequest) Reference() FeatureReference {
	return FeatureReference{Schema: FeatureVersion, ID: request.ID, Target: request.Target}
}

type FeatureResult struct {
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (result FeatureResult) Validate() error {
	switch result.Status {
	case "success", "failure", "cancelled", "unknown":
	default:
		return errors.New("invalid feature result status")
	}
	if len(result.Payload) > MaxBody-4096 || len(result.Error) > 1024 || len(result.Payload) > 0 && !json.Valid(result.Payload) {
		return errors.New("feature result exceeds its transport contract")
	}
	return nil
}

type FeatureOperation struct {
	// ReceiptOnly responses contain routing metadata, not a replayable Request.
	// Input and result bodies are intentionally unavailable after restart.
	Request           FeatureRequest `json:"request"`
	Status            string         `json:"status"`
	Result            *FeatureResult `json:"result,omitempty"`
	CancelRequested   bool           `json:"cancel_requested"`
	ReceiptOnly       bool           `json:"receipt_only,omitempty"`
	ResultDigest      string         `json:"result_digest,omitempty"`
	RecoveryRequested bool           `json:"recovery_requested,omitempty"`
	// Reconciled acknowledges stopped execution, never a successful mutation.
	Reconciled       bool                  `json:"reconciled,omitempty"`
	RecoveryChoice   string                `json:"recovery_choice,omitempty"`
	RecoveryResource string                `json:"recovery_resource,omitempty"`
	RecoveryError    string                `json:"recovery_error,omitempty"`
	HostRecovery     *HostTerminalRecovery `json:"host_recovery,omitempty"`
}

type FeatureAssignment struct {
	Request          FeatureRequest `json:"request"`
	LeaseMillis      int64          `json:"lease_millis"`
	Operator         string         `json:"operator"`
	Reconcile        bool           `json:"reconcile,omitempty"`
	RecoveryChoice   string         `json:"recovery_choice,omitempty"`
	RecoveryResource string         `json:"recovery_resource,omitempty"`
}

type FeaturePulse struct {
	LeaseMillis int64 `json:"lease_millis"`
	Cancel      bool  `json:"cancel"`
}

type FeatureCompletion struct {
	Reference        FeatureReference      `json:"reference"`
	Result           FeatureResult         `json:"result"`
	ReceiptOnly      bool                  `json:"receipt_only,omitempty"`
	ResultDigest     string                `json:"result_digest,omitempty"`
	Deadline         time.Time             `json:"deadline"`
	Reconciled       bool                  `json:"reconciled,omitempty"`
	RecoveryResource string                `json:"recovery_resource,omitempty"`
	HostRecovery     *HostTerminalRecovery `json:"host_recovery,omitempty"`
}

// FeatureReconciliation is emitted only by an idle, exclusively locked worker.
// It binds the old operation and the current authenticated worker lifetime.
type FeatureReconciliation struct {
	Reference FeatureReference `json:"reference"`
	Worker    FeatureTarget    `json:"worker"`
	Resource  string           `json:"resource,omitempty"`
}

type FeatureRecoveryRequest struct {
	FeatureReference
	Choice   string `json:"choice,omitempty"`
	Resource string `json:"resource,omitempty"`
}

type FeatureResultAck struct {
	Reference FeatureReference `json:"reference"`
	Expired   bool             `json:"expired"`
}

func featureDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
