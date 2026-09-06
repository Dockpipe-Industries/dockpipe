package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

type inheritedChain struct{ typ, ops string }

func (c inheritedChain) version() int {
	if len(c.ops) == 2 {
		return 57
	}
	return 58
}

func (c inheritedChain) source() string {
	var s strings.Builder
	fmt.Fprintf(&s, "public Class Cursor { public Result<%s, ArithmeticError> Advance(Result<%s, ArithmeticError> carrier", c.typ, c.typ)
	for i := range c.ops {
		fmt.Fprintf(&s, ", %s operand%d", c.typ, i)
	}
	fmt.Fprintf(&s, ") { %s value0 = propagate(carrier);", c.typ)
	for i, op := range c.ops {
		if i == len(c.ops)-1 {
			fmt.Fprintf(&s, "return value%d %c operand%d;", i, op, i)
		} else {
			fmt.Fprintf(&s, "Result<%s, ArithmeticError> carrier%d = value%d %c operand%d; %s value%d = propagate(carrier%d);", c.typ, i+1, i, op, i, c.typ, i+1, i+1)
		}
	}
	s.WriteString("} }")
	return s.String()
}

func inheritedChains() []inheritedChain {
	var chains []inheritedChain
	var add func(string, int)
	add = func(ops string, remaining int) {
		if remaining == 0 {
			chains = append(chains, inheritedChain{"int", ops})
			return
		}
		for _, op := range []string{"+", "-", "*"} {
			add(ops+op, remaining-1)
		}
	}
	for _, stages := range []int{2, 3} {
		add("", stages)
		chains = append(chains, inheritedChain{"float", strings.Repeat("/", stages)})
	}
	return append(chains, inheritedChain{"int", "+-*+-*+-"}, inheritedChain{"float", "////////"})
}

// All 36 integer two/three-stage operator combinations, their float counterparts,
// and two longer chains retain identical artifacts under every inheriting version.
func TestCheckedChainInheritanceAdmission(t *testing.T) {
	pipelines, outcomes := 0, 0
	for _, chain := range inheritedChains() {
		t.Run(chain.typ+chain.ops, func(t *testing.T) {
			var baseline [][]byte
			for version := chain.version(); version <= 85; version++ {
				t.Run(fmt.Sprint(version), func(t *testing.T) {
					contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
					input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "chain.pipe", chain.source())}, nil)
					input.LanguageContract = contract
					analysis := AnalyzeSemanticModuleSet(input)
					if err := analysis.Error(); err != nil {
						t.Fatal(err)
					}
					projection, err := BuildSemanticProjection(analysis)
					if err != nil {
						t.Fatal(err)
					}
					typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Advance").Identity)
					if err != nil {
						t.Fatal(err)
					}
					program, err := LowerHIRToCore(typed)
					if err != nil {
						t.Fatal(err)
					}
					if typed.LanguageContract != string(contract) || program.LanguageContract != string(contract) || projection.LanguageContract != contract || projection.Schema != "pipelang.semantic.v1" || projection.CompilerContract != "pipelang.compiler.v1" || program.CompilerContract != "pipelang.compiler.v1" {
						t.Fatal("contract identity drift")
					}
					repeated, err := BuildSemanticProjection(AnalyzeSemanticModuleSet(input))
					if err != nil || !reflect.DeepEqual(projection, repeated) {
						t.Fatal("semantic output is nondeterministic")
					}
					generated, err := gobackend.Generate(program)
					if err != nil {
						t.Fatal(err)
					}
					again, err := gobackend.Generate(program)
					if err != nil || !bytes.Equal(generated, again) {
						t.Fatal("Go output is nondeterministic")
					}
					tests := inheritedChainOutcomes(t, program, chain)
					pipelines++
					outcomes += len(inheritedChainSamples(chain))
					// Normalize metadata only; spans, types, identities, and full bodies must agree.
					normalized := fmt.Sprintf("v0.%d.0", chain.version())
					typed.LanguageContract, program.LanguageContract, projection.LanguageContract = normalized, normalized, LanguageContract(normalized)
					artifacts := [][]byte{nil, nil, nil, generated}
					for i, value := range []any{typed, program, projection} {
						artifacts[i], err = json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
					}
					if version == chain.version() {
						baseline = artifacts
						compileAndRunGeneratedGoFiles(t, generated, tests)
					} else if !reflect.DeepEqual(baseline, artifacts) {
						t.Fatal("inherited HIR/Core/semantic/Go artifacts changed beyond language metadata")
					}
				})
			}
		})
	}
	t.Logf("%d compiler pipelines; %d outcomes through each evaluator entrypoint; %d distinct generated Go artifacts", pipelines, outcomes, len(inheritedChains()))
}

