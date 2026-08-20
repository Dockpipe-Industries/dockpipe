package applicationir

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestDockerObservabilityGoldenUsesCanonicalSemanticAndCore(t *testing.T) {
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
	find := func(name string) pipelang.SemanticMemberProjection {
		for _, m := range semantic.Modules {
			for _, typ := range m.Types {
				for _, x := range typ.Members {
					if x.Name == name {
						return x
					}
				}
			}
		}
		t.Fatalf("missing member %s", name)
		return pipelang.SemanticMemberProjection{}
	}
	visible := find("VisibleContainers")
	visibleHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *visible.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(visibleHIR.Functions) != 3 || visibleHIR.Functions[2].Body.Kind != hir.ExprCall {
		t.Fatalf("VisibleContainers HIR is not a closed three-function call graph: %#v", visibleHIR)
	}
	visibleCore, err := pipelang.LowerHIRToCore(visibleHIR)
	if err != nil {
		t.Fatal(err)
	}
	visibleGo, err := gobackend.Generate(visibleCore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(visibleGo), "PipeLangOrderContainers(PipeLangFilterContainers(") {
		t.Fatalf("VisibleContainers generated Go lost validated composition:\n%s", visibleGo)
	}
	var visibleFunction coreir.Function
	for _, function := range visibleCore.Functions {
		if function.Name == "VisibleContainers" {
			visibleFunction = function
		}
	}
	listType := visibleFunction.Parameters[0].Type
	rowType := listType.List.Element
	textType := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	row := func(values ...string) coreeval.Value {
		fields := make([]coreeval.Value, len(values))
		for index, value := range values {
			fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
		}
		return coreeval.Value{Type: rowType, Record: fields}
	}
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		row("2", "beta", "running", "Up", "dockpipe:b", "", "later"),
		row("1", "Alpha", "running", "Up", "dockpipe:a", "", "earlier"),
	}}
	outcome, err := coreeval.EvaluateProgram(visibleCore, coreir.SemanticIdentity{PackageID: string(visible.Identity.PackageID), Path: string(visible.Identity.Path)}, []coreeval.Value{rows, {Type: textType, String: ""}})
	if err != nil || !outcome.OK || len(outcome.Value.List) != 2 || outcome.Value.List[0].Record[1].String != "Alpha" {
		t.Fatalf("VisibleContainers consumer result = %#v, %v", outcome, err)
	}
	typeID := func(name string) Identity {
		for _, m := range semantic.Modules {
			for _, x := range m.Types {
				if x.Name == name && x.Identity != nil {
					return Identity{string(x.Identity.PackageID), string(x.Identity.Path)}
				}
			}
		}
		t.Fatalf("missing type %s", name)
		return Identity{}
	}
	member := func(name string) LocatedIdentity {
		x := find(name)
		return LocatedIdentity{Identity: Identity{string(x.Identity.PackageID), string(x.Identity.Path)}, Source: x.Declaration}
	}
	project := find("Project")
	typed, err := pipelang.LowerSemanticMethodToHIR(analysis, *project.Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := pipelang.LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Containers", "Networks", "Volumes", "Selection", "Details", "Logs", "FilterContainers", "OrderContainers", "OrderNetworks", "OrderVolumes"} {
		m := find(name)
		h, err := pipelang.LowerSemanticMethodToHIR(analysis, *m.Identity)
		if err != nil {
			t.Fatal(err)
		}
		p, err := pipelang.LowerHIRToCore(h)
		if err != nil {
			t.Fatal(err)
		}
		core.Functions = append(core.Functions, p.Functions...)
	}
	locType := func(name string) LocatedIdentity { return LocatedIdentity{Identity: typeID(name)} }
	spec := Spec{Identity: LocatedIdentity{Identity: Identity{string(project.Identity.PackageID), string(project.Identity.Path)}, Source: project.Declaration}, SnapshotType: locType("DockerSnapshot"),
		Sections: []SectionSpec{
			section(member("Containers"), locType("ContainerRow"), member("Id"), []string{"Name", "State", "Image", "Ports", "Created"}, []string{"Name", "State", "Image", "Ports", "Created"}),
			section(member("Networks"), locType("NetworkRow"), member("Id"), []string{"Name", "Driver", "Scope"}, nil),
			section(member("Volumes"), locType("VolumeRow"), member("Name"), []string{"Name", "Driver", "Mountpoint"}, nil),
		}, Selection: ptr(member("Selection")), Details: ptr(member("Details")), Logs: ptr(member("Logs"))}
	spec.Sections[0].Filter = ptr(member("FilterContainers"))
	spec.Sections[0].OrderBinding = member("OrderContainers")
	spec.Sections[1].OrderBinding = member("OrderNetworks")
	spec.Sections[2].OrderBinding = member("OrderVolumes")
	// Resolve duplicate field names to the field owned by each row type.
	for i, rowName := range []string{"ContainerRow", "NetworkRow", "VolumeRow"} {
		row := typeID(rowName)
		keyNames := []string{"Id", "Id", "Name"}
		for j := range spec.Sections[i].Columns {
			spec.Sections[i].Columns[j].Field = ownedField(semantic, row, spec.Sections[i].Columns[j].Label)
		}
		spec.Sections[i].Key = ownedField(semantic, row, keyNames[i])
		for j := range spec.Sections[i].FilterFields {
			spec.Sections[i].FilterFields[j] = ownedField(semantic, row, spec.Sections[i].FilterFields[j].Path)
		}
		for j := range spec.Sections[i].Order {
			spec.Sections[i].Order[j].Field = spec.Sections[i].Columns[0].Field
		}
		if spec.Sections[i].Key.Path == "" {
			t.Fatalf("missing resolved key for %s row=%#v", rowName, row)
		}
	}
	if os.Getenv("UPDATE_APPLICATIONIR_GOLDEN") != "" {
		b, _ := json.MarshalIndent(spec, "", "  ")
		_ = os.WriteFile("testdata/docker-observability.spec.json", b, 0644)
	}
	app, err := Project(semantic, &core, spec)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CanonicalJSON(app)
	if err != nil {
		t.Fatal(err)
	}
	golden := "testdata/docker-observability.application.json"
	if os.Getenv("UPDATE_APPLICATIONIR_GOLDEN") != "" {
		if err := os.WriteFile(golden, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("application fixture changed\n%s", got)
	}
	var checked Spec
	raw, err := os.ReadFile("testdata/docker-observability.spec.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &checked); err != nil {
		t.Fatal(err)
	}
	if len(checked.Sections) != 3 || len(app.Sections) != 3 || app.Selection == nil || app.Details == nil || app.Logs == nil || app.Metadata.LanguageContract != "v0.36.0" {
		t.Fatalf("incomplete fixture: %#v", app)
	}
	bad := spec
	bad.Sections = append([]SectionSpec(nil), spec.Sections...)
	bad.Sections[0].ResultType = bad.Sections[0].Key
	if _, err := Project(semantic, &core, bad); err == nil {
		t.Fatal("accepted section without Result<List<Row>, string> signature")
	} else if located, ok := err.(*ValidationError); !ok || located.Source.File != bad.Sections[0].Key.Source.File {
		t.Fatalf("structured rejection lost source: %v", err)
	}
	mismatch := core
	mismatch.LanguageContract = "v0.34.0"
	if _, err := Project(semantic, &mismatch, spec); err == nil {
		t.Fatal("accepted mismatched canonical Core")
	}
}

