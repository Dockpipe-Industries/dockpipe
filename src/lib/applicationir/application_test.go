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
	input := pipelang.ModuleSetInput{LanguageContract: pipelang.PipeLangLanguageContractV660, PackageID: "docker.observability", Root: "app.root", Modules: []pipelang.ModuleInput{module}}
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
	selectedName := find("SelectedName")
	selectedNameHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *selectedName.Identity)
	if err != nil {
		t.Fatal(err)
	}
	selectedNameFunction := selectedNameHIR.Functions[len(selectedNameHIR.Functions)-1]
	if selectedNameFunction.Body.Kind != hir.ExprMatch || selectedNameFunction.Body.Match.Arms[0].Body.Kind != hir.ExprCall {
		t.Fatalf("SelectedName HIR lost composed match-arm call: %#v", selectedNameFunction.Body)
	}
	selectedNameCore, err := pipelang.LowerHIRToCore(selectedNameHIR)
	if err != nil {
		t.Fatal(err)
	}
	selectedNameCoreFunction := selectedNameCore.Functions[len(selectedNameCore.Functions)-1]
	selectedRowType := selectedNameCoreFunction.Parameters[0].Type
	selectedPayload := row("3", "  worker  ", "running", "Up", "dockpipe:worker", "", "now")
	selectedValue := coreeval.Value{Type: selectedRowType, Optional: &coreeval.OptionalValue{Present: true, Value: &selectedPayload}}
	selectedOutcome, err := coreeval.EvaluateProgram(selectedNameCore, coreir.SemanticIdentity{PackageID: string(selectedName.Identity.PackageID), Path: string(selectedName.Identity.Path)}, []coreeval.Value{selectedValue})
	if err != nil || !selectedOutcome.OK || selectedOutcome.Value.String != "worker" {
		t.Fatalf("SelectedName consumer result = %#v, %v", selectedOutcome, err)
	}
	selectedGo, err := gobackend.Generate(selectedNameCore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(selectedGo), "return PipeLangNormalizeName(") {
		t.Fatalf("SelectedName generated Go lost match-arm call:\n%s", selectedGo)
	}
	resolveSelection := find("ResolveSelection")
	resolveSelectionHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *resolveSelection.Identity)
	if err != nil {
		t.Fatal(err)
	}
	resolveSelectionFunction := resolveSelectionHIR.Functions[len(resolveSelectionHIR.Functions)-1]
	resolveCarrier := resolveSelectionFunction.Body.ImmutableLocal
	if resolveSelectionHIR.LanguageContract != coreir.LanguageContractV660 || resolveCarrier == nil || resolveCarrier.Initializer.Kind != hir.ExprCall || resolveCarrier.Initializer.Call == nil || len(resolveCarrier.Initializer.Call.Arguments) != 2 || resolveCarrier.Return == nil || resolveCarrier.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("ResolveSelection HIR lost multi-parameter helper carrier: %#v", resolveSelectionFunction.Body)
	}
	for position, argument := range resolveCarrier.Initializer.Call.Arguments {
		if argument.Kind != hir.ExprReference || argument.Reference == nil || argument.Reference.Kind != hir.BindingParameter || argument.Reference.Position != position {
			t.Fatalf("ResolveSelection helper argument %d = %#v", position, argument)
		}
	}
	resolveSelected := resolveCarrier.Return.ImmutableLocal
	if resolveSelected == nil || resolveSelected.Initializer.Kind != hir.ExprPropagate || resolveSelected.Initializer.Propagate == nil || resolveSelected.Initializer.Propagate.Value == nil || resolveSelected.Initializer.Propagate.Value.Kind != hir.ExprReference || resolveSelected.Initializer.Propagate.Value.Reference == nil || resolveSelected.Initializer.Propagate.Value.Reference.Kind != hir.BindingLocal || resolveSelected.Initializer.Propagate.Value.Reference.Position != resolveCarrier.Binding.Position {
		t.Fatalf("ResolveSelection HIR lost adjacent propagation local: %#v", resolveSelectionFunction.Body)
	}
	resolveSelectionCore, err := pipelang.LowerHIRToCore(resolveSelectionHIR)
	if err != nil {
		t.Fatal(err)
	}
	resolveSelectionCoreFunction := resolveSelectionCore.Functions[len(resolveSelectionCore.Functions)-1]
	resolveEntry := coreir.SemanticIdentity{PackageID: string(resolveSelection.Identity.PackageID), Path: string(resolveSelection.Identity.Path)}
	resolveOutcome, err := coreeval.EvaluateProgram(resolveSelectionCore, resolveEntry, []coreeval.Value{rows, {Type: resolveSelectionCoreFunction.Parameters[1].Type, String: "2"}})
	if err != nil || !resolveOutcome.OK || resolveOutcome.Value.Optional == nil || !resolveOutcome.Value.Optional.Present || resolveOutcome.Value.Optional.Value.Record[1].String != "beta" {
		t.Fatalf("ResolveSelection present result = %#v, %v", resolveOutcome, err)
	}
	resolveOutcome, err = coreeval.EvaluateProgram(resolveSelectionCore, resolveEntry, []coreeval.Value{rows, {Type: resolveSelectionCoreFunction.Parameters[1].Type, String: "missing"}})
	if err != nil || !resolveOutcome.OK || resolveOutcome.Value.Optional == nil || resolveOutcome.Value.Optional.Present {
		t.Fatalf("ResolveSelection absent result = %#v, %v", resolveOutcome, err)
	}
	resolveSelectionGo, err := gobackend.Generate(resolveSelectionCore)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(resolveSelectionGo), " := PipeLangFindSelection(") != 1 || !strings.Contains(string(resolveSelectionGo), "pipelangCloneListDockerObservabilityAppRootContainerrow(p0), p1") || !strings.Contains(string(resolveSelectionGo), "pipelangPropagateOptional(") {
		t.Fatalf("ResolveSelection generated Go lost once-only multi-parameter propagation:\n%s", resolveSelectionGo)
	}
	selectedNameByID := find("SelectedNameById")
	selectedNameByIDHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *selectedNameByID.Identity)
	if err != nil {
		t.Fatal(err)
	}
	selectedNameByIDFunction := selectedNameByIDHIR.Functions[len(selectedNameByIDHIR.Functions)-1]
	selectedNameByIDFallback := selectedNameByIDFunction.Body.ImmutableLocal
	if selectedNameByIDFunction.Body.Kind != hir.ExprImmutableLocal || selectedNameByIDFallback == nil || selectedNameByIDFallback.Initializer.Kind != hir.ExprLiteral || selectedNameByIDFallback.Return == nil || selectedNameByIDFallback.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost fallback local: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDCarrier := selectedNameByIDFallback.Return.ImmutableLocal
	if selectedNameByIDCarrier == nil || selectedNameByIDCarrier.Return == nil || selectedNameByIDCarrier.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost carrier local: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDLocal := selectedNameByIDCarrier.Return.ImmutableLocal
	if selectedNameByIDCarrier.Initializer.Kind != hir.ExprCall || selectedNameByIDCarrier.Initializer.Call == nil || len(selectedNameByIDCarrier.Initializer.Call.Arguments) != 2 || selectedNameByIDLocal == nil || selectedNameByIDLocal.Initializer.Kind != hir.ExprMatch || selectedNameByIDLocal.Initializer.Match == nil || selectedNameByIDLocal.Initializer.Match.Value == nil || selectedNameByIDLocal.Initializer.Match.Value.Kind != hir.ExprReference || selectedNameByIDLocal.Initializer.Match.Value.Reference == nil || selectedNameByIDLocal.Initializer.Match.Value.Reference.Kind != hir.BindingLocal || selectedNameByIDLocal.Initializer.Match.Value.Reference.Position != selectedNameByIDCarrier.Binding.Position || selectedNameByIDLocal.Return == nil || selectedNameByIDLocal.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost first prior-local helper-carrier match: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDConfirmationCarrier := selectedNameByIDLocal.Return.ImmutableLocal
	if selectedNameByIDConfirmationCarrier == nil || selectedNameByIDConfirmationCarrier.Initializer.Kind != hir.ExprCall || selectedNameByIDConfirmationCarrier.Initializer.Call == nil || len(selectedNameByIDConfirmationCarrier.Initializer.Call.Arguments) != 3 || selectedNameByIDConfirmationCarrier.Initializer.Call.Arguments[0].Kind != hir.ExprReference || selectedNameByIDConfirmationCarrier.Initializer.Call.Arguments[0].Reference == nil || selectedNameByIDConfirmationCarrier.Initializer.Call.Arguments[0].Reference.Kind != hir.BindingLocal || selectedNameByIDConfirmationCarrier.Initializer.Call.Arguments[0].Reference.Position != selectedNameByIDLocal.Binding.Position || selectedNameByIDConfirmationCarrier.Return == nil || selectedNameByIDConfirmationCarrier.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost second carrier local: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDConfirmed := selectedNameByIDConfirmationCarrier.Return.ImmutableLocal
	if selectedNameByIDConfirmed == nil || selectedNameByIDConfirmed.Initializer.Kind != hir.ExprMatch || selectedNameByIDConfirmed.Initializer.Match == nil || selectedNameByIDConfirmed.Initializer.Match.Value == nil || selectedNameByIDConfirmed.Initializer.Match.Value.Kind != hir.ExprReference || selectedNameByIDConfirmed.Initializer.Match.Value.Reference == nil || selectedNameByIDConfirmed.Initializer.Match.Value.Reference.Kind != hir.BindingLocal || selectedNameByIDConfirmed.Initializer.Match.Value.Reference.Position != selectedNameByIDConfirmationCarrier.Binding.Position || selectedNameByIDConfirmed.Return == nil || selectedNameByIDConfirmed.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost second prior-local helper-carrier match: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDFinalizationCarrier := selectedNameByIDConfirmed.Return.ImmutableLocal
	if selectedNameByIDFinalizationCarrier == nil || selectedNameByIDFinalizationCarrier.Initializer.Kind != hir.ExprCall || selectedNameByIDFinalizationCarrier.Initializer.Call == nil || len(selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments) != 4 || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[0].Kind != hir.ExprReference || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[0].Reference == nil || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[0].Reference.Kind != hir.BindingLocal || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[0].Reference.Position != selectedNameByIDLocal.Binding.Position || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[1].Kind != hir.ExprReference || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[1].Reference == nil || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[1].Reference.Kind != hir.BindingLocal || selectedNameByIDFinalizationCarrier.Initializer.Call.Arguments[1].Reference.Position != selectedNameByIDConfirmed.Binding.Position || selectedNameByIDFinalizationCarrier.Return == nil || selectedNameByIDFinalizationCarrier.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("SelectedNameById HIR lost third carrier local: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDFinalized := selectedNameByIDFinalizationCarrier.Return.ImmutableLocal
	if selectedNameByIDFinalized == nil || selectedNameByIDFinalized.Initializer.Kind != hir.ExprMatch || selectedNameByIDFinalized.Initializer.Match == nil || selectedNameByIDFinalized.Initializer.Match.Value == nil || selectedNameByIDFinalized.Initializer.Match.Value.Kind != hir.ExprReference || selectedNameByIDFinalized.Initializer.Match.Value.Reference == nil || selectedNameByIDFinalized.Initializer.Match.Value.Reference.Kind != hir.BindingLocal || selectedNameByIDFinalized.Initializer.Match.Value.Reference.Position != selectedNameByIDFinalizationCarrier.Binding.Position || selectedNameByIDFinalized.Return == nil || selectedNameByIDFinalized.Return.Kind != hir.ExprCall {
		t.Fatalf("SelectedNameById HIR lost third prior-local helper-carrier match and continuation: %#v", selectedNameByIDFunction.Body)
	}
	selectedNameByIDCore, err := pipelang.LowerHIRToCore(selectedNameByIDHIR)
	if err != nil {
		t.Fatal(err)
	}
	selectedNameByIDCoreFunction := selectedNameByIDCore.Functions[len(selectedNameByIDCore.Functions)-1]
	selectedNameByIDEntry := coreir.SemanticIdentity{PackageID: string(selectedNameByID.Identity.PackageID), Path: string(selectedNameByID.Identity.Path)}
	selectedNameByIDOutcome, err := coreeval.EvaluateProgram(selectedNameByIDCore, selectedNameByIDEntry, []coreeval.Value{rows, {Type: selectedNameByIDCoreFunction.Parameters[1].Type, String: "2"}})
	if err != nil || !selectedNameByIDOutcome.OK || selectedNameByIDOutcome.Value.String != "beta" {
		t.Fatalf("SelectedNameById present result = %#v, %v", selectedNameByIDOutcome, err)
	}
	selectedNameByIDOutcome, err = coreeval.EvaluateProgram(selectedNameByIDCore, selectedNameByIDEntry, []coreeval.Value{rows, {Type: selectedNameByIDCoreFunction.Parameters[1].Type, String: "missing"}})
	if err != nil || !selectedNameByIDOutcome.OK || selectedNameByIDOutcome.Value.String != "" {
		t.Fatalf("SelectedNameById absent result = %#v, %v", selectedNameByIDOutcome, err)
	}
	selectedNameByIDGo, err := gobackend.Generate(selectedNameByIDCore)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(selectedNameByIDGo), " := PipeLangFindSelection(") != 1 || strings.Count(string(selectedNameByIDGo), " := PipeLangConfirmSelection(") != 1 || strings.Count(string(selectedNameByIDGo), " := PipeLangFinalizeSelectionHistory(") != 1 || strings.Count(string(selectedNameByIDGo), "matched := ") != 3 || !strings.Contains(string(selectedNameByIDGo), "matched.(pipelangOptionalSome[") || !strings.Contains(string(selectedNameByIDGo), "return PipeLangNormalizeName(") {
		t.Fatalf("SelectedNameById generated Go lost dependent carrier chain continuation:\n%s", selectedNameByIDGo)
	}
	displayName := find("DisplayName")
	displayNameHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *displayName.Identity)
	if err != nil {
		t.Fatal(err)
	}
	displayNameFunction := displayNameHIR.Functions[len(displayNameHIR.Functions)-1]
	if displayNameFunction.Body.Kind != hir.ExprImmutableLocal || displayNameFunction.Body.ImmutableLocal == nil || displayNameFunction.Body.ImmutableLocal.Initializer.Kind != hir.ExprCall || displayNameFunction.Body.ImmutableLocal.Return.Kind != hir.ExprImmutableLocal || displayNameFunction.Body.ImmutableLocal.Return.ImmutableLocal == nil || displayNameFunction.Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Kind != hir.ExprConditional {
		t.Fatalf("DisplayName HIR lost ordered immutable normalization and selection locals: %#v", displayNameFunction.Body)
	}
	displayNameCore, err := pipelang.LowerHIRToCore(displayNameHIR)
	if err != nil {
		t.Fatal(err)
	}
	displayNameCoreFunction := displayNameCore.Functions[len(displayNameCore.Functions)-1]
	displayOutcome, err := coreeval.EvaluateProgram(displayNameCore, coreir.SemanticIdentity{PackageID: string(displayName.Identity.PackageID), Path: string(displayName.Identity.Path)}, []coreeval.Value{{Type: displayNameCoreFunction.Parameters[0].Type, String: ""}, {Type: displayNameCoreFunction.Parameters[1].Type, String: "container-3"}})
	if err != nil || !displayOutcome.OK || displayOutcome.Value.String != "container-3" {
		t.Fatalf("DisplayName fallback result = %#v, %v", displayOutcome, err)
	}
	displayOutcome, err = coreeval.EvaluateProgram(displayNameCore, coreir.SemanticIdentity{PackageID: string(displayName.Identity.PackageID), Path: string(displayName.Identity.Path)}, []coreeval.Value{{Type: displayNameCoreFunction.Parameters[0].Type, String: "   "}, {Type: displayNameCoreFunction.Parameters[1].Type, String: "container-3"}})
	if err != nil || !displayOutcome.OK || displayOutcome.Value.String != "container-3" {
		t.Fatalf("DisplayName normalized-empty fallback result = %#v, %v", displayOutcome, err)
	}
	displayOutcome, err = coreeval.EvaluateProgram(displayNameCore, coreir.SemanticIdentity{PackageID: string(displayName.Identity.PackageID), Path: string(displayName.Identity.Path)}, []coreeval.Value{{Type: displayNameCoreFunction.Parameters[0].Type, String: "  worker  "}, {Type: displayNameCoreFunction.Parameters[1].Type, String: "container-3"}})
	if err != nil || !displayOutcome.OK || displayOutcome.Value.String != "worker" {
		t.Fatalf("DisplayName normalized result = %#v, %v", displayOutcome, err)
	}
	displayGo, err := gobackend.Generate(displayNameCore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(displayGo), "p2 := PipeLangNormalizeName(p0)") || !strings.Contains(string(displayGo), "p3 := func() string") || !strings.Contains(string(displayGo), "if pipelangCompareOrdinalText(p2, \"\") == 0") {
		t.Fatalf("DisplayName generated Go lost ordered immutable normalization and selection locals:\n%s", displayGo)
	}
	details := find("Details")
	detailsHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *details.Identity)
	if err != nil {
		t.Fatal(err)
	}
	detailsFunction := detailsHIR.Functions[len(detailsHIR.Functions)-1]
	if detailsFunction.Body.Kind != hir.ExprImmutableLocal || detailsFunction.Body.ImmutableLocal == nil || detailsFunction.Body.ImmutableLocal.Initializer.Kind != hir.ExprCall || detailsFunction.Body.ImmutableLocal.Return.Kind != hir.ExprImmutableLocal || detailsFunction.Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("Details HIR lost prior-local helper propagation: %#v", detailsFunction.Body)
	}
	detailsCore, err := pipelang.LowerHIRToCore(detailsHIR)
	if err != nil {
		t.Fatal(err)
	}
	detailsCoreFunction := detailsCore.Functions[len(detailsCore.Functions)-1]
	success := coreeval.Value{Type: detailsCoreFunction.Parameters[0].Type, String: "  inspect  "}
	detailsOutcome, err := coreeval.EvaluateProgram(detailsCore, coreir.SemanticIdentity{PackageID: string(details.Identity.PackageID), Path: string(details.Identity.Path)}, []coreeval.Value{success})
	if err != nil || !detailsOutcome.OK || detailsOutcome.Value.String != "inspect" {
		t.Fatalf("Details success = %#v, %v", detailsOutcome, err)
	}
	failure := coreeval.Value{Type: detailsCoreFunction.Parameters[0].Type, String: ""}
	detailsOutcome, err = coreeval.EvaluateProgram(detailsCore, coreir.SemanticIdentity{PackageID: string(details.Identity.PackageID), Path: string(details.Identity.Path)}, []coreeval.Value{failure})
	if err != nil || detailsOutcome.OK || detailsOutcome.Failure == nil || detailsOutcome.Failure.String != "details unavailable" {
		t.Fatalf("Details failure = %#v, %v", detailsOutcome, err)
	}
	detailsGo, err := gobackend.Generate(detailsCore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailsGo), "p1 := PipeLangValidateDetails(p0)") || !strings.Contains(string(detailsGo), "if !p1.OK") || !strings.Contains(string(detailsGo), "p2 := p1.Value") || !strings.Contains(string(detailsGo), "p3 := pipelangTrimText(p2)") {
		t.Fatalf("Details generated Go lost prior-local helper propagation:\n%s", detailsGo)
	}
	detailsMessage := find("DetailsMessage")
	detailsMessageHIR, err := pipelang.LowerSemanticMethodToHIR(analysis, *detailsMessage.Identity)
	if err != nil {
		t.Fatal(err)
	}
	detailsMessageFunction := detailsMessageHIR.Functions[len(detailsMessageHIR.Functions)-1]
	if detailsMessageFunction.Body.Kind != hir.ExprMatch || detailsMessageFunction.Body.Match == nil || detailsMessageFunction.Body.Match.Value == nil || detailsMessageFunction.Body.Match.Value.Kind != hir.ExprCall {
		t.Fatalf("DetailsMessage HIR lost helper-result match: %#v", detailsMessageFunction.Body)
	}
	detailsMessageCore, err := pipelang.LowerHIRToCore(detailsMessageHIR)
	if err != nil {
		t.Fatal(err)
	}
	detailsMessageCoreFunction := detailsMessageCore.Functions[len(detailsMessageCore.Functions)-1]
	detailsMessageEntry := coreir.SemanticIdentity{PackageID: string(detailsMessage.Identity.PackageID), Path: string(detailsMessage.Identity.Path)}
	detailsMessageOutcome, err := coreeval.EvaluateProgram(detailsMessageCore, detailsMessageEntry, []coreeval.Value{{Type: detailsMessageCoreFunction.Parameters[0].Type, String: "  inspect  "}})
	if err != nil || !detailsMessageOutcome.OK || detailsMessageOutcome.Value.String != "inspect" {
		t.Fatalf("DetailsMessage success = %#v, %v", detailsMessageOutcome, err)
	}
	detailsMessageOutcome, err = coreeval.EvaluateProgram(detailsMessageCore, detailsMessageEntry, []coreeval.Value{{Type: detailsMessageCoreFunction.Parameters[0].Type, String: ""}})
	if err != nil || !detailsMessageOutcome.OK || detailsMessageOutcome.Value.String != "details unavailable" {
		t.Fatalf("DetailsMessage failure arm = %#v, %v", detailsMessageOutcome, err)
	}
	detailsMessageGo, err := gobackend.Generate(detailsMessageCore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailsMessageGo), "matched := PipeLangValidateDetails(p0)") || !strings.Contains(string(detailsMessageGo), "if matched.OK") || !strings.Contains(string(detailsMessageGo), "if !matched.OK") {
		t.Fatalf("DetailsMessage generated Go lost helper-result match:\n%s", detailsMessageGo)
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
	if len(checked.Sections) != 3 || len(app.Sections) != 3 || app.Selection == nil || app.Details == nil || app.Logs == nil || app.Metadata.LanguageContract != "v0.66.0" {
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
	_, err := Project(&pipelang.SemanticProjection{Schema: pipelang.PipeLangSemanticProjectionVersion, CompilerContract: pipelang.PipeLangCompilerContract, LanguageContract: pipelang.PipeLangLanguageContractV370, View: pipelang.SemanticProjectionPublic}, &coreir.Program{CompilerContract: pipelang.PipeLangCompilerContract, LanguageContract: "v0.37.0"}, Spec{Identity: LocatedIdentity{Identity: Identity{"p", "missing"}, Source: pipelang.SourceRange{File: "model.pipe"}}})
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
