package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const terminalBooleanSelectorInitializersSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){string value=(a ? b : c) ? trim(raw) : raw;if(a){return value;}else{return value;}}}`

// Preserve the independent type/carrier oracle while exercising each scope class.
func v1020TypedScopes(t *testing.T, source string) string {
	t.Helper()
	method := strings.Index(source, " Select(")
	start := method + strings.Index(source[method:], "{") + 1
	end := start + strings.Index(source[start:], "}")
	body := source[start:end]
	pos := strings.Index(body, "return selected;")
	if method < 0 || pos < 0 {
		t.Fatal("missing typed selector initializer")
	}
	locals := body[:pos]
	condition := v890TypeCondition(source)
	leaf := strings.ReplaceAll(locals+"return selected;", "selected", "selectedLeaf")
	branch := strings.ReplaceAll(locals, "selected", "selectedBranch") + "if(" + condition + "){ " + leaf + "}else{" + leaf + "}"
	return source[:start] + strings.ReplaceAll(locals, "selected", "selectedRoot") + "if(" + condition + "){" + branch + "}else{" + branch + "}" + source[end:]
}
func TestV1020TerminalBooleanSelectorInitializersAdmission(t *testing.T) {
	for _, s := range []string{terminalBooleanSelectorInitializersSource, strings.Replace(terminalBooleanSelectorInitializersSource, "public string Select", "string Select", 1), v940WrapBody(t, straightLineBooleanSelectorInitializersSource, terminalTrees(3)[25], "a")} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, s, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}
func TestV1020TerminalBooleanSelectorInitializersTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1020, true)
}
func TestV1020TerminalBooleanSelectorInitializersCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1020, true)
}

func TestV1020TerminalBooleanSelectorInitializersInheritance(t *testing.T) {
	for _, source := range []string{straightLineBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1010, PipeLangLanguageContractV1020} {
			a, p := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := BuildSemanticProjection(a)
			if err != nil {
				t.Fatal(err)
			}
			g, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			h.LanguageContract = coreir.LanguageContractV1010
			p.LanguageContract = coreir.LanguageContractV1010
			projection.LanguageContract = PipeLangLanguageContractV1010
			artifacts := [][]byte{g}
			for _, v := range []any{h, p, projection} {
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, b)
			}
			if baseline == nil {
				baseline = artifacts
			} else if !reflect.DeepEqual(baseline, artifacts) {
				t.Fatal("inherited HIR/Core/semantic/Go changed")
			}
		}
	}
}

func TestV1020TerminalBooleanSelectorInitializersComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=(a ? b : c) ? Make(value) : Make(0);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=(a ? b : c) ? Make(value) : Make(0);return selected;}}`
			source = v940WrapBody(t, source, terminalTrees(3)[25], "a")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, source, []string{"Select"})
			f := coreFunctionNamed(t, p, "Select")
			var checks strings.Builder
			for _, value := range []int64{-9223372036854775808, 0, 9223372036854775807} {
				for mask := 0; mask < 8; mask++ {
					selected := mask&4 != 0
					if mask&1 != 0 {
						selected = mask&2 != 0
					}
					want, success := int64(1), true
					if selected {
						success = value != 9223372036854775807
						if success {
							want = value + 1
						}
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, Int: value}}
					for i := 0; i < 3; i++ {
						args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: mask&(1<<i) != 0})
					}
					got, err := coreeval.EvaluateProgram(p, f.Identity, args)
					if err != nil || got.OK != success || (success && got.Value.Int != want) || (!success && got.Error != coreir.ArithmeticOverflow) {
						t.Fatal(value, mask, got, err)
					}
					fmt.Fprintf(&checks, "{got:=PipeLangSelect(%d,%t,%t,%t);if got.OK!=%t||(got.OK&&got.Value!=%d)||(!got.OK&&string(got.Error)!=\"overflow\"){t.Fatal(got)}}\n", value, mask&1 != 0, mask&2 != 0, mask&4 != 0, success, want)
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
		})
	}
}