func section(result LocatedIdentity, row LocatedIdentity, key LocatedIdentity, columns, filters []string) SectionSpec {
	s := SectionSpec{Identity: result, ResultType: result, RowType: row, Key: key, Columns: []Column{}, FilterFields: []LocatedIdentity{}, Order: []OrderKey{}}
	for _, x := range columns {
		s.Columns = append(s.Columns, Column{Field: LocatedIdentity{Identity: Identity{Path: x}}, Label: x})
	}
	for _, x := range filters {
		s.FilterFields = append(s.FilterFields, LocatedIdentity{Identity: Identity{Path: x}})
	}
	s.Order = []OrderKey{{Field: LocatedIdentity{}, Direction: "ascending"}}
	return s
}
func ptr(v LocatedIdentity) *LocatedIdentity { return &v }
func ownedField(p *pipelang.SemanticProjection, row Identity, name string) LocatedIdentity {
	for _, m := range p.Modules {
		for _, typ := range m.Types {
			if typ.Identity != nil && string(typ.Identity.Path) == row.Path {
				for _, x := range typ.Members {
					if x.Name == name {
						return LocatedIdentity{Identity: Identity{string(x.Identity.PackageID), string(x.Identity.Path)}, Source: x.Declaration}
					}
				}
			}
		}
	}
	return LocatedIdentity{}
}

func TestProjectRejectsUnknownIdentityAtItsSource(t *testing.T) {
	_, err := Project(&pipelang.SemanticProjection{Schema: pipelang.PipeLangSemanticProjectionVersion, CompilerContract: pipelang.PipeLangCompilerContract, LanguageContract: pipelang.PipeLangLanguageContractV360, View: pipelang.SemanticProjectionPublic}, &coreir.Program{CompilerContract: pipelang.PipeLangCompilerContract, LanguageContract: "v0.36.0"}, Spec{Identity: LocatedIdentity{Identity: Identity{"p", "missing"}, Source: pipelang.SourceRange{File: "model.pipe"}}})
	v, ok := err.(*ValidationError)
	if !ok || v.Source.File != "model.pipe" {
		t.Fatalf("expected located rejection, got %v", err)
	}
}

func TestProductionPackageHasNoParserEvaluatorOrTargetImports(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("application.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte("coreeval"), []byte("gobackend"), []byte("src/app"), []byte("parser")} {
		if bytes.Contains(data, bad) {
			t.Fatalf("forbidden production dependency %q", bad)
		}
	}
}
