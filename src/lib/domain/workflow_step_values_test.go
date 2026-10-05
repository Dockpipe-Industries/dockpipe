package domain

import "testing"

func TestWorkflowStepValueConstantsMatchWireVocabulary(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "container kind", got: string(StepKindContainer), want: "container"},
		{name: "host kind", got: string(StepKindHost), want: "host"},
		{name: "source scope", got: string(StepPathScopeSource), want: "source"},
		{name: "artifact scope", got: string(StepPathScopeArtifacts), want: "artifacts"},
		{name: "package build builtin", got: string(StepHostBuiltinPackageBuildStore), want: "package_build_store"},
		{name: "compose up builtin", got: string(StepHostBuiltinComposeUp), want: "compose_up"},
		{name: "compose down builtin", got: string(StepHostBuiltinComposeDown), want: "compose_down"},
		{name: "compose ps builtin", got: string(StepHostBuiltinComposePS), want: "compose_ps"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q want %q", test.got, test.want)
			}
		})
	}
}

func TestWorkflowStepWireFieldsAndCompatibilityHelpersRemainStrings(t *testing.T) {
	var kind string = (Step{}).Kind
	var cwd string = (Step{}).CWD
	var source string = (StepScopes{}).Source
	var artifacts string = (StepScopes{}).Artifacts
	var builtin string = (Step{}).HostBuiltin
	var kindName string = (&Step{}).KindName()
	var cwdMode string = (&Step{}).CWDMode()
	var sourceMode string = (&Step{}).SourceScopeMode()
	var artifactsMode string = (&Step{}).ArtifactsScopeMode()
	_ = []string{kind, cwd, source, artifacts, builtin, kindName, cwdMode, sourceMode, artifactsMode}
}

func TestNormalizeStepKindPreservesDefaultsAndUnknownValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  StepKind
		valid bool
	}{
		{name: "default", want: StepKindContainer, valid: true},
		{name: "trimmed host", value: " HOST ", want: StepKindHost, valid: true},
		{name: "container", value: "container", want: StepKindContainer, valid: true},
		{name: "unknown preserved", value: " Future ", want: "future", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NormalizeStepKind(test.value)
			if got != test.want || got.IsValid() != test.valid {
				t.Fatalf("NormalizeStepKind(%q) = %q valid=%t, want %q valid=%t", test.value, got, got.IsValid(), test.want, test.valid)
			}
		})
	}
}

func TestStepPathScopesPreserveDefaultsAliasesAndUnknownValues(t *testing.T) {
	tests := []struct {
		name string
		step Step
		cwd  StepPathScope
		src  StepPathScope
		art  StepPathScope
	}{
		{name: "defaults", step: Step{}, cwd: StepPathScopeSource, src: StepPathScopeSource, art: StepPathScopeArtifacts},
		{name: "aliases", step: Step{CWD: " WorkDir ", Scopes: StepScopes{Source: " Repo ", Artifacts: " Artifact "}}, cwd: StepPathScopeSource, src: StepPathScopeSource, art: StepPathScopeArtifacts},
		{name: "unknown preserved", step: Step{CWD: " Future ", Scopes: StepScopes{Source: " Left ", Artifacts: " Right "}}, cwd: "future", src: "left", art: "right"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.step.EffectiveCWD(); got != test.cwd {
				t.Fatalf("EffectiveCWD = %q want %q", got, test.cwd)
			}
			if got := test.step.EffectiveSourceScope(); got != test.src {
				t.Fatalf("EffectiveSourceScope = %q want %q", got, test.src)
			}
			if got := test.step.EffectiveArtifactsScope(); got != test.art {
				t.Fatalf("EffectiveArtifactsScope = %q want %q", got, test.art)
			}
		})
	}
}

func TestStepHostBuiltinOwnsValidationAndComposeBehavior(t *testing.T) {
	tests := []struct {
		value        string
		want         StepHostBuiltin
		valid        bool
		needsCompose bool
		action       string
	}{
		{value: " package_build_store ", want: StepHostBuiltinPackageBuildStore, valid: true, action: "package_build_store"},
		{value: " compose_up ", want: StepHostBuiltinComposeUp, valid: true, needsCompose: true, action: "up"},
		{value: "compose_down", want: StepHostBuiltinComposeDown, valid: true, needsCompose: true, action: "down"},
		{value: "compose_ps", want: StepHostBuiltinComposePS, valid: true, needsCompose: true, action: "ps"},
		{value: " Compose_Up ", want: "Compose_Up", action: "Compose_Up"},
		{value: " future ", want: "future", action: "future"},
	}
	for _, test := range tests {
		got := NormalizeStepHostBuiltin(test.value)
		if got != test.want || got.IsValid() != test.valid || got.NeedsCompose() != test.needsCompose || got.NeedsDocker() != test.needsCompose || got.ComposeAction() != test.action {
			t.Fatalf("NormalizeStepHostBuiltin(%q) = %q valid=%t compose=%t action=%q", test.value, got, got.IsValid(), got.NeedsCompose(), got.ComposeAction())
		}
	}
}

func TestWorkflowStepValidationDoesNotMutateWireValues(t *testing.T) {
	step := Step{
		Kind:        " HOST ",
		CWD:         " Repo ",
		Scopes:      StepScopes{Source: " WorkDir ", Artifacts: " Artifact "},
		HostBuiltin: " compose_up ",
	}
	if err := ValidateStepKind(0, step); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStepCWD(0, step); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStepScopes(0, step); err != nil {
		t.Fatal(err)
	}
	if err := ValidateStepHostBuiltin(0, step); err != nil {
		t.Fatal(err)
	}
	if step.Kind != " HOST " || step.CWD != " Repo " || step.Scopes.Source != " WorkDir " || step.Scopes.Artifacts != " Artifact " || step.HostBuiltin != " compose_up " {
		t.Fatalf("validation mutated authored step values: %+v", step)
	}
}

func TestWorkflowStepUnknownValidationErrorsRemainCompatible(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "kind", err: ValidateStepKind(1, Step{Kind: " Future "}), want: "step 2: kind must be host or container"},
		{name: "cwd", err: ValidateStepCWD(1, Step{CWD: " Future "}), want: "step 2: cwd must be source, repo, or artifacts"},
		{name: "source scope", err: ValidateStepScopes(1, Step{Scopes: StepScopes{Source: " Future "}}), want: "step 2: scopes.source must be source, repo, or artifacts"},
		{name: "host builtin", err: ValidateStepHostBuiltin(1, Step{Kind: "host", HostBuiltin: " Future "}), want: "step 2: unknown host_builtin \"Future\" (allowed: package_build_store, compose_up, compose_down, compose_ps)"},
		{name: "compose requirement", err: ValidateStepComposeBuiltin(1, Step{HostBuiltin: " compose_up "}, nil), want: "step 2: host_builtin \"compose_up\" requires workflow compose.file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err == nil || test.err.Error() != test.want {
				t.Fatalf("error = %v, want %q", test.err, test.want)
			}
		})
	}
}