func TestV1020TerminalBooleanSelectorInitializersMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "x type", "result type", "nested a", "nested b", "nested c", "nested x", "nested y", "argument", "terminal outer", "terminal selector", "hidden a", "hidden x", "local", "init", "continuation", "position", "reference", "self", "identity", "version"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, terminalBooleanSelectorInitializersSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			local := f.Body.ImmutableLocal
			returned := local.Initializer
			outer := returned.Conditional
			selector := outer.Condition.Conditional
			switch mutation {
			case "outer":
				returned.Conditional = nil
			case "selector":
				outer.Condition.Conditional = nil
			case "a":
				selector.Condition = nil
			case "b":
				selector.WhenTrue = nil
			case "c":
				selector.WhenFalse = nil
			case "x":
				outer.WhenTrue = nil
			case "y":
				outer.WhenFalse = nil
			case "a type":
				selector.Condition = outer.WhenFalse
			case "b type":
				selector.WhenTrue = outer.WhenFalse
			case "c type":
				selector.WhenFalse = outer.WhenFalse
			case "x type":
				outer.WhenTrue = selector.Condition
			case "result type":
				returned.Type = outer.Condition.Type
			case "nested a":
				selector.Condition = clone(outer.Condition)
			case "nested b":
				selector.WhenTrue = clone(outer.Condition)
			case "nested c":
				selector.WhenFalse = clone(outer.Condition)
			case "nested x":
				outer.WhenTrue = clone(returned)
			case "nested y":
				outer.WhenFalse = clone(returned)
			case "argument":
				local.Return = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: returned.Type, TextTrim: &coreir.TextTrim{Value: clone(returned)}}
			case "terminal outer":
				outer.TerminalStatement = true
			case "terminal selector":
				selector.TerminalStatement = true
			case "hidden a":
				hidden := clone(&f.Body)
				hidden.Type = selector.Condition.Type
				hidden.ImmutableLocal.Return = clone(selector.Condition)
				selector.Condition = hidden
			case "hidden x":
				outer.WhenTrue = clone(&f.Body)
			case "local":
				f.Body.ImmutableLocal = nil
			case "init":
				local.Initializer = nil
			case "continuation":
				local.Return = nil
			case "position":
				local.Position = 999
			case "reference":
				ref := 999
				selector.Condition = &coreir.Expr{Kind: coreir.ExprReference, Type: selector.Condition.Type, Parameter: &ref}
			case "self":
				local.Initializer = clone(local.Return)
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.103.0"
			}
			if coreir.ValidateProgram(p) == nil {
				t.Fatal("malformed Core admitted")
			}
			if _, err := gobackend.Generate(p); err == nil {
				t.Fatal("backend admitted malformed Core")
			}
			if _, err := coreeval.EvaluateProgram(p, f.Identity, nil); err == nil {
				t.Fatal("evaluator admitted malformed Core")
			}
		})
	}
}

func TestV1020TerminalBooleanSelectorInitializersPriorContracts(t *testing.T) {
	sources := []string{terminalBooleanSelectorInitializersSource,
		`public Class Choices {public string Select(string raw,bool a,bool b,bool c){if(a){string value=(a ? b : c) ? trim(raw) : raw;if(b){return value;}else{return value;}}else{return raw;}}}`,
		v940WrapBody(t, straightLineBooleanSelectorInitializersSource, terminalTrees(3)[25], "a")}
	for placement, source := range sources {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, source, []string{"Select"})
		for version := 1; version <= 101; version++ {
			contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
			input.LanguageContract = contract
			if AnalyzeSemanticModuleSet(input).Error() == nil {
				t.Fatal("earlier source admitted initializer", placement, version)
			}
			p.LanguageContract = string(contract)
			assertAdmissionRejected(t, p, "")
		}
	}
}

