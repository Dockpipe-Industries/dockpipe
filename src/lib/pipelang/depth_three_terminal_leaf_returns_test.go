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

func TestV940DepthThreeTerminalLeafReturnsScopeOrder(t *testing.T) {
	source := `public Class Choices {
 public string Echo(string value)=>value;
 public bool Check(string name,bool value)=>value;
 public string Select(bool a,bool b,bool c,bool q,bool r){
  string root=Echo("R");
  if(Check("a",a)){
   string middle=Check("q",q) ? (Check("r",r) ? Echo(root+"T") : Echo(root+"M")) : Echo(root+"F");
   if(Check("b",b)){
    string unused=Echo(middle+"U");
    return Check("c",c) ? (Check("q2",q) ? (Check("r2",r) ? Echo(middle+"X") : Echo(middle+"Y")) : Echo(middle+"Z")) : Echo(middle+"N");
   }else{return Echo(middle+"B");}
  }else{return Echo(root+"A");}
 }}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	var checks, orders strings.Builder
	for mask := 0; mask < 32; mask++ {
		a, b, c, q, r := mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0
		trace := []string{"E:R", "C:a"}
		want := "RA"
		if a {
			trace = append(trace, "C:q")
			middle := "RF"
			if q {
				trace = append(trace, "C:r")
				middle = "RM"
				if r {
					middle = "RT"
				}
			}
			trace = append(trace, "E:"+middle, "C:b")
			want = middle + "B"
			if b {
				trace = append(trace, "E:"+middle+"U", "C:c")
				want = middle + "N"
				if c {
					trace = append(trace, "C:q2")
					want = middle + "Z"
					if q {
						trace = append(trace, "C:r2")
						want = middle + "Y"
						if r {
							want = middle + "X"
						}
					}
				}
			}
		}
		trace = append(trace, "E:"+want)
		args := []coreeval.Value{}
		for i := 0; i < 5; i++ {
			args = append(args, coreeval.Value{Type: f.Parameters[i].Type, Bool: mask&(1<<i) != 0})
		}
		got, err := coreeval.EvaluateProgram(p, f.Identity, args)
		if err != nil || !got.OK || got.Value.String != want {
			t.Fatalf("%d: %#v %v", mask, got, err)
		}
		call := fmt.Sprintf("PipeLangSelect(%t,%t,%t,%t,%t)", a, b, c, q, r)
		fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
		fmt.Fprintf(&orders, "v940Trace=nil;%s;if !reflect.DeepEqual(v940Trace,[]string{%s}){t.Fatal(v940Trace)}\n", call, quotedStrings(trace))
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	observed := string(generated)
	for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v940Trace=append(v940Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v940Trace=append(v940Trace,"C:"+p0)`} {
		if strings.Count(observed, marker) != 1 {
			t.Fatal("missing probe marker")
		}
		observed = strings.Replace(observed, marker, marker+probe+"\n", 1)
	}
	compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v940Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
	for _, pair := range [][2]string{{`Echo(root+"A")`, `Echo(middle+"A")`}, {`string middle=`, `string root=`}, {`string root=Echo("R")`, `string root=Echo(middle)`}, {`string unused=Echo(middle+"U")`, `string unused=Echo(unused)`}, {`return Echo(middle+"B");`, `if(c){if(q){return middle;}else{return middle;}}else{return middle;}`}} {
		bad := strings.Replace(source, pair[0], pair[1], 1)
		if bad == source {
			t.Fatal("missed source mutation")
		}
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "scope.pipe", bad)}, nil)
		input.LanguageContract = PipeLangLanguageContractV940
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("admitted scope/depth violation", pair)
		}
	}
}

const depthThreeTerminalLeafReturnsSource = `public Class Choices {
 public string Select(string raw,bool a,bool b,bool c) {
  string first=a ? (b ? trim(raw) : raw) : raw;
  if(a){return a ? (b ? (c ? first : raw) : raw) : raw;}
  else{return a ? raw : (b ? raw : (c ? first : raw));}
 }
}`