type inheritedChainSample struct {
	name     string
	ints     []int64
	floats   []float64
	incoming coreir.ArithmeticError
}

func inheritedChainSamples(c inheritedChain) []inheritedChainSample {
	var samples []inheritedChainSample
	if c.typ == "int" {
		neutral := func(start int64) []int64 {
			values := make([]int64, len(c.ops)+1)
			values[0] = start
			for i, op := range c.ops {
				if op == '*' {
					values[i+1] = 1
				}
			}
			return values
		}
		// Identity at both extrema and adjacent values must not trip overflow.
		for _, start := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64 - 1, math.MaxInt64} {
			samples = append(samples, inheritedChainSample{name: fmt.Sprintf("neutral_%d", start), ints: neutral(start)})
		}
		values := neutral(19)
		for i := 1; i < len(values); i++ {
			values[i] = int64(i + 1)
		}
		samples = append(samples, inheritedChainSample{name: "ordered", ints: values})
		for i, op := range c.ops {
			for _, lower := range []bool{false, true} {
				start, operand := int64(math.MaxInt64), int64(1)
				if lower {
					start, operand = math.MinInt64, -1
				}
				if op == '-' {
					operand = -operand
				}
				if op == '*' {
					operand = 2
				}
				values := neutral(start)
				values[i+1] = operand
				samples = append(samples, inheritedChainSample{name: fmt.Sprintf("stage_%d_lower_%t", i, lower), ints: values})
			}
			if op == '*' {
				values := neutral(math.MinInt64)
				values[i+1] = -1
				samples = append(samples, inheritedChainSample{name: fmt.Sprintf("negate_min_%d", i), ints: values})
			}
		}
		for _, failure := range []coreir.ArithmeticError{coreir.ArithmeticOverflow, coreir.ArithmeticDivisionByZero} {
			values := neutral(0)
			for i := 1; i < len(values); i++ {
				values[i] = math.MaxInt64
			}
			samples = append(samples, inheritedChainSample{name: "incoming_" + string(failure), ints: values, incoming: failure})
		}
	} else {
		neutral := func(start float64) []float64 {
			values := make([]float64, len(c.ops)+1)
			values[0] = start
			for i := 1; i < len(values); i++ {
				values[i] = 1
			}
			return values
		}
		for i, start := range []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, math.MaxFloat64, 8} {
			samples = append(samples, inheritedChainSample{name: fmt.Sprintf("transport_%d", i), floats: neutral(start)})
		}
		for i, pair := range [][2]float64{{math.SmallestNonzeroFloat64, 2}, {-math.SmallestNonzeroFloat64, 2}, {math.NaN(), 0}, {math.Inf(1), math.Inf(1)}} {
			values := neutral(pair[0])
			values[1] = pair[1]
			samples = append(samples, inheritedChainSample{name: fmt.Sprintf("IEEE_boundary_%d", i), floats: values})
		}
		values := neutral(4096)
		for i := 1; i < len(values); i++ {
			values[i] = float64(i + 1)
		}
		samples = append(samples, inheritedChainSample{name: "ordered", floats: values})
		for i := range c.ops {
			for j, divisor := range []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.SmallestNonzeroFloat64, math.MaxFloat64, -2} {
				values := neutral(8)
				values[i+1] = divisor
				samples = append(samples, inheritedChainSample{name: fmt.Sprintf("stage_%d_divisor_%d", i, j), floats: values})
			}
		}
		for _, failure := range []coreir.ArithmeticError{coreir.ArithmeticOverflow, coreir.ArithmeticDivisionByZero} {
			values := neutral(0)
			values[1] = 0
			samples = append(samples, inheritedChainSample{name: "incoming_" + string(failure), floats: values, incoming: failure})
		}
	}
	return samples
}