func TestV1020TerminalBooleanSelectorInitializersInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, terminalBooleanSelectorInitializersSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			outer := f.Body.ImmutableLocal.Initializer.Conditional
			selector := outer.Condition.Conditional
			target := &selector.Condition
			switch slot {
			case "b":
				target = &selector.WhenTrue
			case "c":
				target = &selector.WhenFalse
			case "x":
				target = &outer.WhenTrue
			case "y":
				target = &outer.WhenFalse
			}
			value := *target
			position := len(f.Parameters)
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("generic internal Core narrowed: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}

func TestV1020TerminalBooleanSelectorInitializersSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c)`
	for _, body := range []string{
		`{string x=((a ? b : c) ? b : c) ? raw : "x";return x;}`,
		`{string x=(a ? (b ? a : c) : c) ? raw : "x";return x;}`,
		`{string x=(a ? b : (c ? a : b)) ? raw : "x";return x;}`,
		`{string x=(a ? b : c) ? (a ? raw : "x") : raw;return x;}`,
		`{string x=(a ? b : c) ? raw : (a ? raw : "x");return x;}`,
		`{string x=((a ? b : c) && a) ? raw : "x";return x;}`,
		`{string x=(raw ? b : c) ? raw : "x";return x;}`,
		`{string x=(a ? raw : c) ? raw : "x";return x;}`,
		`{string x=(a ? b : raw) ? raw : "x";return x;}`,
		`{string x=(a ? b : c) ? true : raw;return x;}`,
		`{bool x=(a ? b : c) ? raw : "x";return raw;}`,
		`{string x=(a ? b : c) ? missing : raw;return x;}`,
		`{string x=(a ? b : c) ? x : raw;return x;}`,
		`{string x=(a ? b : c) ? later : raw;string later=raw;return x;}`,
		`{string raw=(a ? b : c) ? raw : "x";return raw;}`,
		`{string x=(a ? b : c) ? raw : "x";string x=raw;return x;}`,
		`{string x=trim((a ? b : c) ? raw : "x");return x;}`,
		`{string x=(a ? b : c) ? propagate(raw) : raw;return x;}`,
		`{string x=(a ? b : c) ? match(raw){some(v)=>v,none=>raw} : raw;return x;}`,
		`{string x=(a ? b : c) ? raw : "x";return trim((a ? b : c) ? x : raw);}`,
		`{string x=(a ? b : c) ? raw : "x";return a ? (b ? (c ? (a ? x : raw) : raw) : raw) : raw;}`,
		`{if(a ? b : c){return raw;}else{return "x";}}`,
		`=> {string x=(a ? b : c) ? raw : "x";return x;};`,
	} {
		bodies := []string{body}
		if strings.HasPrefix(body, "{") {
			bodies = append(bodies, "{if(a){"+strings.TrimSuffix(strings.TrimPrefix(body, "{"), "}")+"}else{return raw;}}")
		}
		for _, bad := range bodies {
			source := prefix + bad + "}"
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV1020
			analysis := AnalyzeSemanticModuleSet(input)
			if analysis.Error() == nil {
				t.Fatal("excluded source admitted", bad)
			}
			for _, d := range analysis.Diagnostics {
				if !d.Primary.IsValid() || d.Primary.End > len(source) {
					t.Fatal("invalid diagnostic span", d)
				}
			}
		}
	}
	for _, source := range []string{strings.Replace(terminalBooleanSelectorInitializersSource, "public string Select", "private string Select", 1), strings.Replace(arrowBooleanSelectorsSource, "public string Select", "private string Select", 1)} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1020
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("private new form admitted")
		}
	}
}

// Twenty-four layouts cover all nonempty subsets of two selector slots, a four-local
// mixed sequence, unused tails and ordinary/depth-three/selector returns.
// The two selector triples and return triple are independent (512 vectors/layout).

func TestV1020TerminalBooleanSelectorInitializersLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal("shape inventory drift")
	}
	for partition := 0; partition < 200; partition++ {
		shape := partition / 8
		tree := shapes[shape]
		t.Run(fmt.Sprint(partition), func(t *testing.T) {
			enterFiniteShape(t)
			type layout struct {
				choices, scopes, count int
				unused                 bool
			}
			layouts := []layout{}
			for choices := 0; choices < 8; choices++ {
				for _, unused := range []bool{false, true} {
					layouts = append(layouts, layout{choices, 7, 3, unused})
				}
			}
			if shape == 24 {
				for scopes := 0; scopes < 8; scopes++ {
					for _, unused := range []bool{false, true} {
						layouts = append(layouts, layout{7, scopes, 3, unused})
					}
				}
				layouts = append(layouts, layout{21, 7, 5, false}, layout{31, 7, 5, true})
			}
			for sample, l := range layouts {
				if sample%8 != partition%8 {
					continue
				}
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(string raw`)
				for _, prefix := range []string{"s", "c"} {
					count := 7
					if prefix == "c" {
						count = 9
					}
					for bit := 0; bit < count; bit++ {
						fmt.Fprintf(&source, ",bool %s%d", prefix, bit)
					}
				}
				source.WriteString("){")
				scopeKind := func(n *terminalTree, index int) int {
					if index == 0 {
						return 0
					}
					if n == nil {
						return 2
					}
					return 1
				}
				var emit func(*terminalTree, int, string)
				emit = func(n *terminalTree, index int, previous string) {
					for i := 0; i < l.count; i++ {
						name := fmt.Sprintf("n%dq%d", index, i)
						init := fmt.Sprintf(`Echo(%s+"O%d.%d")`, previous, index, i)
						if l.scopes&(1<<scopeKind(n, index)) != 0 && l.choices&(1<<i) != 0 {
							init = v1020ChoiceSource(index, i, previous)
						} else if l.count == 5 && i == 1 {
							init = v930ChoiceSource(shapes[24], 0, previous)
						}
						fmt.Fprintf(&source, "string %s=%s;", name, init)
						if !l.unused || i < l.count-1 {
							previous = name
						}
					}
					if n == nil {
						switch sample % 3 {
						case 0:
							fmt.Fprintf(&source, `return Echo(%s+"R%d");`, previous, index)
						case 1:
							fmt.Fprintf(&source, `return Check("R6",c6) ? (Check("R7",c7) ? (Check("R8",c8) ? Echo(%s+"T%d") : Echo(%s+"U%d")) : Echo(%s+"V%d")) : Echo(%s+"F%d");`, previous, index, previous, index, previous, index, previous, index)
						case 2:
							fmt.Fprintf(&source, `return (Check("R6",c6) ? Check("R7",c7) : Check("R8",c8)) ? Echo(%s+"T%d") : Echo(%s+"F%d");`, previous, index, previous, index)
						}
						return
					}
					fmt.Fprintf(&source, `if(Check("S%d",s%d) && %s!=""){`, index, index, previous)
					emit(n.yes, index*2+1, previous)
					source.WriteString("}else{")
					emit(n.no, index*2+2, previous)
					source.WriteString("}")
				}
				emit(tree, 0, "raw")
				source.WriteString("}}")
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, source.String(), []string{"Select", "Echo", "Check"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, source.String(), []string{"Select", "Echo", "Check"})
				h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
				if err != nil {
					t.Fatal(err)
				}
				hh, err := LowerSemanticMethodToHIR(aa, semanticMethodNamed(t, aa, "Select").Identity)
				if err != nil {
					t.Fatal(err)
				}
				projection, err := BuildSemanticProjection(a)
				if err != nil {
					t.Fatal(err)
				}
				again, err := BuildSemanticProjection(aa)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(h, hh) || !reflect.DeepEqual(p, pp) || !reflect.DeepEqual(projection, again) {
					t.Fatal("nondeterministic HIR/Core/semantic")
				}
				generated, err := gobackend.Generate(p)
				if err != nil {
					t.Fatal(err)
				}
				repeated, err := gobackend.Generate(pp)
				if err != nil || !bytes.Equal(generated, repeated) {
					t.Fatal("nondeterministic Go")
				}
				f := coreFunctionNamed(t, p, "Select")
				prepared := prepareConformanceProgram(t, p)
				type row struct {
					Flags []bool
					Value string
					Trace []string
				}
				rows := []row{}
				// One representative per reachable statement path; irrelevant routing bits collapse.
				// Exhaust two independent initializer triples and an independent return triple.
				// Selector triples alternate across locals and are shared between scopes.
				paths := []int{}
				seen := map[string]bool{}
				for bits := 0; bits < 128; bits++ {
					leaf, _ := v930ChoiceWant(tree, bits)
					if !seen[leaf] {
						seen[leaf] = true
						paths = append(paths, bits)
					}
				}
				for _, statement := range paths {
					for initializer := 0; initializer < 64; initializer++ {
						for returned := 0; returned < 8; returned++ {
							bits := initializer | returned<<6
							trace := []string{}
							want := "raw"
							n, index := tree, 0
							for {
								for i := 0; i < l.count; i++ {
									value := want + fmt.Sprintf("O%d.%d", index, i)
									if l.scopes&(1<<scopeKind(n, index)) != 0 && l.choices&(1<<i) != 0 {
										suffix, path := v1020ChoiceWant(index, i, initializer)
										value = want + suffix
										trace = append(trace, path...)
									} else if l.count == 5 && i == 1 {
										suffix, path := v930ChoiceWant(shapes[24], bits)
										value = want + suffix
										for _, event := range path {
											trace = append(trace, "C:"+event)
										}
									}
									trace = append(trace, "E:"+value)
									if !l.unused || i < l.count-1 {
										want = value
									}
								}
								if n == nil {
									suffix := "R"
									if sample%3 != 0 {
										trace = append(trace, "C:R6")
										a, b, c := returned&1 != 0, returned&2 != 0, returned&4 != 0
										suffix = "F"
										if sample%3 == 1 {
											if a {
												trace = append(trace, "C:R7")
												suffix = "V"
												if b {
													trace = append(trace, "C:R8")
													suffix = "U"
													if c {
														suffix = "T"
													}
												}
											}
										} else {
											picked := c
											if a {
												trace = append(trace, "C:R7")
												picked = b
											} else {
												trace = append(trace, "C:R8")
											}
											if picked {
												suffix = "T"
											}
										}
									}
									want += fmt.Sprintf("%s%d", suffix, index)
									trace = append(trace, "E:"+want)
									break
								}
								trace = append(trace, fmt.Sprintf("C:S%d", index))
								if statement&(1<<index) != 0 {
									n = n.yes
									index = index*2 + 1
								} else {
									n = n.no
									index = index*2 + 2
								}
							}
							flags := []bool{}
							for bit := 0; bit < 7; bit++ {
								flags = append(flags, statement&(1<<bit) != 0)
							}
							for bit := 0; bit < 9; bit++ {
								flags = append(flags, bits&(1<<bit) != 0)
							}
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							for bit, flag := range flags {
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
							}
							got, err := prepared.Evaluate(f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("shape %d layout %+v bits %d/%d: %#v %v want %q", shape, l, statement, initializer, got, err, want)
							}
							rows = append(rows, row{flags, want, trace})
						}
					}
				}
				fixture, err := json.Marshal(rows)
				if err != nil {
					t.Fatal(err)
				}
				call := `PipeLangSelect("raw"`
				for bit := 0; bit < 16; bit++ {
					call += fmt.Sprintf(",r.Flags[%d]", bit)
				}
				call += ")"
				loader := `type row struct{Flags []bool;Value string;Trace []string};func load(t *testing.T)[]row{b,e:=os.ReadFile("oracle.json");if e!=nil{t.Fatal(e)};var rows []row;if e=json.Unmarshal(b,&rows);e!=nil{t.Fatal(e)};if len(rows)==0{t.Fatal("empty oracle")};return rows}`
				checks := fmt.Sprintf("package %s\nimport(\"testing\";\"encoding/json\";\"os\")\n%s\nfunc TestValues(t *testing.T){for _,r:=range load(t){if got:=%s;got!=r.Value{t.Fatal(got,r.Value)}}}", gobackend.PackageName, loader, call)
				compileAndRunGeneratedGoFilesWithFixtures(t, generated, []byte(checks), map[string][]byte{"oracle.json": fixture})
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1020Trace=append(v1020Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1020Trace=append(v1020Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				orders := fmt.Sprintf("package %s\nimport(\"testing\";\"encoding/json\";\"os\";\"reflect\")\nvar v1020Trace []string\n%s\nfunc TestOrder(t *testing.T){for _,r:=range load(t){v1020Trace=nil;got:=%s;if got!=r.Value||!reflect.DeepEqual(v1020Trace,r.Trace){t.Fatal(got,r.Value,v1020Trace,r.Trace)}}}", gobackend.PackageName, loader, call)
				compileAndRunGeneratedGoFilesWithFixtures(t, []byte(observed), []byte(orders), map[string][]byte{"oracle.json": fixture})
				t.Logf("v102-layout shape=%d choices=%d scopes=%d locals=%d unused=%t vectors=%d", shape, l.choices, l.scopes, l.count, l.unused, len(rows))
			}
		})
	}
}

