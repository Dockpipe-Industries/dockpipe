package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNodeConnectorServiceLifecycleProfilesIntentsAndDiagnostics(t *testing.T) {
	profiles := []NodeConnectorServicePlatformProfile{
		{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"},
		{HostOS: "linux", ServiceManager: "systemd", InstallScope: "system"},
	}
	intents := []string{"install", "start", "stop", "restart", "uninstall"}
	classifications := []struct {
		state, health, failure string
	}{
		{state: "not_installed", health: "unknown", failure: "none"},
		{state: "stopped", health: "unknown", failure: "none"},
		{state: "running", health: "healthy", failure: "none"},
		{state: "degraded", health: "degraded", failure: "configuration"},
		{state: "failed", health: "unhealthy", failure: "unexpected_exit"},
		{state: "unknown", health: "unknown", failure: "unknown"},
	}
	for profileIndex, profile := range profiles {
		for intentIndex, intentValue := range intents {
			classification := classifications[(profileIndex+intentIndex)%len(classifications)]
			t.Run(profile.HostOS+"_"+intentValue, func(t *testing.T) {
				root, expected, intentFixture := nodeConnectorServiceLifecycleFixture(t, profile, intentValue)
				lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
				intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, intentFixture)
				diagnosticFixture := nodeConnectorServiceDiagnosticFixture(intent, expected, classification.state, classification.health, classification.failure)
				diagnostic := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, diagnosticFixture)
				if intent.Intent != intentValue || intent.Target.Platform != profile || diagnostic.ServiceState != classification.state || diagnostic.Health != classification.health || diagnostic.FailureClass != classification.failure {
					t.Fatal("accepted service lifecycle evidence changed its exact profile, intent, or classification")
				}
				if intent.InstallerInvoked || intent.ServiceManagerInvoked || intent.ProcessInvoked || intent.MutationPerformed || intent.LifecycleTransitionCompleted || intent.FutureOperationAuthorized || intent.Authority != (NodeConnectorServiceAuthority{}) {
					t.Fatal("lifecycle intent claimed a service operation or authority")
				}
				if diagnostic.ProbeInvoked || diagnostic.LifecycleIntentExecuted || diagnostic.LifecycleTransitionProven || diagnostic.ConnectorPresenceProven || diagnostic.SessionHealthProven || diagnostic.ExecutionReadinessProven || diagnostic.LeaseAuthorityGranted || diagnostic.CompletionProven || diagnostic.Authority != (NodeConnectorServiceAuthority{}) {
					t.Fatal("diagnostic evidence claimed execution, health, completion, lease, or lifecycle authority")
				}
			})
		}
	}
	for _, classification := range classifications {
		t.Run("classification_"+classification.state, func(t *testing.T) {
			root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, profiles[0], "start")
			lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
			intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
			diagnostic := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, nodeConnectorServiceDiagnosticFixture(intent, expected, classification.state, classification.health, classification.failure))
			if diagnostic.ServiceState != classification.state {
				t.Fatal("allowed service-state classification was not preserved")
			}
		})
	}
}