func TestV940DepthThreeTerminalLeafReturnsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "leaf-three.pipe", depthThreeTerminalLeafReturnsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.94.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV940DepthThreeReturnsInheritance(t *testing.T) {
	for _, source := range []string{depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV930, PipeLangLanguageContractV940} {
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
			h.LanguageContract = coreir.LanguageContractV930
			p.LanguageContract = coreir.LanguageContractV930
			projection.LanguageContract = PipeLangLanguageContractV930
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

// Wrap the selected method's existing body in a bounded statement tree. Leaves
// deliberately share the same body; the separate subset matrix distinguishes paths.
func v940WrapBody(t *testing.T, source string, tree *terminalTree, condition string) string {
	return v940WrapMethod(t, source, tree, condition, "Select")
}
func v940WrapMethod(t *testing.T, source string, tree *terminalTree, condition, method string) string {
	t.Helper()
	start := strings.Index(source, " "+method+"(")
	if start < 0 {
		t.Fatal("missing Select")
	}
	open := strings.Index(source[start:], "{") + start
	depth, end := 1, open+1
	for ; end < len(source) && depth > 0; end++ {
		if source[end] == '{' {
			depth++
		}
		if source[end] == '}' {
			depth--
		}
	}
	if depth != 0 {
		t.Fatal("unclosed Select")
	}
	body := source[open+1 : end-1]
	var wrap func(*terminalTree) string
	wrap = func(n *terminalTree) string {
		if n == nil {
			return body
		}
		return "if(" + condition + "){" + wrap(n.yes) + "}else{" + wrap(n.no) + "}"
	}
	return source[:open+1] + wrap(tree) + source[end-1:]
}
func TestV940DepthThreeTerminalLeafReturnsTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV940, zero) })
	}
}
func TestV940DepthThreeTerminalLeafReturnsCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV940, zero) })
	}
}

func TestV940DepthThreeTerminalLeafReturnsLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal(len(shapes))
	}
	for shape, tree := range shapes {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			// Zero locals and mixed dependent ordinary/depth-two locals, including an unused tail.
			for _, count := range []int{0, 1, 3} {
				for _, unused := range []bool{false, true} {
					var source strings.Builder
					source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Select(string raw,bool q,bool r`)
					for i := 0; i < 7; i++ {
						fmt.Fprintf(&source, ",bool c%d", i)
					}
					source.WriteString("){")
					previous := "raw"
					for i := 0; i < count; i++ {
						if i%2 == 0 {
							fmt.Fprintf(&source, `string l%d=Check("q%d",q) ? (Check("r%d",r) ? Echo(%s+"T") : Echo(%s+"M")) : Echo(%s+"F");`, i, i, i, previous, previous, previous)
						} else {
							fmt.Fprintf(&source, `string l%d=Echo(%s+"O");`, i, previous)
						}
						if !unused || i < count-1 {
							previous = fmt.Sprintf("l%d", i)
						}
					}
					fmt.Fprintf(&source, "return %s;}}", v930ChoiceSource(tree, 0, previous))
					a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, v940WrapBody(t, source.String(), shapes[shape], "q"), []string{"Select"})
					aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, v940WrapBody(t, source.String(), shapes[shape], "q"), []string{"Select"})
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
					repeated, err := BuildSemanticProjection(aa)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(h, hh) || !reflect.DeepEqual(p, pp) || !reflect.DeepEqual(projection, repeated) {
						t.Fatal("nondeterministic representations")
					}
					generated, err := gobackend.Generate(p)
					if err != nil {
						t.Fatal(err)
					}
					again, err := gobackend.Generate(pp)
					if err != nil || !bytes.Equal(generated, again) {
						t.Fatal("nondeterministic Go")
					}
					f := coreFunctionNamed(t, p, "Select")
					var checks, orders strings.Builder
					for localMask := 0; localMask < 4; localMask++ {
						for mask := 0; mask < 128; mask++ {
							value := "raw"
							trace := []string{}
							for i := 0; i < count; i++ {
								suffix := "O"
								if i%2 == 0 {
									trace = append(trace, fmt.Sprintf("C:q%d", i))
									suffix = "F"
									if localMask&1 != 0 {
										trace = append(trace, fmt.Sprintf("C:r%d", i))
										suffix = "M"
										if localMask&2 != 0 {
											suffix = "T"
										}
									}
								}
								next := value + suffix
								trace = append(trace, "E:"+next)
								if !unused || i < count-1 {
									value = next
								}
							}
							suffix, path := v930ChoiceWant(tree, mask)
							for _, event := range path {
								trace = append(trace, "C:"+event)
							}
							want := value + suffix
							trace = append(trace, "E:"+want)
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							flags := []bool{localMask&1 != 0, localMask&2 != 0}
							for bit := 0; bit < 7; bit++ {
								flags = append(flags, mask&(1<<bit) != 0)
							}
							call := `PipeLangSelect("raw"`
							for bit, flag := range flags {
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
								call += fmt.Sprintf(",%t", flag)
							}
							call += ")"
							got, err := coreeval.EvaluateProgram(p, f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("%d/%d: %#v %v want %q", localMask, mask, got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
							fmt.Fprintf(&orders, "v930Trace=nil;%s;if !reflect.DeepEqual(v930Trace,[]string{%s}){t.Fatal(v930Trace)}\n", call, quotedStrings(trace))
						}
					}
					compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
					observed := string(generated)
					for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v930Trace=append(v930Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v930Trace=append(v930Trace,"C:"+p0)`} {
						if strings.Count(observed, marker) != 1 {
							t.Fatal("missing trace marker")
						}
						observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
					}
					compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v930Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
				}
			}
		})
	}
}