// The source printer and oracle deliberately use different representations.
func v1020ChoiceSource(index, slot int, previous string) string {
	base := 3 * (slot % 2)
	label := fmt.Sprintf("I%d.%d", index, slot)
	return fmt.Sprintf(`(Check("%sA",c%d) ? Check("%sB",c%d) : Check("%sC",c%d)) ? Echo(%s+"T%d.%d") : Echo(%s+"F%d.%d")`, label, base, label, base+1, label, base+2, previous, index, slot, previous, index, slot)
}
func v1020ChoiceWant(index, slot, bits int) (string, []string) {
	flags := (bits >> (3 * (slot % 2))) & 7
	label := fmt.Sprintf("C:I%d.%d", index, slot)
	trace := []string{label + "A"}
	picked := flags&4 != 0
	if flags&1 != 0 {
		trace = append(trace, label+"B")
		picked = flags&2 != 0
	} else {
		trace = append(trace, label+"C")
	}
	suffix := "F"
	if picked {
		suffix = "T"
	}
	return fmt.Sprintf("%s%d.%d", suffix, index, slot), trace
}

func TestV1020TerminalBooleanSelectorInitializersDependentScope(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool q,bool r){
 bool selected=(a ? b : c) ? q : r;
 if(selected){
  string left=(q ? r : a) ? trim(raw) : raw;
  if(left!=""){return left+"!";}else{return left;}
 }else{string right=(b ? c : q) ? trim(raw) : raw;string sibling=right;return sibling+"?";}
 }}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	prepared := prepareConformanceProgram(t, p)
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	var checks strings.Builder
	for _, raw := range []string{"", " ", " value "} {
		for mask := 0; mask < 32; mask++ {
			a, b, c, q, r := mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0
			want := raw
			choose := c
			if a {
				choose = b
			}
			selected := r
			if choose {
				selected = q
			}
			if selected {
				trimmed := a
				if q {
					trimmed = r
				}
				if trimmed {
					want = strings.TrimSpace(want)
				}
				if want != "" {
					want += "!"
				}
			} else {
				trimmed := q
				if b {
					trimmed = c
				}
				if trimmed {
					want = strings.TrimSpace(want)
				}
				want += "?"
			}
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}}
			for i, flag := range []bool{a, b, c, q, r} {
				args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: flag})
			}
			got, err := prepared.Evaluate(f.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(mask, got, err, want)
			}
			fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t,%t,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, a, b, c, q, r, want)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestScope(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	for _, pair := range [][2]string{
		{`return sibling+"?";`, `return left+"?";`},
		{`string right=(b ?`, `string selected=(b ?`},
		{`string left=(q ?`, `string raw=(q ?`},
		{`? trim(raw) : raw`, `? left : raw`},
		{`bool selected=(a ?`, `bool selected=(selected ?`},
		{`if(selected)`, `if(left!="")`},
		{`if(left!="")`, `if(q ? r : a)`},
	} {
		bad := strings.Replace(source, pair[0], pair[1], 1)
		if bad == source {
			t.Fatal("missed mutation")
		}
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "scope.pipe", bad)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1020
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("scope violation admitted", pair)
		}
	}
	// Core bindings use lexical positions, which can be reused by siblings.
	// The extra sibling local has a position absent from the receiving branch.
	left := p.Functions[len(p.Functions)-1].Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal
	right := p.Functions[len(p.Functions)-1].Body.ImmutableLocal.Return.Conditional.WhenFalse.ImmutableLocal
	left.Return.Conditional.WhenTrue.Binary.Left = right.Return.ImmutableLocal.Return.Binary.Left
	if coreir.ValidateProgram(p) == nil {
		t.Fatal("Core sibling escape admitted")
	}
	if _, err := gobackend.Generate(p); err == nil {
		t.Fatal("backend sibling escape admitted")
	}
	if _, err := coreeval.EvaluateProgram(p, f.Identity, nil); err == nil {
		t.Fatal("evaluator sibling escape admitted")
	}
}

