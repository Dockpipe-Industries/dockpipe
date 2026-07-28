package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	NodeConnectorServiceLifecycleIntentFixtureSchema = "dorkpipe.node-connector-service-lifecycle-intent-fixture/v1"
	NodeConnectorServiceLifecycleIntentSchema        = "dorkpipe.node-connector-service-lifecycle-intent/v1"
	NodeConnectorServiceDiagnosticFixtureSchema      = "dorkpipe.node-connector-service-diagnostic-fixture/v1"
	NodeConnectorServiceDiagnosticSchema             = "dorkpipe.node-connector-service-diagnostic/v1"

	nodeConnectorServiceLifecycleIntentProvenance = "fixture_only_local_service_lifecycle_intent"
	nodeConnectorServiceDiagnosticProvenance      = "fixture_only_local_service_diagnostic"
	nodeConnectorServiceLifecycleIntentName       = "node-connector-service-lifecycle-intent.json"
	nodeConnectorServiceDiagnosticName            = "node-connector-service-diagnostic.json"
	nodeConnectorServiceMaxFixtureBytes           = 64 << 10
	nodeConnectorServiceMaxArtifactBytes          = 128 << 10
	nodeConnectorServiceMaxDiagnosticReferences   = 16
	nodeConnectorServiceMaxDiagnosticBytes        = int64(32 << 20)
	nodeConnectorServiceMaxDiagnosticTotalBytes   = int64(64 << 20)
)

var (
	nodeConnectorServiceWriteIntentAtomic     = writeJSONFileAtomic
	nodeConnectorServiceWriteDiagnosticAtomic = writeJSONFileAtomic
)

type NodeConnectorServicePlatformProfile struct {
	HostOS         string `json:"host_os"`
	ServiceManager string `json:"service_manager"`
	InstallScope   string `json:"install_scope"`
}

type NodeConnectorServiceTarget struct {
	ServiceID                       string                              `json:"service_id"`
	MachineID                       string                              `json:"machine_id"`
	ConnectorArtifactFingerprint    string                              `json:"connector_artifact_fingerprint"`
	ServiceConfigurationFingerprint string                              `json:"service_configuration_fingerprint"`
	Platform                        NodeConnectorServicePlatformProfile `json:"platform"`
}

type NodeConnectorServiceDiagnosticReference struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Digest    string `json:"digest"`
	Bytes     int64  `json:"bytes"`
}

type NodeConnectorServiceLifecycleExpected struct {
	Target               NodeConnectorServiceTarget                `json:"target"`
	DiagnosticReferences []NodeConnectorServiceDiagnosticReference `json:"diagnostic_references"`
}

// NodeConnectorServiceAuthority is deliberately all false. Fixture lifecycle
// and diagnostic evidence cannot authorize a service operation or any adjacent
// execution, orchestration, network, provider, mutation, or Git transition.
type NodeConnectorServiceAuthority struct {
	Installer       bool `json:"installer"`
	ServiceManager  bool `json:"service_manager"`
	Process         bool `json:"process"`
	DiagnosticProbe bool `json:"diagnostic_probe"`
	Execution       bool `json:"execution"`
	Validation      bool `json:"validation"`
	Scheduling      bool `json:"scheduling"`
	Transport       bool `json:"transport"`
	Provider        bool `json:"provider"`
	Network         bool `json:"network"`
	Mutation        bool `json:"mutation"`
	Repair          bool `json:"repair"`
	Git             bool `json:"git"`
	Apply           bool `json:"apply"`
	Checkpoint      bool `json:"checkpoint"`
	Commit          bool `json:"commit"`
	Push            bool `json:"push"`
	Publication     bool `json:"publication"`
	Completion      bool `json:"completion"`
	Lifecycle       bool `json:"lifecycle"`
}

type NodeConnectorServiceLifecycleIntentFixture struct {
	Schema     string                     `json:"schema"`
	IntentID   string                     `json:"intent_id"`
	ReplayID   string                     `json:"replay_id"`
	Target     NodeConnectorServiceTarget `json:"target"`
	Intent     string                     `json:"intent"`
	Provenance string                     `json:"provenance"`
}