func TestV940DepthThreeTerminalLeafReturnsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			body := `a ? (b ? (c ? Make(value) : Make(0)) : Make(0)) : (c ? Make(1) : Make(2))`
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c){return ` + body + `;}public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`

			if !unused {
				source = strings.Replace(source, "{return Choose(value,a,b,c);}", "=>Choose(value,a,b,c);", 1)
			}
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, v940WrapMethod(t, source, terminalTrees(1)[1], "a", "Choose"), []string{"Select"})
			f := coreFunctionNamed(t, program, "Select")
			var checks strings.Builder
			for _, value := range []int64{-9223372036854775808, 0, 9223372036854775807} {
				for mask := 0; mask < 8; mask++ {
					want, success := int64(3), true
					if mask&1 != 0 {
						want = 1
						if mask&2 != 0 && mask&4 != 0 {
							success = value != 9223372036854775807
							if success {
								want = value + 1
							}
						}
					} else if mask&4 != 0 {
						want = 2
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, Int: value}}
					for bit := 0; bit < 3; bit++ {
						args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
					}
					got, err := coreeval.EvaluateProgram(program, f.Identity, args)
					if err != nil || got.OK != success || (success && got.Value.Int != want) || (!success && got.Error != coreir.ArithmeticOverflow) {
						t.Fatalf("%d/%d: %#v %v", value, mask, got, err)
					}
					fmt.Fprintf(&checks, "{got:=PipeLangSelect(%d,%t,%t,%t);if got.OK!=%t || (got.OK && got.Value!=%d) || (!got.OK && string(got.Error)!=\"overflow\"){t.Fatal(got)}}\n", value, mask&1 != 0, mask&2 != 0, mask&4 != 0, success, want)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
		})
	}
}

func TestV940DepthThreeTerminalLeafReturnsSubsets(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	for shape := 0; shape < 100; shape++ {
		tree := trees[shape/4]
		variant := shape % 4
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			leaves := []string{}
			var inventory func(*terminalTree, string)
			inventory = func(n *terminalTree, p string) {
				if n == nil {
					leaves = append(leaves, p)
					return
				}
				inventory(n.yes, p+"T")
				inventory(n.no, p+"F")
			}
			inventory(tree, "R")
			outcomes := 0
			for start := 0; start < 1<<len(leaves); start += 16 {
				end := start + 16
				if end > 1<<len(leaves) {
					end = 1 << len(leaves)
				}
				var source, checks strings.Builder
				source.WriteString("public Class Choices {")
				methods := []string{}
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					methods = append(methods, name)
					var emit func(*terminalTree, string) string
					emit = func(n *terminalTree, path string) string {
						if n == nil {
							for i, p := range leaves {
								if p == path && subset&(1<<i) != 0 {
									return "return " + v940SubsetChoice(path, (variant+i)%4) + ";"
								}
							}
							return fmt.Sprintf("return %q;", path)
						}
						return fmt.Sprintf("if(d%d){%s}else{%s}", len(path)-1, emit(n.yes, path+"T"), emit(n.no, path+"F"))
					}
					fmt.Fprintf(&source, "public string %s(bool d0,bool d1,bool d2,bool choose,bool yes,bool no){%s}", name, emit(tree, "R"))
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, source.String(), methods)
				_, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, source.String(), methods)
				a, _ := json.Marshal(program)
				b, _ := json.Marshal(again)
				if !bytes.Equal(a, b) {
					t.Fatal("Core nondeterminism")
				}
				generated, err := gobackend.Generate(program)
				if err != nil {
					t.Fatal(err)
				}
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					f := coreFunctionNamed(t, program, name)
					typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
					if err != nil {
						t.Fatal(err)
					}
					if typed.LanguageContract != coreir.LanguageContractV940 {
						t.Fatal("HIR contract")
					}
					for mask := 0; mask < 64; mask++ {
						node, path := tree, "R"
						for depth := 0; node != nil; depth++ {
							if mask&(1<<depth) != 0 {
								node = node.yes
								path += "T"
							} else {
								node = node.no
								path += "F"
							}
						}
						want := path
						for i, p := range leaves {
							if p == path && subset&(1<<i) != 0 {
								if mask&8 != 0 {
									want += "T"
									if (variant+i)%4&1 != 0 {
										if mask&16 != 0 {
											want += "Y"
										} else {
											want += "N"
										}
									}
								} else {
									want += "F"
									if (variant+i)%4&2 != 0 {
										if mask&32 != 0 {
											want += "Y"
										} else {
											want += "N"
										}
									}
								}
							}
						}
						args := []coreeval.Value{}
						for bit := 0; bit < 6; bit++ {
							args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := coreeval.EvaluateProgram(program, f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("subset %d mask %d: %#v %v want %s", subset, mask, got, err, want)
						}
						fmt.Fprintf(&checks, "if got:=PipeLang%s(%t,%t,%t,%t,%t,%t);got!=%q{t.Fatal(got)}\n", name, mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0, mask&32 != 0, want)
						outcomes++
					}
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestSubsets(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			}
			t.Logf("%d leaves, %d subsets, %d evaluator/Go outcomes", len(leaves), 1<<len(leaves), outcomes)
		})
	}
}

// Four rotations exercise every leaf subset with inherited six input bits. A
// duplicated outer choice preserves that independent oracle while reaching three
// levels for nested rotations; the layout matrix exhausts all seven return bits.
func v940SubsetChoice(path string, shape int) string {
	choice := v890Choice(path, shape)
	return "choose ? (" + choice + ") : (" + choice + ")"
}

func TestV940DepthThreeTerminalLeafReturnsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	deep := `a ? (b ? (c ? raw : "C") : "B") : "A"`
	cases := []string{
		`=>` + deep + `;`,
		`{string local=` + deep + `;return local;}`,
		`{return a ? (b ? (c ? (a ? raw : "D") : "C") : "B") : "A";}`,
		`{return a ? raw : (b ? raw : (c ? raw : (a ? raw : "D")));}`,
		`{return (a ? b : c) ? (b ? (c ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? ((a ? b : c) ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? trim(a ? raw : "x") : "C") : "B") : "A";}`,
		`{return trim(` + deep + `);}`,
		`{return a ? (b ? (raw ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? true : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? missing : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? raw : ) : "B") : "A";}`,
		`{string local=local;return ` + deep + `;}`,
		`{string first=second;string second=raw;return ` + deep + `;}`,
		`{string raw="shadow";return ` + deep + `;}`,
		`{return a ? (b ? (c ? propagate(raw) : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? match(raw){some(v)=>v,none=>raw} : "C") : "B") : "A";}`,
	}
	for i, body := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			source := prefix + body + `}`
			if strings.HasPrefix(body, "{return ") {
				source = v940WrapBody(t, source, terminalTrees(1)[1], "a")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV940
			analysis := AnalyzeSemanticModuleSet(input)
			if analysis.Error() == nil {
				t.Fatal("admitted", body)
			}
			for _, d := range analysis.Diagnostics {
				if !d.Primary.IsValid() || d.Primary.File != "depth-three.pipe" || d.Primary.End > len(source) {
					t.Fatal("invalid diagnostic", d)
				}
			}
		})
	}
	for version := 1; version <= 93; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", prefix+`{if(a){return `+deep+`;}else{return raw;}}}`)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV940DepthThreeTerminalLeafReturnsMalformedCore(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) {return a ? (b ? (c ? raw : "C") : "B") : "A";}}`
	copyExpr := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"condition", "arm", "missing", "depth", "condition placement", "argument placement", "hidden local", "terminal", "identity", "version", "downgrade", "initializer", "leaf"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, v940WrapBody(t, source, terminalTrees(1)[1], "a"), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			root := f.Body.Conditional.WhenTrue.Conditional
			third := root.WhenTrue.Conditional.WhenTrue.Conditional
			switch mutation {
			case "condition":
				third.Condition = third.WhenTrue
			case "arm":
				third.WhenTrue = third.Condition
			case "missing":
				third.WhenFalse = nil
			case "depth":
				third.WhenTrue = copyExpr(root.WhenTrue.Conditional.WhenTrue)
			case "condition placement":
				third.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Condition.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: third.Condition, WhenFalse: third.Condition}}
			case "argument placement":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: f.ReturnType, TextTrim: &coreir.TextTrim{Value: copyExpr(root.WhenTrue)}}
			case "hidden local":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: f.ReturnType, ImmutableLocal: &coreir.ImmutableLocal{}}
			case "terminal":
				third.TerminalStatement = true
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.97.0"
			case "downgrade":
				p.LanguageContract = coreir.LanguageContractV930
			case "initializer":
				_, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, depthThreeStraightLineReturnsSource, []string{"Select"})
				q.Functions[len(q.Functions)-1].Body.ImmutableLocal.Initializer = copyExpr(&f.Body)
				p = q
				f = &p.Functions[len(p.Functions)-1]
			case "leaf":
				for depth := 0; depth < 3; depth++ {
					body := copyExpr(&f.Body)
					f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: f.ReturnType, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: root.Condition, WhenTrue: body, WhenFalse: copyExpr(root.WhenFalse)}}
				}
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
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, v940WrapBody(t, source, terminalTrees(1)[1], "a"), []string{"Select"})
	for version := 1; version <= 93; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if coreir.ValidateProgram(p) == nil {
			t.Fatal("older Core admitted", version)
		}
	}
	// Existing arrow/block-erased depth-two Core remains valid at its original boundary.
	_, p = conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, nestedArrowMethodsSource, []string{"Select"})
	for version := 88; version <= 93; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(version, err)
		}
	}
}

