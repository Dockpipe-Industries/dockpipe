package applicationir

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreir"
)

type reviewApplicationFixture struct {
	semantic *pipelang.SemanticProjection
	core     coreir.Program
	spec     Spec
	types    map[string]LocatedIdentity
}

func loadReviewApplicationFixture(t *testing.T) reviewApplicationFixture {
	t.Helper()
	source, err := os.ReadFile("testdata/docker-observability.pipe")
	if err != nil {
		t.Fatal(err)
	}
	module := pipelang.ModuleInput{ID: "app.root", Namespace: "app.root", DeclarationSpan: pipelang.Span{File: "docker-observability.pipe"}, Sources: []pipelang.SourceInput{{Path: "docker-observability.pipe", Data: source}}}
	input := pipelang.ModuleSetInput{LanguageContract: pipelang.PipeLangLanguageContractV360, PackageID: "docker.observability", Root: "app.root", Modules: []pipelang.ModuleInput{module}}
	input.Lock.Modules = []pipelang.LockedModule{{ID: module.ID, SourceSHA256: pipelang.ModuleSourceSHA256(module.Sources), SemanticSHA256: pipelang.ModuleSemanticSHA256(input.PackageID, module.Namespace, nil)}}
	analysis := pipelang.AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	semantic, err := pipelang.BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	members := map[string]pipelang.SemanticMemberProjection{}
	types := map[string]LocatedIdentity{}
	for _, module := range semantic.Modules {
		for _, typ := range module.Types {
			if typ.Identity != nil {
				types[typ.Name] = LocatedIdentity{Identity: Identity{PackageID: string(typ.Identity.PackageID), Path: string(typ.Identity.Path)}}
			}
			for _, member := range typ.Members {
				members[member.Name] = member
			}
		}
	}
	var core coreir.Program
	for _, name := range []string{"Project", "Containers", "Networks", "Volumes", "Selection", "Details", "Logs", "FilterContainers", "OrderContainers", "OrderNetworks", "OrderVolumes"} {
		member := members[name]
		typed, err := pipelang.LowerSemanticMethodToHIR(analysis, *member.Identity)
		if err != nil {
			t.Fatal(err)
		}
		program, err := pipelang.LowerHIRToCore(typed)
		if err != nil {
			t.Fatal(err)
		}
		if core.CompilerContract == "" {
			core = program
		} else {
			core.Functions = append(core.Functions, program.Functions...)
		}
	}
	raw, err := os.ReadFile("testdata/docker-observability.spec.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	return reviewApplicationFixture{semantic: semantic, core: core, spec: spec, types: types}
}

func TestReviewCheckedSpecProjectsCheckedGolden(t *testing.T) {
	fixture := loadReviewApplicationFixture(t)
	app, err := Project(fixture.semantic, &fixture.core, fixture.spec)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalJSON(app)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/docker-observability.application.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("decoded checked-in spec did not produce checked-in Application IR")
	}
}

func TestReviewProjectRejectsCrossIdentityAndMetadataDrift(t *testing.T) {
	t.Run("snapshot is class not record", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		fixture.spec.SnapshotType = fixture.types["DockerApplication"]
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted non-record SnapshotType")
		}
	})
	t.Run("snapshot record unrelated to application function", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		fixture.spec.SnapshotType = fixture.types["ContainerRow"]
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted SnapshotType unrelated to application function")
		}
	})
	t.Run("selection record absent from sections", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		fixture.spec.Sections = append([]SectionSpec(nil), fixture.spec.Sections[1:]...)
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted Optional selection for an unrelated row type")
		}
	})
	t.Run("section identity owned by unrelated type", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		fixture.spec.Sections[0].Identity = fixture.types["ContainerRow"]
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted section identity with mismatched ownership")
		}
	})
	t.Run("section identity disagrees with result binding", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		fixture.spec.Sections[0].Identity = *fixture.spec.Details
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted section identity unrelated to its result binding")
		}
	})
	t.Run("Core callable signature disagrees with semantic", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		filter := fixture.spec.Sections[0].Filter.Identity
		for i := range fixture.core.Functions {
			if fixture.core.Functions[i].Identity.PackageID == filter.PackageID && fixture.core.Functions[i].Identity.Path == filter.Path {
				fixture.core.Functions[i].Parameters = nil
			}
		}
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted semantic/Core callable signature mismatch")
		}
	})
	t.Run("order metadata disagrees with Core selector", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		binding := fixture.spec.Sections[0].OrderBinding.Identity
		for i := range fixture.core.Functions {
			function := &fixture.core.Functions[i]
			if function.Identity.PackageID == binding.PackageID && function.Identity.Path == binding.Path && function.Body.ListSortByOrdinalText != nil {
				function.Body.ListSortByOrdinalText.Position = 2
				function.Body.ListSortByOrdinalText.Name = "State"
				function.Body.ListSortByOrdinalText.Field = function.Parameters[0].Type.List.Element.Record.Fields[2].Identity
			}
		}
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted order metadata inconsistent with bound Core callable")
		}
	})
	t.Run("filter metadata disagrees with Core selectors", func(t *testing.T) {
		fixture := loadReviewApplicationFixture(t)
		binding := fixture.spec.Sections[0].Filter.Identity
		for i := range fixture.core.Functions {
			function := &fixture.core.Functions[i]
			if function.Identity.PackageID == binding.PackageID && function.Identity.Path == binding.Path && function.Body.ListFilterJoinedContainsCaseFolded != nil {
				field := function.Parameters[0].Type.List.Element.Record.Fields[0]
				function.Body.ListFilterJoinedContainsCaseFolded.Selectors[0].Field = field.Identity
				function.Body.ListFilterJoinedContainsCaseFolded.Selectors[0].Name = field.Name
				function.Body.ListFilterJoinedContainsCaseFolded.Selectors[0].Position = 0
			}
		}
		if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
			t.Error("accepted filter metadata inconsistent with bound Core callable")
		}
	})
}

func TestReviewProjectRejectsDuplicateMetadata(t *testing.T) {
	cases := map[string]func(*Spec){
		"columns": func(spec *Spec) {
			spec.Sections[0].Columns = append(spec.Sections[0].Columns, spec.Sections[0].Columns[0])
		},
		"filter fields": func(spec *Spec) {
			spec.Sections[0].FilterFields = append(spec.Sections[0].FilterFields, spec.Sections[0].FilterFields[0])
		},
		"order keys": func(spec *Spec) {
			spec.Sections[0].Order = append(spec.Sections[0].Order, spec.Sections[0].Order[0])
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := loadReviewApplicationFixture(t)
			mutate(&fixture.spec)
			if _, err := Project(fixture.semantic, &fixture.core, fixture.spec); err == nil {
				t.Error("accepted duplicate " + name)
			}
		})
	}
}