type NodeConnectorServiceLifecycleIntent struct {
	Schema                       string                        `json:"schema"`
	IntentID                     string                        `json:"intent_id"`
	ReplayID                     string                        `json:"replay_id"`
	Target                       NodeConnectorServiceTarget    `json:"target"`
	Intent                       string                        `json:"intent"`
	Provenance                   string                        `json:"provenance"`
	InstallerInvoked             bool                          `json:"installer_invoked"`
	ServiceManagerInvoked        bool                          `json:"service_manager_invoked"`
	ProcessInvoked               bool                          `json:"process_invoked"`
	MutationPerformed            bool                          `json:"mutation_performed"`
	LifecycleTransitionCompleted bool                          `json:"lifecycle_transition_completed"`
	FutureOperationAuthorized    bool                          `json:"future_operation_authorized"`
	Authority                    NodeConnectorServiceAuthority `json:"authority"`
	IntentFingerprint            string                        `json:"intent_fingerprint"`
}

type NodeConnectorServiceDiagnosticFixture struct {
	Schema                     string                                    `json:"schema"`
	DiagnosticID               string                                    `json:"diagnostic_id"`
	ReplayID                   string                                    `json:"replay_id"`
	LifecycleIntentID          string                                    `json:"lifecycle_intent_id"`
	LifecycleIntentFingerprint string                                    `json:"lifecycle_intent_fingerprint"`
	Target                     NodeConnectorServiceTarget                `json:"target"`
	ServiceState               string                                    `json:"service_state"`
	Health                     string                                    `json:"health"`
	FailureClass               string                                    `json:"failure_class"`
	ObservedAt                 string                                    `json:"observed_at"`
	References                 []NodeConnectorServiceDiagnosticReference `json:"references"`
	Provenance                 string                                    `json:"provenance"`
}

type NodeConnectorServiceDiagnostic struct {
	Schema                     string                                    `json:"schema"`
	DiagnosticID               string                                    `json:"diagnostic_id"`
	ReplayID                   string                                    `json:"replay_id"`
	LifecycleIntentID          string                                    `json:"lifecycle_intent_id"`
	LifecycleIntentFingerprint string                                    `json:"lifecycle_intent_fingerprint"`
	Target                     NodeConnectorServiceTarget                `json:"target"`
	ServiceState               string                                    `json:"service_state"`
	Health                     string                                    `json:"health"`
	FailureClass               string                                    `json:"failure_class"`
	ObservedAt                 string                                    `json:"observed_at"`
	References                 []NodeConnectorServiceDiagnosticReference `json:"references"`
	Provenance                 string                                    `json:"provenance"`
	ProbeInvoked               bool                                      `json:"probe_invoked"`
	LifecycleIntentExecuted    bool                                      `json:"lifecycle_intent_executed"`
	LifecycleTransitionProven  bool                                      `json:"lifecycle_transition_proven"`
	ConnectorPresenceProven    bool                                      `json:"connector_presence_proven"`
	SessionHealthProven        bool                                      `json:"session_health_proven"`
	ExecutionReadinessProven   bool                                      `json:"execution_readiness_proven"`
	LeaseAuthorityGranted      bool                                      `json:"lease_authority_granted"`
	CompletionProven           bool                                      `json:"completion_proven"`
	Authority                  NodeConnectorServiceAuthority             `json:"authority"`
	DiagnosticFingerprint      string                                    `json:"diagnostic_fingerprint"`
}

type NodeConnectorServiceLifecycle struct {
	root       string
	expected   NodeConnectorServiceLifecycleExpected
	intent     *NodeConnectorServiceLifecycleIntent
	diagnostic *NodeConnectorServiceDiagnostic
	mu         sync.Mutex
}

