package pipelang

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

// Opt-in measurements use the same source fixture builders as conformance tests.
// Run in canonical containment; timings are evidence, never CI thresholds.
func TestConformancePerformanceProfile(t *testing.T) {
	if os.Getenv("PIPELANG_PERFORMANCE_PROFILE") != "1" {
		t.Skip("opt-in contained performance profile")
	}
	for _, size := range []struct {
		name    string
		targets []string
	}{
		{"small", []string{"R"}}, {"large", []string{"R", "RT", "RT", "RF", "RF"}},
	} {
		t.Run(size.name, func(t *testing.T) {
			tree := terminalTrees(3)[25]
			source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;` + conditionalChoicesTreeMethod(tree, size.targets, false, "Select", true, true, true, true) + "}"
			analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, source, []string{"Echo", "Check", "Select"})
			function := coreFunctionNamed(t, program, "Select")
			identity := semanticMethodNamed(t, analysis, "Select").Identity
			typed, err := LowerSemanticMethodToHIR(analysis, identity)
			if err != nil {
				t.Fatal(err)
			}
			args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "value"}}
			for _, p := range function.Parameters[1:] {
				args = append(args, coreeval.Value{Type: p.Type, Bool: true})
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional-local.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV910
			payload, _ := json.Marshal(program)
			t.Logf("inventory source_bytes=%d core_json_bytes=%d functions=%d locals=%d vectors=%d", len(source), len(payload), len(program.Functions), len(size.targets), 1<<(len(size.targets)+8))
			prepared, err := coreeval.PrepareProgram(program)
			if err != nil {
				t.Fatal(err)
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			retained := make([]*coreeval.PreparedProgram, 32)
			for i := range retained {
				retained[i], err = coreeval.PrepareProgram(program)
				if err != nil {
					t.Fatal(err)
				}
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			t.Logf("prepared_retained_bytes_per_program=%d samples=%d", (int64(after.HeapAlloc)-int64(before.HeapAlloc))/int64(len(retained)), len(retained))
			runtime.KeepAlive(retained)
			phases := []struct {
				name string
				run  func() error
			}{
				{"analyze", func() error { return AnalyzeSemanticModuleSet(input).Error() }},
				{"lower_hir", func() error { _, err := LowerSemanticMethodToHIR(analysis, identity); return err }},
				{"lower_core", func() error { _, err := LowerHIRToCore(typed); return err }},
				{"validate", func() error { return coreir.ValidateProgram(program) }},
				{"evaluate_program", func() error { _, err := coreeval.EvaluateProgram(program, function.Identity, args); return err }},
				{"prepare", func() error { _, err := coreeval.PrepareProgram(program); return err }},
				{"evaluate_prepared", func() error { _, err := prepared.Evaluate(function.Identity, args); return err }},
				{"generate", func() error { _, err := gobackend.Generate(program); return err }},
			}
			for _, phase := range phases {
				result := testing.Benchmark(func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						if err := phase.run(); err != nil {
							b.Fatal(err)
						}
					}
				})
				t.Logf("phase=%s %s %s", phase.name, result.String(), result.MemString())
			}
		})
	}
}