// The oracle uses arbitrary-precision integer arithmetic and native IEEE division,
// independently of compiler/Core checked arithmetic implementations.
func inheritedChainExpected(c inheritedChain, sample inheritedChainSample) (int64, float64, coreir.ArithmeticError) {
	if sample.incoming != "" {
		return 0, 0, sample.incoming
	}
	if c.typ == "float" {
		value := sample.floats[0]
		for _, divisor := range sample.floats[1:] {
			if divisor == 0 {
				return 0, 0, coreir.ArithmeticDivisionByZero
			}
			value /= divisor
		}
		return 0, value, ""
	}
	value := big.NewInt(sample.ints[0])
	for i, op := range c.ops {
		right := big.NewInt(sample.ints[i+1])
		switch op {
		case '+':
			value.Add(value, right)
		case '-':
			value.Sub(value, right)
		case '*':
			value.Mul(value, right)
		}
		if !value.IsInt64() {
			return 0, 0, coreir.ArithmeticOverflow
		}
	}
	return value.Int64(), 0, ""
}

func inheritedChainOutcomes(t *testing.T, program coreir.Program, c inheritedChain) []byte {
	t.Helper()
	f := coreFunctionNamed(t, program, "Advance")
	payload := f.Parameters[1].Type
	var tests strings.Builder
	tests.WriteString("package pipelanggenerated\nimport (\"math\";\"testing\")\nfunc TestInheritedChain(t *testing.T) { _ = math.MaxInt64\n")
	typ := "int64"
	if c.typ == "float" {
		typ = "float64"
	}
	for _, sample := range inheritedChainSamples(c) {
		wi, wf, failure := inheritedChainExpected(c, sample)
		args := make([]coreeval.Value, len(f.Parameters))
		literals := make([]string, len(args))
		for i := range args {
			args[i].Type = payload
			if c.typ == "int" {
				args[i].Int = sample.ints[i]
				literals[i] = fmt.Sprint(sample.ints[i])
			} else {
				args[i].Float = sample.floats[i]
				literals[i] = fmt.Sprintf("math.Float64frombits(%d)", math.Float64bits(sample.floats[i]))
			}
		}
		incoming := coreeval.Outcome{OK: sample.incoming == "", Value: args[0], Error: sample.incoming}
		args[0] = coreeval.Value{Type: f.Parameters[0].Type, Result: &incoming}
		before := incoming
		for _, standalone := range []bool{false, true} {
			var got coreeval.Outcome
			var err error
			if standalone {
				got, err = coreeval.Evaluate(f, args)
			} else {
				got, err = coreeval.EvaluateProgram(program, f.Identity, args)
			}
			if err != nil || got.OK != (failure == "") || got.Error != failure || !reflect.DeepEqual(got.Value.Type, payload) || got.Value.Int != wi || (!(math.IsNaN(got.Value.Float) && math.IsNaN(wf)) && math.Float64bits(got.Value.Float) != math.Float64bits(wf)) {
				t.Fatalf("%s standalone=%t: %#v, %v; want %d/%v/%s", sample.name, standalone, got, err, wi, wf, failure)
			}
		}
		if incoming.OK != before.OK || incoming.Error != before.Error || incoming.Value.Int != before.Value.Int || math.Float64bits(incoming.Value.Float) != math.Float64bits(before.Value.Float) || !reflect.DeepEqual(incoming.Value.Type, before.Value.Type) {
			t.Fatal("input carrier changed")
		}
		check := fmt.Sprintf("got.Value != %d", wi)
		if c.typ == "float" {
			check = fmt.Sprintf("math.Float64bits(got.Value) != %d", math.Float64bits(wf))
			if math.IsNaN(wf) {
				check = "!math.IsNaN(got.Value)"
			}
		}
		fmt.Fprintf(&tests, "{ got := PipeLangAdvance(PipeLangArithmeticResult[%s]{OK:%t,Value:%s,Error:%q},%s); if got.OK!=%t || got.Error!=%q || %s {t.Fatalf(%q,got)} }\n", typ, sample.incoming == "", literals[0], sample.incoming, strings.Join(literals[1:], ","), failure == "", failure, check, sample.name+": %#v")
	}
	for _, bad := range []struct {
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{{true, 0, coreir.ArithmeticOverflow}, {false, 0, "unknown"}, {false, 1, coreir.ArithmeticOverflow}, {false, 0, ""}} {
		carrier := coreeval.Outcome{OK: bad.ok, Value: coreeval.Value{Type: payload, Int: bad.value, Float: float64(bad.value)}, Error: bad.failure}
		args := make([]coreeval.Value, len(f.Parameters))
		for i := range args {
			args[i] = coreeval.Value{Type: payload, Int: 1, Float: 1}
		}
		args[0] = coreeval.Value{Type: f.Parameters[0].Type, Result: &carrier}
		if _, err := coreeval.Evaluate(f, args); err == nil {
			t.Fatal("function accepted malformed carrier")
		}
		if _, err := coreeval.EvaluateProgram(program, f.Identity, args); err == nil {
			t.Fatal("program accepted malformed carrier")
		}
		fmt.Fprintf(&tests, "func(){defer func(){if recover()==nil {t.Fatal(\"malformed carrier accepted\")}}();PipeLangAdvance(PipeLangArithmeticResult[%s]{OK:%t,Value:%d,Error:%q},%s)}()\n", typ, bad.ok, bad.value, bad.failure, strings.TrimSuffix(strings.Repeat("1,", len(c.ops)), ","))
	}
	tests.WriteString("}\n")
	return []byte(tests.String())
}

func TestCheckedChainInheritanceRejection(t *testing.T) {
	for _, chain := range []inheritedChain{{"int", "+-"}, {"int", "+-*"}, {"float", "//"}, {"float", "///"}} {
		source := chain.source()
		last := len(chain.ops) - 1
		for version := chain.version(); version <= 85; version++ {
			t.Run(fmt.Sprintf("%s%s/%d", chain.typ, chain.ops, version), func(t *testing.T) {
				contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
				first := fmt.Sprintf("value0 %c operand0", chain.ops[0])
				terminal := fmt.Sprintf("value%d %c operand%d", last, chain.ops[last], last)
				mutations := []struct{ name, old, new string }{
					{"reversed first", first, fmt.Sprintf("operand0 %c value0", chain.ops[0])},
					{"repeated first", first, fmt.Sprintf("value0 %c value0", chain.ops[0])},
					{"literal first", first, fmt.Sprintf("value0 %c 1", chain.ops[0])},
					{"computed first", first, fmt.Sprintf("value0 %c (operand0 + 1)", chain.ops[0])},
					{"wrong terminal", terminal, fmt.Sprintf("value%d %c operand0", last, chain.ops[last])},
					{"computed propagation", fmt.Sprintf("Result<%s, ArithmeticError> carrier1 = %s; %s value1 = propagate(carrier1);", chain.typ, first, chain.typ), fmt.Sprintf("%s value1 = propagate(%s);", chain.typ, first)},
					{"gap before carrier", fmt.Sprintf("Result<%s, ArithmeticError> carrier1", chain.typ), fmt.Sprintf("int gap = 0; Result<%s, ArithmeticError> carrier1", chain.typ)},
					{"gap before propagation", chain.typ + " value1 =", "int gap = 0; " + chain.typ + " value1 ="},
					{"wrong carrier", "propagate(carrier1)", "propagate(carrier)"},
					{"forward carrier", "propagate(carrier1)", "propagate(value1)"},
					{"shadow payload", chain.typ + " value1 =", chain.typ + " value0 ="},
					{"extra parameter", fmt.Sprintf("operand%d)", last), fmt.Sprintf("operand%d, %s extra)", last, chain.typ)},
					{"missing stage", fmt.Sprintf("Result<%s, ArithmeticError> carrier1 = %s; %s value1 = propagate(carrier1);", chain.typ, first, chain.typ), ""},
					{"branch composition", "return " + terminal + ";", "if (true) { return " + terminal + "; } else { return " + terminal + "; }"},
				}
				wrongType := "float"
				if chain.typ == "float" {
					wrongType = "int"
				}
				mutations = append(mutations, struct{ name, old, new string }{"wrong operand type", chain.typ + " operand0", wrongType + " operand0"})
				if chain.typ == "float" {
					mutations = append(mutations, struct{ name, old, new string }{"unsupported float arithmetic", first, "value0 + operand0"})
				}
				for _, mutation := range mutations {
					t.Run(mutation.name, func(t *testing.T) {
						changed := strings.Replace(source, mutation.old, mutation.new, 1)
						if changed == source {
							t.Fatal("mutation missed")
						}
						input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "invalid.pipe", changed)}, nil)
						input.LanguageContract = contract
						err := AnalyzeSemanticModuleSet(input).Error()
						ds, ok := AsDiagnostics(err)
						if err == nil || !ok || len(ds) == 0 {
							t.Fatal("excluded source admitted")
						}
						for _, d := range ds {
							if !d.Primary.IsValid() || d.Primary.File != "invalid.pipe" || d.Primary.End > len(changed) {
								t.Fatal("invalid diagnostic span")
							}
						}
					})
				}
				// Each mutation starts from independently valid Core and bypasses source analysis.
				valid := reviewCore(t, contract, source, "Advance")
				encoded, err := json.Marshal(valid)
				if err != nil {
					t.Fatal(err)
				}
				for stage := 0; stage < len(chain.ops); stage++ {
					for _, mutation := range []string{"left operand", "right operand", "propagation carrier", "binding position", "carrier type"} {
						if stage == len(chain.ops)-1 && mutation != "left operand" && mutation != "right operand" {
							continue
						}
						t.Run(fmt.Sprintf("Core/%d/%s", stage, mutation), func(t *testing.T) {
							var bad coreir.Program
							if err := json.Unmarshal(encoded, &bad); err != nil {
								t.Fatal(err)
							}
							payload := bad.Functions[0].Body.ImmutableLocal
							for i := 0; i < stage; i++ {
								payload = payload.Return.ImmutableLocal.Return.ImmutableLocal
							}
							arithmetic := payload.Return
							if stage < len(chain.ops)-1 {
								arithmetic = payload.Return.ImmutableLocal.Initializer
							}
							switch mutation {
							case "left operand":
								wrong := stage + 1
								arithmetic.Binary.Left.Parameter = &wrong
							case "right operand":
								wrong := payload.Position
								arithmetic.Binary.Right.Parameter = &wrong
							case "propagation carrier":
								wrong := 0
								payload.Return.ImmutableLocal.Return.ImmutableLocal.Initializer.Propagate.Value.Parameter = &wrong
							case "binding position":
								payload.Return.ImmutableLocal.Position++
							case "carrier type":
								payload.Return.ImmutableLocal.Type = payload.Type
							}
							assertAdmissionRejected(t, bad, "")
						})
					}
				}
			})
		}
		for _, contract := range []LanguageContract{LanguageContract(fmt.Sprintf("v0.%d.0", chain.version()-1)), LanguageContract("v0.86.0")} {
			input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "invalid.pipe", source)}, nil)
			input.LanguageContract = contract
			if AnalyzeSemanticModuleSet(input).Error() == nil {
				t.Fatalf("source admitted under %s", contract)
			}
			program := reviewCore(t, PipeLangLanguageContractV820, source, "Advance")
			program.LanguageContract = string(contract)
			assertAdmissionRejected(t, program, "")
		}
	}
}