func OpenNodeConnectorServiceLifecycle(root string, expected NodeConnectorServiceLifecycleExpected) (*NodeConnectorServiceLifecycle, error) {
	normalized, err := normalizeNodeConnectorServiceLifecycleExpected(expected)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("connector service lifecycle root must be an existing regular directory")
	}
	lifecycle := &NodeConnectorServiceLifecycle{root: root, expected: normalized}
	intent, intentExists, err := loadNodeConnectorServiceLifecycleIntent(root, normalized)
	if err != nil {
		return nil, err
	}
	diagnostic, diagnosticExists, err := loadNodeConnectorServiceDiagnostic(root, normalized, intent, intentExists)
	if err != nil {
		return nil, err
	}
	if diagnosticExists && !intentExists {
		return nil, errors.New("connector service diagnostic exists without its exact durable lifecycle intent")
	}
	if intentExists {
		lifecycle.intent = &intent
	}
	if diagnosticExists {
		lifecycle.diagnostic = &diagnostic
	}
	return lifecycle, nil
}

func (lifecycle *NodeConnectorServiceLifecycle) RecordIntent(raw []byte) (NodeConnectorServiceLifecycleIntent, error) {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorServiceMaxFixtureBytes {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorServiceLifecycleIntentFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent fixture is not strict canonical JSON")
	}
	intent, err := deriveNodeConnectorServiceLifecycleIntent(lifecycle.expected, fixture)
	if err != nil {
		return NodeConnectorServiceLifecycleIntent{}, err
	}
	if lifecycle.intent != nil {
		if !nodeExecutionEqual(*lifecycle.intent, intent) {
			return NodeConnectorServiceLifecycleIntent{}, errors.New("changed or conflicting connector service lifecycle intent replay is rejected")
		}
		return cloneNodeConnectorServiceLifecycleIntent(*lifecycle.intent), nil
	}
	path := filepath.Join(lifecycle.root, nodeConnectorServiceLifecycleIntentName)
	if err := requireNodeConnectorServiceArtifactAbsent(path, "lifecycle intent"); err != nil {
		return NodeConnectorServiceLifecycleIntent{}, err
	}
	if err := nodeConnectorServiceWriteIntentAtomic(path, intent); err != nil {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent could not be published")
	}
	lifecycle.intent = &intent
	return cloneNodeConnectorServiceLifecycleIntent(intent), nil
}

func (lifecycle *NodeConnectorServiceLifecycle) RecordDiagnostic(raw []byte) (NodeConnectorServiceDiagnostic, error) {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorServiceMaxFixtureBytes {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic fixture exceeds its encoded bound")
	}
	if lifecycle.intent == nil {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic requires an exact durable lifecycle intent")
	}
	intent, intentExists, err := loadNodeConnectorServiceLifecycleIntent(lifecycle.root, lifecycle.expected)
	if err != nil || !intentExists || !nodeExecutionEqual(intent, *lifecycle.intent) {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic could not revalidate its durable lifecycle intent")
	}
	var fixture NodeConnectorServiceDiagnosticFixture
	if err := decodeNodeExecutionCanonical(raw, &fixture); err != nil {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic fixture is not strict canonical JSON")
	}
	diagnostic, err := deriveNodeConnectorServiceDiagnostic(lifecycle.expected, intent, fixture)
	if err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	if lifecycle.diagnostic != nil {
		if !nodeExecutionEqual(*lifecycle.diagnostic, diagnostic) {
			return NodeConnectorServiceDiagnostic{}, errors.New("changed or conflicting connector service diagnostic replay is rejected")
		}
		return cloneNodeConnectorServiceDiagnostic(*lifecycle.diagnostic), nil
	}
	path := filepath.Join(lifecycle.root, nodeConnectorServiceDiagnosticName)
	if err := requireNodeConnectorServiceArtifactAbsent(path, "diagnostic"); err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	if err := nodeConnectorServiceWriteDiagnosticAtomic(path, diagnostic); err != nil {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic could not be published after the durable lifecycle intent")
	}
	lifecycle.diagnostic = &diagnostic
	return cloneNodeConnectorServiceDiagnostic(diagnostic), nil
}