func TestV940NestedTerminalLeafReturnsRefusesEmbeddedLocals(t *testing.T) {
	for _, placement := range []string{"return arm", "inner condition", "outer condition", "initializer arm", "statement condition", "ordinary sibling operand"} {
		t.Run(placement, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV940, strings.Replace(nestedTerminalLeafReturnsSource, "finish ? second : first", "finish ? (pick ? second : first) : first", 1), []string{"Select"})
			f := &p.Functions[0]
			first := f.Body.ImmutableLocal
			third := first.Return.Conditional.WhenTrue.ImmutableLocal.Return.ImmutableLocal
			returned := third.Return
			choice := returned.Conditional
			position := len(f.Parameters) + 3
			target := &choice.WhenTrue.Conditional.WhenTrue
			switch placement {
			case "inner condition":
				target = &choice.WhenTrue.Conditional.Condition
			case "outer condition":
				target = &choice.Condition
			case "ordinary sibling operand":
				branch := first.Return.Conditional
				value := branch.WhenFalse.Conditional.WhenFalse
				branch.WhenFalse = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: value.Type, TextTrim: &coreir.TextTrim{Value: value}}
				target = &branch.WhenFalse.TextTrim.Value
				position = len(f.Parameters) + 1
			case "statement condition":
				target = &first.Return.Conditional.Condition
				position = len(f.Parameters) + 1
			case "initializer arm":
				target = &third.Initializer.Conditional.WhenTrue
				position--
			}
			value := *target
			reference := &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: reference}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("internal Core must retain local expressions: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