func TestNodeConnectorServiceLifecycleBindsExactIntentDiagnosticAndTarget(t *testing.T) {
	profile := NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"}
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, profile, "restart")
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorServiceLifecycleIntentFixture)
	}{
		{name: "service", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) { value.Target.ServiceID = "service-other-001" }},
		{name: "machine", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) { value.Target.MachineID = "machine-other-001" }},
		{name: "connector artifact", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) {
			value.Target.ConnectorArtifactFingerprint = nodeConnectorServiceFingerprint("c")
		}},
		{name: "configuration", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) {
			value.Target.ServiceConfigurationFingerprint = nodeConnectorServiceFingerprint("d")
		}},
		{name: "host os", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) { value.Target.Platform.HostOS = "linux" }},
		{name: "service manager", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) {
			value.Target.Platform.ServiceManager = "systemd"
		}},
		{name: "install scope", mutate: func(value *NodeConnectorServiceLifecycleIntentFixture) { value.Target.Platform.InstallScope = "system" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			changed := fixture
			tc.mutate(&changed)
			if _, err := mustOpenNodeConnectorServiceLifecycle(t, root, expected).RecordIntent(mustMarshalNodeConnectorService(t, changed)); err == nil {
				t.Fatal("wrong service target binding was accepted")
			}
		})
	}
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	diagnosticFixture := nodeConnectorServiceDiagnosticFixture(intent, expected, "running", "healthy", "none")
	diagnosticMutations := []struct {
		name   string
		mutate func(*NodeConnectorServiceDiagnosticFixture)
	}{
		{name: "intent id", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.LifecycleIntentID = "intent-other-001" }},
		{name: "intent fingerprint", mutate: func(value *NodeConnectorServiceDiagnosticFixture) {
			value.LifecycleIntentFingerprint = nodeConnectorServiceFingerprint("f")
		}},
		{name: "service", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.Target.ServiceID = "service-other-001" }},
		{name: "machine", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.Target.MachineID = "machine-other-001" }},
		{name: "configuration", mutate: func(value *NodeConnectorServiceDiagnosticFixture) {
			value.Target.ServiceConfigurationFingerprint = nodeConnectorServiceFingerprint("e")
		}},
		{name: "os", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.Target.Platform.HostOS = "linux" }},
	}
	for _, tc := range diagnosticMutations {
		t.Run("diagnostic_"+tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorServiceDiagnosticFixture(diagnosticFixture)
			tc.mutate(&changed)
			if _, err := lifecycle.RecordDiagnostic(mustMarshalNodeConnectorService(t, changed)); err == nil {
				t.Fatal("wrong diagnostic intent or target binding was accepted")
			}
		})
	}
}

func TestNodeConnectorServiceLifecycleRequiresExactSortedBoundedDiagnosticReferences(t *testing.T) {
	profile := NodeConnectorServicePlatformProfile{HostOS: "linux", ServiceManager: "systemd", InstallScope: "system"}
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, profile, "start")
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	valid := nodeConnectorServiceDiagnosticFixture(intent, expected, "degraded", "degraded", "dependency")
	mutations := []struct {
		name   string
		mutate func(*NodeConnectorServiceDiagnosticFixture)
	}{
		{name: "missing", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.References = value.References[:1] }},
		{name: "duplicate", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.References[1] = value.References[0] }},
		{name: "reordered", mutate: func(value *NodeConnectorServiceDiagnosticFixture) {
			value.References[0], value.References[1] = value.References[1], value.References[0]
		}},
		{name: "extra", mutate: func(value *NodeConnectorServiceDiagnosticFixture) {
			value.References = append(value.References, NodeConnectorServiceDiagnosticReference{Name: "summary-three", MediaType: "application/json", Digest: nodeConnectorServiceFingerprint("3"), Bytes: 3})
		}},
		{name: "name substitution", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.References[0].Name = "different-one" }},
		{name: "media substitution", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.References[0].MediaType = "text/plain" }},
		{name: "digest substitution", mutate: func(value *NodeConnectorServiceDiagnosticFixture) {
			value.References[0].Digest = nodeConnectorServiceFingerprint("9")
		}},
		{name: "byte substitution", mutate: func(value *NodeConnectorServiceDiagnosticFixture) { value.References[0].Bytes++ }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneNodeConnectorServiceDiagnosticFixture(valid)
			tc.mutate(&changed)
			if _, err := lifecycle.RecordDiagnostic(mustMarshalNodeConnectorService(t, changed)); err == nil {
				t.Fatal("missing, duplicate, reordered, extra, or substituted diagnostic reference was accepted")
			}
		})
	}
	tooMany := expected
	tooMany.DiagnosticReferences = make([]NodeConnectorServiceDiagnosticReference, nodeConnectorServiceMaxDiagnosticReferences+1)
	for index := range tooMany.DiagnosticReferences {
		tooMany.DiagnosticReferences[index] = NodeConnectorServiceDiagnosticReference{Name: "reference-" + strings.Repeat("0", 3-len(strings.TrimLeft(strings.Repeat("0", 3)+string(rune('0'+index%10)), "0"))) + string(rune('0'+index%10)), MediaType: "application/json", Digest: nodeConnectorServiceFingerprint("a"), Bytes: 1}
	}
	if _, err := OpenNodeConnectorServiceLifecycle(t.TempDir(), tooMany); err == nil {
		t.Fatal("diagnostic reference count bound was not enforced")
	}
	oversized := expected
	oversized.DiagnosticReferences = cloneNodeConnectorServiceDiagnosticReferences(expected.DiagnosticReferences)
	oversized.DiagnosticReferences[0].Bytes = nodeConnectorServiceMaxDiagnosticBytes + 1
	if _, err := OpenNodeConnectorServiceLifecycle(t.TempDir(), oversized); err == nil {
		t.Fatal("diagnostic reference byte bound was not enforced")
	}
	diagnostic := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, valid)
	if !reflect.DeepEqual(diagnostic.References, expected.DiagnosticReferences) {
		t.Fatal("accepted diagnostic references were not preserved exactly")
	}
}