func (lifecycle *NodeConnectorServiceLifecycle) Artifacts() (*NodeConnectorServiceLifecycleIntent, *NodeConnectorServiceDiagnostic) {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	var intent *NodeConnectorServiceLifecycleIntent
	var diagnostic *NodeConnectorServiceDiagnostic
	if lifecycle.intent != nil {
		cloned := cloneNodeConnectorServiceLifecycleIntent(*lifecycle.intent)
		intent = &cloned
	}
	if lifecycle.diagnostic != nil {
		cloned := cloneNodeConnectorServiceDiagnostic(*lifecycle.diagnostic)
		diagnostic = &cloned
	}
	return intent, diagnostic
}

func deriveNodeConnectorServiceLifecycleIntent(expected NodeConnectorServiceLifecycleExpected, fixture NodeConnectorServiceLifecycleIntentFixture) (NodeConnectorServiceLifecycleIntent, error) {
	if fixture.Schema != NodeConnectorServiceLifecycleIntentFixtureSchema || fixture.Provenance != nodeConnectorServiceLifecycleIntentProvenance || !nodeExecutionEqual(fixture.Target, expected.Target) {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent does not exactly bind the expected service target")
	}
	if err := validateNodeExecutionTypedID("intent", fixture.IntentID); err != nil {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayID); err != nil || fixture.ReplayID == fixture.IntentID {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle replay identity is invalid or colliding")
	}
	if !isNodeConnectorServiceLifecycleIntent(fixture.Intent) {
		return NodeConnectorServiceLifecycleIntent{}, errors.New("connector service lifecycle intent value is not allowed")
	}
	intent := NodeConnectorServiceLifecycleIntent{
		Schema: NodeConnectorServiceLifecycleIntentSchema, IntentID: fixture.IntentID, ReplayID: fixture.ReplayID,
		Target: fixture.Target, Intent: fixture.Intent, Provenance: nodeConnectorServiceLifecycleIntentProvenance,
	}
	fingerprint, err := nodeConnectorServiceLifecycleIntentFingerprint(intent)
	if err != nil {
		return NodeConnectorServiceLifecycleIntent{}, err
	}
	intent.IntentFingerprint = fingerprint
	if err := validateNodeConnectorServiceLifecycleIntent(intent, expected); err != nil {
		return NodeConnectorServiceLifecycleIntent{}, err
	}
	return intent, nil
}

func deriveNodeConnectorServiceDiagnostic(expected NodeConnectorServiceLifecycleExpected, intent NodeConnectorServiceLifecycleIntent, fixture NodeConnectorServiceDiagnosticFixture) (NodeConnectorServiceDiagnostic, error) {
	if fixture.Schema != NodeConnectorServiceDiagnosticFixtureSchema || fixture.Provenance != nodeConnectorServiceDiagnosticProvenance ||
		fixture.LifecycleIntentID != intent.IntentID || fixture.LifecycleIntentFingerprint != intent.IntentFingerprint || !nodeExecutionEqual(fixture.Target, expected.Target) {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic does not exactly bind the durable lifecycle intent and service target")
	}
	if err := validateNodeExecutionTypedID("diagnostic", fixture.DiagnosticID); err != nil {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic identity is invalid")
	}
	if err := validateNodeExecutionTypedID("replay", fixture.ReplayID); err != nil || fixture.ReplayID == fixture.DiagnosticID || fixture.ReplayID == intent.ReplayID {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic replay identity is invalid or colliding")
	}
	if fixture.DiagnosticID == intent.IntentID || !nodeExecutionEqual(fixture.References, expected.DiagnosticReferences) {
		return NodeConnectorServiceDiagnostic{}, errors.New("connector service diagnostic identity or exact reference binding is invalid")
	}
	if err := validateNodeConnectorServiceClassification(fixture.ServiceState, fixture.Health, fixture.FailureClass); err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	if _, err := parseNodeExecutionTime(fixture.ObservedAt); err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	diagnostic := NodeConnectorServiceDiagnostic{
		Schema: NodeConnectorServiceDiagnosticSchema, DiagnosticID: fixture.DiagnosticID, ReplayID: fixture.ReplayID,
		LifecycleIntentID: intent.IntentID, LifecycleIntentFingerprint: intent.IntentFingerprint, Target: fixture.Target,
		ServiceState: fixture.ServiceState, Health: fixture.Health, FailureClass: fixture.FailureClass,
		ObservedAt: fixture.ObservedAt, References: cloneNodeConnectorServiceDiagnosticReferences(fixture.References), Provenance: nodeConnectorServiceDiagnosticProvenance,
	}
	fingerprint, err := nodeConnectorServiceDiagnosticFingerprint(diagnostic)
	if err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	diagnostic.DiagnosticFingerprint = fingerprint
	if err := validateNodeConnectorServiceDiagnostic(diagnostic, expected, intent); err != nil {
		return NodeConnectorServiceDiagnostic{}, err
	}
	return diagnostic, nil
}