func TestV1020TerminalBooleanSelectorInitializersTreeCoreRefusal(t *testing.T) {
	for _, scope := range []string{"root", "true", "false"} {
		for _, mutation := range []string{"hidden arm", "hidden condition", "hidden statement condition", "depth four choice", "terminal initializer", "missing arm", "wrong type", "forward", "statement condition", "depth four"} {
			t.Run(scope+"/"+mutation, func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1020, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){string root=(a ? b : c) ? trim(raw) : raw;if(a){string left=(b ? c : a) ? root : raw;return left;}else{string right=(c ? a : b) ? root : raw;return right;}}}`, []string{"Select"})
				f := &p.Functions[0]
				root := f.Body.ImmutableLocal
				tree := root.Return.Conditional
				local := root
				if scope == "true" {
					local = tree.WhenTrue.ImmutableLocal
				}
				if scope == "false" {
					local = tree.WhenFalse.ImmutableLocal
				}
				choice := local.Initializer.Conditional
				switch mutation {
				case "hidden arm", "hidden condition":
					target := &choice.WhenTrue
					if mutation == "hidden condition" {
						target = &choice.Condition
					}
					value := *target
					position := local.Position
					*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
					if err := coreir.ValidateFunction(*f); err != nil {
						t.Fatalf("internal Core narrowed: %v", err)
					}
				case "hidden statement condition":
					// Exercise each statement depth, not three copies of the root.
					statement := tree
					depth := 0
					if scope == "true" {
						depth = 1
					}
					if scope == "false" {
						depth = 2
					}
					for i := 0; i < depth; i++ {
						previous := statement.WhenTrue
						statement.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: previous.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: tree.Condition, WhenTrue: previous, WhenFalse: tree.WhenFalse}}
						statement = statement.WhenTrue.Conditional
					}
					value := statement.Condition
					position := root.Position + 1
					statement.Condition = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
					if err := coreir.ValidateFunction(*f); err != nil {
						t.Fatalf("internal Core narrowed: %v", err)
					}
				case "depth four choice":
					leaf := choice.WhenTrue
					for i := 0; i < 3; i++ {
						leaf = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: choice.Condition, WhenTrue: leaf, WhenFalse: choice.WhenFalse}}
					}
					choice.WhenTrue = leaf
				case "terminal initializer":
					choice.TerminalStatement = true
				case "missing arm":
					choice.WhenFalse = nil
				case "wrong type":
					choice.WhenFalse = choice.Condition
				case "forward":
					position := local.Position
					choice.WhenFalse = &coreir.Expr{Kind: coreir.ExprReference, Type: local.Type, Parameter: &position}
				case "statement condition":
					tree.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: tree.Condition.Type, Conditional: &coreir.Conditional{Condition: tree.Condition, WhenTrue: tree.Condition, WhenFalse: tree.Condition}}
				case "depth four":
					tail := tree.WhenFalse
					for i := 0; i < 3; i++ {
						tail = &coreir.Expr{Kind: coreir.ExprConditional, Type: tail.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: tree.Condition, WhenTrue: tail, WhenFalse: tree.WhenFalse}}
					}
					tree.WhenFalse = tail
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}