func TestNodeConnectorServiceLifecycleRejectsInvalidClassificationsProfilesAndInput(t *testing.T) {
	invalidProfiles := []NodeConnectorServicePlatformProfile{
		{HostOS: "windows", ServiceManager: "systemd", InstallScope: "machine"},
		{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "system"},
		{HostOS: "linux", ServiceManager: "windows_scm", InstallScope: "system"},
		{HostOS: "linux", ServiceManager: "systemd", InstallScope: "machine"},
		{HostOS: "darwin", ServiceManager: "launchd", InstallScope: "system"},
	}
	for _, profile := range invalidProfiles {
		_, expected, _ := nodeConnectorServiceLifecycleFixture(t, NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"}, "start")
		expected.Target.Platform = profile
		if _, err := OpenNodeConnectorServiceLifecycle(t.TempDir(), expected); err == nil {
			t.Fatal("unsupported or substituted platform profile was accepted")
		}
	}
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"}, "start")
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	invalid := [][3]string{
		{"running", "unhealthy", "none"}, {"running", "healthy", "dependency"}, {"degraded", "degraded", "none"},
		{"failed", "unhealthy", "none"}, {"stopped", "healthy", "none"}, {"not_installed", "unknown", "unknown"},
		{"unknown", "unknown", "none"}, {"other", "unknown", "unknown"},
	}
	for _, values := range invalid {
		changed := nodeConnectorServiceDiagnosticFixture(intent, expected, values[0], values[1], values[2])
		if _, err := lifecycle.RecordDiagnostic(mustMarshalNodeConnectorService(t, changed)); err == nil {
			t.Fatal("internally inconsistent service diagnostic classification was accepted")
		}
	}
	validIntent := mustMarshalNodeConnectorService(t, fixture)
	inputs := [][]byte{[]byte("{not-json"), append(append([]byte{}, validIntent...), []byte(" trailing")...), make([]byte, nodeConnectorServiceMaxFixtureBytes+1)}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, validIntent, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs = append(inputs, pretty.Bytes())
	for _, field := range []string{"unknown", "command", "path", "executable", "account", "credential", "environment", "pid", "provider", "connection", "availability", "ingress", "stdout", "stderr", "raw_output"} {
		inputs = append(inputs, append(append([]byte{}, validIntent[:len(validIntent)-1]...), []byte(`,"`+field+`":"forbidden"}`)...))
	}
	otherRoot, otherExpected, _ := nodeConnectorServiceLifecycleFixture(t, expected.Target.Platform, "start")
	for _, raw := range inputs {
		if _, err := mustOpenNodeConnectorServiceLifecycle(t, otherRoot, otherExpected).RecordIntent(raw); err == nil {
			t.Fatal("malformed, noncanonical, oversized, trailing, unknown, or forbidden intent input was accepted")
		}
	}
}

func TestNodeConnectorServiceLifecycleReplayRestartConflictsExpectedChangesAndTamper(t *testing.T) {
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, NodeConnectorServicePlatformProfile{HostOS: "linux", ServiceManager: "systemd", InstallScope: "system"}, "install")
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	diagnosticFixture := nodeConnectorServiceDiagnosticFixture(intent, expected, "running", "healthy", "none")
	diagnostic := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, diagnosticFixture)

	originalIntentWriter := nodeConnectorServiceWriteIntentAtomic
	originalDiagnosticWriter := nodeConnectorServiceWriteDiagnosticAtomic
	writes := 0
	nodeConnectorServiceWriteIntentAtomic = func(path string, value any) error { writes++; return originalIntentWriter(path, value) }
	nodeConnectorServiceWriteDiagnosticAtomic = func(path string, value any) error { writes++; return originalDiagnosticWriter(path, value) }
	t.Cleanup(func() {
		nodeConnectorServiceWriteIntentAtomic = originalIntentWriter
		nodeConnectorServiceWriteDiagnosticAtomic = originalDiagnosticWriter
	})
	if replay := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture); !reflect.DeepEqual(replay, intent) || writes != 0 {
		t.Fatal("identical intent replay rewrote or changed the durable artifact")
	}
	if replay := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, diagnosticFixture); !reflect.DeepEqual(replay, diagnostic) || writes != 0 {
		t.Fatal("identical diagnostic replay rewrote or changed the durable artifact")
	}
	restarted := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	if replay := mustRecordNodeConnectorServiceIntent(t, restarted, fixture); !reflect.DeepEqual(replay, intent) || writes != 0 {
		t.Fatal("restart intent replay rewrote or changed the durable artifact")
	}
	if replay := mustRecordNodeConnectorServiceDiagnostic(t, restarted, diagnosticFixture); !reflect.DeepEqual(replay, diagnostic) || writes != 0 {
		t.Fatal("restart diagnostic replay rewrote or changed the durable artifact")
	}
	for _, mutate := range []func(*NodeConnectorServiceLifecycleIntentFixture){
		func(value *NodeConnectorServiceLifecycleIntentFixture) { value.IntentID = "intent-conflict-001" },
		func(value *NodeConnectorServiceLifecycleIntentFixture) { value.ReplayID = "replay-conflict-001" },
		func(value *NodeConnectorServiceLifecycleIntentFixture) { value.Intent = "stop" },
	} {
		changed := fixture
		mutate(&changed)
		if _, err := restarted.RecordIntent(mustMarshalNodeConnectorService(t, changed)); err == nil || writes != 0 {
			t.Fatal("conflicting intent replay was accepted or rewrote state")
		}
	}
	for _, mutate := range []func(*NodeConnectorServiceDiagnosticFixture){
		func(value *NodeConnectorServiceDiagnosticFixture) { value.DiagnosticID = "diagnostic-conflict-001" },
		func(value *NodeConnectorServiceDiagnosticFixture) { value.ReplayID = "replay-diagnostic-conflict-001" },
		func(value *NodeConnectorServiceDiagnosticFixture) {
			value.ServiceState, value.Health, value.FailureClass = "failed", "unhealthy", "permission"
		},
	} {
		changed := cloneNodeConnectorServiceDiagnosticFixture(diagnosticFixture)
		mutate(&changed)
		if _, err := restarted.RecordDiagnostic(mustMarshalNodeConnectorService(t, changed)); err == nil || writes != 0 {
			t.Fatal("conflicting diagnostic replay was accepted or rewrote state")
		}
	}
	changedExpected := expected
	changedExpected.Target.MachineID = "machine-changed-001"
	if _, err := OpenNodeConnectorServiceLifecycle(root, changedExpected); err == nil {
		t.Fatal("changed expected service binding was accepted")
	}

	intentPath := filepath.Join(root, nodeConnectorServiceLifecycleIntentName)
	intentRaw := mustReadNodeConnectorServiceFile(t, intentPath)
	if err := os.WriteFile(intentPath, bytes.Replace(intentRaw, []byte(`"intent": "install"`), []byte(`"intent": "start"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorServiceLifecycle(root, expected); err == nil {
		t.Fatal("tampered durable lifecycle intent was accepted")
	}
	if err := os.WriteFile(intentPath, intentRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	diagnosticPath := filepath.Join(root, nodeConnectorServiceDiagnosticName)
	diagnosticRaw := mustReadNodeConnectorServiceFile(t, diagnosticPath)
	if err := os.WriteFile(diagnosticPath, bytes.Replace(diagnosticRaw, []byte(`"completion_proven": false`), []byte(`"completion_proven": true`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorServiceLifecycle(root, expected); err == nil {
		t.Fatal("tampered durable diagnostic was accepted")
	}
}

func TestNodeConnectorServiceLifecycleAtomicFailuresRecoveryAndNoDiagnosticWithoutIntent(t *testing.T) {
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"}, "restart")
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	originalIntentWriter := nodeConnectorServiceWriteIntentAtomic
	originalDiagnosticWriter := nodeConnectorServiceWriteDiagnosticAtomic
	t.Cleanup(func() {
		nodeConnectorServiceWriteIntentAtomic = originalIntentWriter
		nodeConnectorServiceWriteDiagnosticAtomic = originalDiagnosticWriter
	})
	nodeConnectorServiceWriteIntentAtomic = func(string, any) error { return errors.New("injected intent write failure") }
	if _, err := lifecycle.RecordIntent(mustMarshalNodeConnectorService(t, fixture)); err == nil {
		t.Fatal("intent atomic-write failure was accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("intent write failure published an artifact: %#v, %v", entries, err)
	}

	nodeConnectorServiceWriteIntentAtomic = originalIntentWriter
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	diagnosticFixture := nodeConnectorServiceDiagnosticFixture(intent, expected, "stopped", "unknown", "none")
	nodeConnectorServiceWriteDiagnosticAtomic = func(string, any) error { return errors.New("injected diagnostic write failure") }
	if _, err := lifecycle.RecordDiagnostic(mustMarshalNodeConnectorService(t, diagnosticFixture)); err == nil {
		t.Fatal("diagnostic atomic-write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorServiceLifecycleIntentName)); err != nil {
		t.Fatal("diagnostic failure did not preserve the exact durable intent")
	}
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorServiceDiagnosticName)); !os.IsNotExist(err) {
		t.Fatal("diagnostic failure published a partial diagnostic")
	}
	nodeConnectorServiceWriteDiagnosticAtomic = originalDiagnosticWriter
	restarted := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	recovered := mustRecordNodeConnectorServiceDiagnostic(t, restarted, diagnosticFixture)
	if recovered.LifecycleIntentFingerprint != intent.IntentFingerprint {
		t.Fatal("restart did not recover the exact diagnostic bound to the durable intent")
	}

	diagnosticOnlyRoot := t.TempDir()
	diagnosticRaw := mustReadNodeConnectorServiceFile(t, filepath.Join(root, nodeConnectorServiceDiagnosticName))
	if err := os.WriteFile(filepath.Join(diagnosticOnlyRoot, nodeConnectorServiceDiagnosticName), diagnosticRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorServiceLifecycle(diagnosticOnlyRoot, expected); err == nil {
		t.Fatal("diagnostic artifact without its exact durable intent was accepted")
	}
}

func TestNodeConnectorServiceLifecycleJSONShapeAndNoForbiddenCallbacks(t *testing.T) {
	root, expected, fixture := nodeConnectorServiceLifecycleFixture(t, NodeConnectorServicePlatformProfile{HostOS: "windows", ServiceManager: "windows_scm", InstallScope: "machine"}, "start")
	lifecycle := mustOpenNodeConnectorServiceLifecycle(t, root, expected)
	intent := mustRecordNodeConnectorServiceIntent(t, lifecycle, fixture)
	diagnostic := mustRecordNodeConnectorServiceDiagnostic(t, lifecycle, nodeConnectorServiceDiagnosticFixture(intent, expected, "failed", "unhealthy", "startup_timeout"))
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServicePlatformProfile{}, []string{"host_os", "service_manager", "install_scope"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceTarget{}, []string{"service_id", "machine_id", "connector_artifact_fingerprint", "service_configuration_fingerprint", "platform"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceDiagnosticReference{}, []string{"name", "media_type", "digest", "bytes"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceLifecycleIntentFixture{}, []string{"schema", "intent_id", "replay_id", "target", "intent", "provenance"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceLifecycleIntent{}, []string{"schema", "intent_id", "replay_id", "target", "intent", "provenance", "installer_invoked", "service_manager_invoked", "process_invoked", "mutation_performed", "lifecycle_transition_completed", "future_operation_authorized", "authority", "intent_fingerprint"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceDiagnosticFixture{}, []string{"schema", "diagnostic_id", "replay_id", "lifecycle_intent_id", "lifecycle_intent_fingerprint", "target", "service_state", "health", "failure_class", "observed_at", "references", "provenance"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceDiagnostic{}, []string{"schema", "diagnostic_id", "replay_id", "lifecycle_intent_id", "lifecycle_intent_fingerprint", "target", "service_state", "health", "failure_class", "observed_at", "references", "provenance", "probe_invoked", "lifecycle_intent_executed", "lifecycle_transition_proven", "connector_presence_proven", "session_health_proven", "execution_readiness_proven", "lease_authority_granted", "completion_proven", "authority", "diagnostic_fingerprint"})
	nodeConnectorServiceAssertJSONFields(t, NodeConnectorServiceAuthority{}, []string{"installer", "service_manager", "process", "diagnostic_probe", "execution", "validation", "scheduling", "transport", "provider", "network", "mutation", "repair", "git", "apply", "checkpoint", "commit", "push", "publication", "completion", "lifecycle"})
	raw, err := json.Marshal(struct {
		Intent     NodeConnectorServiceLifecycleIntent `json:"intent"`
		Diagnostic NodeConnectorServiceDiagnostic      `json:"diagnostic"`
	}{Intent: intent, Diagnostic: diagnostic})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"executable_path", "unit_file", "registry_path", "service_account", "credential_reference", "environment_variables", "argument_list", "socket", "hostname", "endpoint", "workspace_path", "raw_log", "stdout", "stderr", "provider_payload"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("service lifecycle evidence leaked forbidden raw or authority-bearing field %q", forbidden)
		}
	}
	installerCalls, serviceManagerCalls, processCalls, probeCalls, executorCalls, validatorCalls, schedulerCalls, transportCalls, providerCalls, networkCalls, gitCalls := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	_ = []func(){func() { installerCalls++ }, func() { serviceManagerCalls++ }, func() { processCalls++ }, func() { probeCalls++ }, func() { executorCalls++ }, func() { validatorCalls++ }, func() { schedulerCalls++ }, func() { transportCalls++ }, func() { providerCalls++ }, func() { networkCalls++ }, func() { gitCalls++ }}
	if installerCalls+serviceManagerCalls+processCalls+probeCalls+executorCalls+validatorCalls+schedulerCalls+transportCalls+providerCalls+networkCalls+gitCalls != 0 {
		t.Fatal("fixture-only service lifecycle invoked a forbidden callback")
	}
}

func TestNodeConnectorServiceLifecycleExistingContractSchemasRemainUnchanged(t *testing.T) {
	expected := map[string]string{
		"machine":         NodeExecutionMachineIdentitySchema,
		"capability":      NodeExecutionCapabilitySnapshotSchema,
		"session":         NodeConnectorSessionNegotiationSchema,
		"lease":           NodeExecutionLeaseSchema,
		"receipt":         NodeExecutionReceiptSchema,
		"aggregate":       NodeConnectorMultiTargetValidationAggregateSchema,
		"repair_decision": NodeConnectorMultiTargetRepairDecisionSchema,
		"repair_request":  NodeConnectorMultiTargetRepairRequestSchema,
	}
	want := map[string]string{
		"machine":         "dorkpipe.node-execution.machine-identity/v1",
		"capability":      "dorkpipe.node-execution.capability-snapshot/v1",
		"session":         "dorkpipe.node-connector.session-negotiation/v1",
		"lease":           "dorkpipe.node-execution.task-lease/v1",
		"receipt":         "dorkpipe.node-execution.execution-receipt/v1",
		"aggregate":       "dorkpipe.multi-target-validation-aggregate/v1",
		"repair_decision": "dorkpipe.multi-target-repair-decision/v1",
		"repair_request":  "dorkpipe.multi-target-repair-request/v1",
	}
	if !reflect.DeepEqual(expected, want) {
		t.Fatalf("an existing machine, capability, session, lease, receipt, aggregate, or repair schema changed: %#v", expected)
	}
}

func nodeConnectorServiceLifecycleFixture(t *testing.T, profile NodeConnectorServicePlatformProfile, intent string) (string, NodeConnectorServiceLifecycleExpected, NodeConnectorServiceLifecycleIntentFixture) {
	t.Helper()
	expected := NodeConnectorServiceLifecycleExpected{
		Target: NodeConnectorServiceTarget{
			ServiceID: "service-node-connector-001", MachineID: "machine-node-connector-001",
			ConnectorArtifactFingerprint: nodeConnectorServiceFingerprint("a"), ServiceConfigurationFingerprint: nodeConnectorServiceFingerprint("b"), Platform: profile,
		},
		DiagnosticReferences: []NodeConnectorServiceDiagnosticReference{
			{Name: "diagnostic-summary-one", MediaType: "application/json", Digest: nodeConnectorServiceFingerprint("1"), Bytes: 1024},
			{Name: "diagnostic-summary-two", MediaType: "application/vnd.dorkpipe.diagnostic+json", Digest: nodeConnectorServiceFingerprint("2"), Bytes: 2048},
		},
	}
	fixture := NodeConnectorServiceLifecycleIntentFixture{
		Schema: NodeConnectorServiceLifecycleIntentFixtureSchema, IntentID: "intent-service-lifecycle-001", ReplayID: "replay-service-lifecycle-001",
		Target: expected.Target, Intent: intent, Provenance: nodeConnectorServiceLifecycleIntentProvenance,
	}
	return t.TempDir(), expected, fixture
}

func nodeConnectorServiceDiagnosticFixture(intent NodeConnectorServiceLifecycleIntent, expected NodeConnectorServiceLifecycleExpected, state, health, failure string) NodeConnectorServiceDiagnosticFixture {
	return NodeConnectorServiceDiagnosticFixture{
		Schema: NodeConnectorServiceDiagnosticFixtureSchema, DiagnosticID: "diagnostic-service-lifecycle-001", ReplayID: "replay-service-diagnostic-001",
		LifecycleIntentID: intent.IntentID, LifecycleIntentFingerprint: intent.IntentFingerprint, Target: expected.Target,
		ServiceState: state, Health: health, FailureClass: failure, ObservedAt: "2026-07-27T16:00:00Z",
		References: cloneNodeConnectorServiceDiagnosticReferences(expected.DiagnosticReferences), Provenance: nodeConnectorServiceDiagnosticProvenance,
	}
}

func mustOpenNodeConnectorServiceLifecycle(t *testing.T, root string, expected NodeConnectorServiceLifecycleExpected) *NodeConnectorServiceLifecycle {
	t.Helper()
	lifecycle, err := OpenNodeConnectorServiceLifecycle(root, expected)
	if err != nil {
		t.Fatal(err)
	}
	return lifecycle
}

func mustRecordNodeConnectorServiceIntent(t *testing.T, lifecycle *NodeConnectorServiceLifecycle, fixture NodeConnectorServiceLifecycleIntentFixture) NodeConnectorServiceLifecycleIntent {
	t.Helper()
	intent, err := lifecycle.RecordIntent(mustMarshalNodeConnectorService(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return intent
}

func mustRecordNodeConnectorServiceDiagnostic(t *testing.T, lifecycle *NodeConnectorServiceLifecycle, fixture NodeConnectorServiceDiagnosticFixture) NodeConnectorServiceDiagnostic {
	t.Helper()
	diagnostic, err := lifecycle.RecordDiagnostic(mustMarshalNodeConnectorService(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return diagnostic
}

func mustMarshalNodeConnectorService(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorServiceFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorServiceDiagnosticFixture(value NodeConnectorServiceDiagnosticFixture) NodeConnectorServiceDiagnosticFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorServiceDiagnosticFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func nodeConnectorServiceAssertJSONFields(t *testing.T, value any, expected []string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	actual := make([]string, 0, len(fields))
	for field := range fields {
		actual = append(actual, field)
	}
	sortStrings(actual)
	want := append([]string{}, expected...)
	sortStrings(want)
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("JSON field shape mismatch: got %v want %v", actual, want)
	}
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}

func nodeConnectorServiceFingerprint(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}
