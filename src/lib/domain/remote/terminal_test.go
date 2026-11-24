package remote

import (
	"strings"
	"testing"
	"time"
)

func validTerminalIntent() TerminalOpen {
	request := TerminalOpen{Schema: TerminalVersion, Target: FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Root: "owner", Nonce: strings.Repeat("d", 64), Deadline: time.Now().UTC().Add(time.Minute), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	return request
}

func TestTerminalIntentBindsEpochTargetRootSizeAndDeadline(t *testing.T) {
	request := validTerminalIntent()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*TerminalOpen){
		func(value *TerminalOpen) { value.Epoch = strings.Repeat("e", 64) },
		func(value *TerminalOpen) { value.Target.Generation = strings.Repeat("f", 64) },
		func(value *TerminalOpen) { value.Root = "different" },
		func(value *TerminalOpen) { value.Columns++ },
		func(value *TerminalOpen) { value.Deadline = value.Deadline.Add(time.Second) },
	} {
		changed := request
		mutate(&changed)
		if changed.Validate() == nil {
			t.Fatal("changed terminal intent retained original ID")
		}
	}
}

func TestTerminalScopesAndCatalogPublicationBoundary(t *testing.T) {
	if OperatorScope("/v1/terminal-read") != "remote.read" {
		t.Fatal("terminal read scope changed")
	}
	for _, action := range []string{"open", "input", "resize", "close", "release", "pulse"} {
		if OperatorScope("/v1/terminal-"+action) != "remote.execute" {
			t.Fatal("terminal write/renewal scope changed")
		}
	}
	if OperatorScope("/v1/terminal-worker") != "" || OperatorScope("/v1/terminal-exec") != "" {
		t.Fatal("worker or arbitrary command got operator authority")
	}
	catalog := FeatureCatalog{Schema: FeatureVersion, Target: validTerminalIntent().Target, Roots: []FeatureRoot{{ID: "owner", Name: "Owner", Terminal: true}}}
	if err := catalog.Validate(); err != nil {
		t.Fatal("worker publication needs no broker epoch", err)
	}
	if catalog.ValidateOperatorCatalog() == nil {
		t.Fatal("operator accepted terminal without epoch")
	}
	catalog.TerminalEpoch = strings.Repeat("a", 64)
	if err := catalog.ValidateOperatorCatalog(); err != nil {
		t.Fatal(err)
	}
}

func TestContainerTerminalIntentIsTypedAndBindsContainerAndShell(t *testing.T) {
	request := validTerminalIntent()
	request.Root, request.Container, request.Shell = "", strings.Repeat("e", 64), "/bin/sh"
	request.ID, _ = request.Digest()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*TerminalOpen){
		func(value *TerminalOpen) { value.Container = strings.Repeat("f", 64) },
		func(value *TerminalOpen) { value.Shell = "/bin/bash" },
	} {
		changed := request
		mutate(&changed)
		if changed.Validate() == nil {
			t.Fatal("changed container selection retained session identity")
		}
	}
	for _, invalid := range []TerminalOpen{
		{Root: "project", Container: request.Container, Shell: "/bin/sh"},
		{Container: "short", Shell: "/bin/sh"},
		{Container: request.Container, Shell: "/bin/sh -c arbitrary"},
	} {
		invalid.Schema, invalid.Target, invalid.Epoch, invalid.Nonce, invalid.Deadline, invalid.Columns, invalid.Rows = request.Schema, request.Target, request.Epoch, request.Nonce, request.Deadline, request.Columns, request.Rows
		invalid.ID, _ = invalid.Digest()
		if invalid.Validate() == nil {
			t.Fatal("invalid container exec selection accepted")
		}
	}
}
