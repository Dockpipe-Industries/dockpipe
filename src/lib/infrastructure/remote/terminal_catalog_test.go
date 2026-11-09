package remote

import (
	"encoding/json"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestTerminalHeartbeatPreservesOnlyMatchingFeatureRecoveryCatalog(t *testing.T) {
	identity, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	reference := containerTerminalTestReference()
	reference.Target.Fingerprint = remoteidentity.Fingerprint(identity.PublicKey)
	orphan := reference
	orphan.Target.Generation = strings.Repeat("9", 64)
	recovery := contract.ContainerTerminalReference{Reference: orphan, Container: strings.Repeat("e", 64), Phase: "running"}
	catalog := contract.FeatureCatalog{Schema: contract.FeatureVersion, Target: reference.Target, DockerRead: true, DockerExec: true,
		ContainerTerminals: []contract.ContainerTerminalReference{recovery}}
	broker := &Broker{state: BrokerState{Nodes: map[string]Enrollment{reference.Target.Node: {MachineKey: identity.PublicKey}}},
		features: &featureBroker{catalogs: map[string]contract.FeatureCatalog{}}}
	publishFeature := func(value contract.FeatureCatalog) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := broker.featureWorker(reference.Target.Node, "/v1/features-next", raw); err != nil {
			t.Fatal(err)
		}
	}
	heartbeat := func(value contract.FeatureCatalog) {
		t.Helper()
		exchange := contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: broker.terminalState().epoch, Catalog: value}
		raw, err := json.Marshal(exchange)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := broker.terminalWorker(reference.Target.Node, raw); err != nil {
			t.Fatal(err)
		}
	}
	publishFeature(catalog)
	previous := broker.features.catalogs[reference.Target.Node]
	heartbeatCatalog := catalog
	heartbeatCatalog.ContainerTerminals = nil
	heartbeat(heartbeatCatalog)
	retained := broker.features.catalogs[reference.Target.Node].ContainerTerminals
	if len(retained) != 1 || retained[0] != recovery {
		t.Fatal("matching terminal heartbeat erased the original recovery reference")
	}
	previous.ContainerTerminals[0].Phase = "changed"
	if retained[0] != recovery {
		t.Fatal("terminal heartbeat retained an aliased recovery slice")
	}
	heartbeatCatalog.Target.Generation = strings.Repeat("8", 64)
	// A terminal worker cannot carry old discovery into a new generation,
	// including by echoing recovery entries in its own exchange.
	heartbeatCatalog.ContainerTerminals = []contract.ContainerTerminalReference{recovery}
	heartbeat(heartbeatCatalog)
	if len(broker.features.catalogs[reference.Target.Node].ContainerTerminals) != 0 {
		t.Fatal("terminal heartbeat imported recovery discovery across generations")
	}
	catalog.Target = heartbeatCatalog.Target
	publishFeature(catalog)
	heartbeatCatalog.ContainerTerminals = nil
	heartbeat(heartbeatCatalog)
	if len(broker.features.catalogs[reference.Target.Node].ContainerTerminals) != 1 {
		t.Fatal("fresh feature publication could not restore durable orphan discovery")
	}
	catalog.ContainerTerminals = nil
	publishFeature(catalog)
	heartbeat(heartbeatCatalog)
	if len(broker.features.catalogs[reference.Target.Node].ContainerTerminals) != 0 {
		t.Fatal("terminal heartbeat restored discovery cleared by the feature worker")
	}
}