func normalizeNodeConnectorServiceLifecycleExpected(value NodeConnectorServiceLifecycleExpected) (NodeConnectorServiceLifecycleExpected, error) {
	if err := validateNodeConnectorServiceTarget(value.Target); err != nil {
		return NodeConnectorServiceLifecycleExpected{}, err
	}
	value.DiagnosticReferences = cloneNodeConnectorServiceDiagnosticReferences(value.DiagnosticReferences)
	if err := validateNodeConnectorServiceDiagnosticReferences(value.DiagnosticReferences); err != nil {
		return NodeConnectorServiceLifecycleExpected{}, err
	}
	return value, nil
}

func validateNodeConnectorServiceTarget(value NodeConnectorServiceTarget) error {
	if err := validateNodeExecutionTypedID("service", value.ServiceID); err != nil {
		return errors.New("connector service identity is invalid")
	}
	if err := validateNodeExecutionTypedID("machine", value.MachineID); err != nil {
		return errors.New("connector service machine identity is invalid")
	}
	if !nodeExecutionFingerprint.MatchString(value.ConnectorArtifactFingerprint) || !nodeExecutionFingerprint.MatchString(value.ServiceConfigurationFingerprint) {
		return errors.New("connector service artifact or immutable configuration fingerprint is invalid")
	}
	windows := value.Platform == (NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"})
	linux := value.Platform == (NodeConnectorServicePlatformProfile{HostOS: "linux", ServiceManager: "systemd", InstallScope: "system"})
	if !windows && !linux {
		return errors.New("connector service platform profile is unsupported or ambiguous")
	}
	return nil
}

func validateNodeConnectorServiceDiagnosticReferences(values []NodeConnectorServiceDiagnosticReference) error {
	if len(values) > nodeConnectorServiceMaxDiagnosticReferences || !sort.SliceIsSorted(values, func(i, j int) bool { return values[i].Name < values[j].Name }) {
		return errors.New("connector service diagnostic references exceed their count bound or are not ordinally sorted")
	}
	last := ""
	total := int64(0)
	for _, value := range values {
		if err := validateNodeExecutionName("diagnostic reference name", value.Name); err != nil || value.Name <= last || strings.ContainsAny(value.Name, `/\\`) || strings.Contains(value.Name, "..") {
			return errors.New("connector service diagnostic reference name is invalid, duplicate, or unsorted")
		}
		if value.MediaType == "" || len(value.MediaType) > 128 || strings.Contains(value.MediaType, "://") || strings.ContainsAny(value.MediaType, "\r\n\t") || !strings.Contains(value.MediaType, "/") {
			return errors.New("connector service diagnostic reference media type is invalid")
		}
		if !nodeExecutionFingerprint.MatchString(value.Digest) || value.Bytes < 0 || value.Bytes > nodeConnectorServiceMaxDiagnosticBytes || total > nodeConnectorServiceMaxDiagnosticTotalBytes-value.Bytes {
			return errors.New("connector service diagnostic reference digest or byte bounds are invalid")
		}
		total += value.Bytes
		last = value.Name
	}
	return nil
}

func validateNodeConnectorServiceLifecycleIntent(value NodeConnectorServiceLifecycleIntent, expected NodeConnectorServiceLifecycleExpected) error {
	if value.Schema != NodeConnectorServiceLifecycleIntentSchema || value.Provenance != nodeConnectorServiceLifecycleIntentProvenance || !nodeExecutionEqual(value.Target, expected.Target) ||
		value.InstallerInvoked || value.ServiceManagerInvoked || value.ProcessInvoked || value.MutationPerformed || value.LifecycleTransitionCompleted || value.FutureOperationAuthorized || value.Authority != (NodeConnectorServiceAuthority{}) || !isNodeConnectorServiceLifecycleIntent(value.Intent) {
		return errors.New("connector service lifecycle intent contract, target binding, or authority is invalid")
	}
	if err := validateNodeExecutionTypedID("intent", value.IntentID); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("replay", value.ReplayID); err != nil || value.ReplayID == value.IntentID {
		return errors.New("connector service lifecycle replay identity is invalid")
	}
	fingerprint, err := nodeConnectorServiceLifecycleIntentFingerprint(value)
	if err != nil || fingerprint != value.IntentFingerprint {
		return errors.New("connector service lifecycle intent fingerprint is invalid")
	}
	return validateNodeConnectorServiceEncodedBound(value)
}

func validateNodeConnectorServiceDiagnostic(value NodeConnectorServiceDiagnostic, expected NodeConnectorServiceLifecycleExpected, intent NodeConnectorServiceLifecycleIntent) error {
	if value.Schema != NodeConnectorServiceDiagnosticSchema || value.Provenance != nodeConnectorServiceDiagnosticProvenance ||
		value.LifecycleIntentID != intent.IntentID || value.LifecycleIntentFingerprint != intent.IntentFingerprint || !nodeExecutionEqual(value.Target, expected.Target) ||
		!nodeExecutionEqual(value.References, expected.DiagnosticReferences) || value.ProbeInvoked || value.LifecycleIntentExecuted || value.LifecycleTransitionProven ||
		value.ConnectorPresenceProven || value.SessionHealthProven || value.ExecutionReadinessProven || value.LeaseAuthorityGranted || value.CompletionProven || value.Authority != (NodeConnectorServiceAuthority{}) {
		return errors.New("connector service diagnostic contract, binding, evidence claim, or authority is invalid")
	}
	if err := validateNodeExecutionTypedID("diagnostic", value.DiagnosticID); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("replay", value.ReplayID); err != nil || value.ReplayID == value.DiagnosticID || value.ReplayID == intent.ReplayID || value.DiagnosticID == intent.IntentID {
		return errors.New("connector service diagnostic identity or replay identity is invalid")
	}
	if err := validateNodeConnectorServiceClassification(value.ServiceState, value.Health, value.FailureClass); err != nil {
		return err
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return err
	}
	if err := validateNodeConnectorServiceDiagnosticReferences(value.References); err != nil {
		return err
	}
	fingerprint, err := nodeConnectorServiceDiagnosticFingerprint(value)
	if err != nil || fingerprint != value.DiagnosticFingerprint {
		return errors.New("connector service diagnostic fingerprint is invalid")
	}
	return validateNodeConnectorServiceEncodedBound(value)
}

func validateNodeConnectorServiceClassification(state, health, failure string) error {
	valid := (state == "not_installed" && health == "unknown" && failure == "none") ||
		(state == "stopped" && health == "unknown" && failure == "none") ||
		(state == "running" && health == "healthy" && failure == "none") ||
		(state == "degraded" && health == "degraded" && isNodeConnectorServiceFailure(failure, false)) ||
		(state == "failed" && health == "unhealthy" && isNodeConnectorServiceFailure(failure, false)) ||
		(state == "unknown" && health == "unknown" && failure == "unknown")
	if !valid {
		return errors.New("connector service diagnostic state, health, and failure classification is inconsistent")
	}
	return nil
}

func isNodeConnectorServiceFailure(value string, allowNone bool) bool {
	if allowNone && value == "none" {
		return true
	}
	switch value {
	case "configuration", "dependency", "permission", "startup_timeout", "unexpected_exit", "unknown":
		return true
	default:
		return false
	}
}

func isNodeConnectorServiceLifecycleIntent(value string) bool {
	switch value {
	case "install", "start", "stop", "restart", "uninstall":
		return true
	default:
		return false
	}
}

func loadNodeConnectorServiceLifecycleIntent(root string, expected NodeConnectorServiceLifecycleExpected) (NodeConnectorServiceLifecycleIntent, bool, error) {
	path := filepath.Join(root, nodeConnectorServiceLifecycleIntentName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorServiceLifecycleIntent{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorServiceMaxArtifactBytes {
		return NodeConnectorServiceLifecycleIntent{}, false, errors.New("durable connector service lifecycle intent is missing, malformed, or oversized")
	}
	var value NodeConnectorServiceLifecycleIntent
	if err := decodeNodeConnectorServiceArtifact(raw, &value); err != nil || validateNodeConnectorServiceLifecycleIntent(value, expected) != nil {
		return NodeConnectorServiceLifecycleIntent{}, false, errors.New("durable connector service lifecycle intent is malformed, noncanonical, or tampered")
	}
	return value, true, nil
}

func loadNodeConnectorServiceDiagnostic(root string, expected NodeConnectorServiceLifecycleExpected, intent NodeConnectorServiceLifecycleIntent, intentExists bool) (NodeConnectorServiceDiagnostic, bool, error) {
	path := filepath.Join(root, nodeConnectorServiceDiagnosticName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NodeConnectorServiceDiagnostic{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorServiceMaxArtifactBytes || !intentExists {
		return NodeConnectorServiceDiagnostic{}, false, errors.New("durable connector service diagnostic is missing its intent, malformed, or oversized")
	}
	var value NodeConnectorServiceDiagnostic
	if err := decodeNodeConnectorServiceArtifact(raw, &value); err != nil || validateNodeConnectorServiceDiagnostic(value, expected, intent) != nil {
		return NodeConnectorServiceDiagnostic{}, false, errors.New("durable connector service diagnostic is malformed, noncanonical, or tampered")
	}
	return value, true, nil
}

func nodeConnectorServiceLifecycleIntentFingerprint(value NodeConnectorServiceLifecycleIntent) (string, error) {
	value.IntentFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func nodeConnectorServiceDiagnosticFingerprint(value NodeConnectorServiceDiagnostic) (string, error) {
	value.DiagnosticFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func validateNodeConnectorServiceEncodedBound(value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorServiceMaxArtifactBytes {
		return errors.New("connector service durable artifact exceeds its encoded bound")
	}
	return nil
}

func decodeNodeConnectorServiceArtifact(raw []byte, target any) error {
	if err := decodeNodeExecutionStrict(raw, target); err != nil {
		return err
	}
	canonical, err := json.MarshalIndent(target, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("connector service durable artifact is not canonical")
	}
	return nil
}

func requireNodeConnectorServiceArtifactAbsent(path, kind string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("unexpected connector service " + kind + " artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cloneNodeConnectorServiceLifecycleIntent(value NodeConnectorServiceLifecycleIntent) NodeConnectorServiceLifecycleIntent {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorServiceLifecycleIntent
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorServiceDiagnostic(value NodeConnectorServiceDiagnostic) NodeConnectorServiceDiagnostic {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorServiceDiagnostic
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorServiceDiagnosticReferences(values []NodeConnectorServiceDiagnosticReference) []NodeConnectorServiceDiagnosticReference {
	return append([]NodeConnectorServiceDiagnosticReference{}, values...)
}

func EncodeNodeConnectorServiceLifecycleIntentFixture(value NodeConnectorServiceLifecycleIntentFixture) ([]byte, error) {
	return json.Marshal(value)
}

func EncodeNodeConnectorServiceDiagnosticFixture(value NodeConnectorServiceDiagnosticFixture) ([]byte, error) {
	return json.Marshal(value)
}
