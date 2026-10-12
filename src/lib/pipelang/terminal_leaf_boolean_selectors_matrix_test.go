package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
)

// The three routing bits are independent of the selector and initializer bits.
// Bits are shared by depth/across initializers; this is not an arbitrary assignment
// to every expression in an unbounded source program.
func TestV990TerminalLeafBooleanSelectorsSubsets(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("statement inventory drift")
	}
	for shape := 0; shape < 100; shape++ {
		tree := trees[shape/4]
		partition := shape % 4
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			enterFiniteShape(t)
			leaves := v990Leaves(tree, "R")
			lower, upper := (1<<len(leaves))*partition/4, (1<<len(leaves))*(partition+1)/4
			for start := lower; start < upper; start += 8 {
				end := min(start+8, upper)
				source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;`
				names := []string{}
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					names = append(names, name)
					source += v990MatrixMethod(name, tree, leaves, subset, 0, false)
				}
				source += "}"
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, source, names)
				generated, err := gobackend.Generate(p)
				if err != nil {
					t.Fatal(err)
				}
				prepared := prepareConformanceProgram(t, p)
				var checks, orders strings.Builder
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					f := coreFunctionNamed(t, p, name)
					// No locals: initializer bits are unused and fixed at false here.
					for mask := 0; mask < 64; mask++ {
						want, trace := v990MatrixOracle(tree, leaves, subset, 0, false, mask)
						args := []coreeval.Value{}
						call := "PipeLang" + name + "("
						for bit := 0; bit < 9; bit++ {
							flag := mask&(1<<bit) != 0
							args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: flag})
							if bit > 0 {
								call += ","
							}
							call += fmt.Sprint(flag)
						}
						call += ")"
						got, err := prepared.Evaluate(f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("subset %d mask %d: %v %v want %q", subset, mask, got, err, want)
						}
						fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
						fmt.Fprintf(&orders, "v990MatrixTrace=nil;%s;if !reflect.DeepEqual(v990MatrixTrace,[]string{%s}){t.Fatal(v990MatrixTrace)}\n", call, quotedStrings(trace))
					}
				}
				v990RunMatrixGo(t, generated, checks.String(), orders.String())
			}
			t.Logf("%d leaves, %d subsets, %d independent routing/selector vectors per subset", len(leaves), upper-lower, 64)
		})
	}
}

func TestV990TerminalLeafBooleanSelectorsScopeLayouts(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("statement inventory drift")
	}
	for shape, tree := range trees {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			enterFiniteShape(t)
			leaves := v990Leaves(tree, "R")
			for _, count := range []int{1, 3} {
				for _, unused := range []bool{false, true} {
					source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;` + v990MatrixMethod("Select", tree, leaves, (1<<len(leaves))-1, count, unused) + "}"
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, source, []string{"Select"})
					generated, err := gobackend.Generate(p)
					if err != nil {
						t.Fatal(err)
					}
					f := coreFunctionNamed(t, p, "Select")
					prepared := prepareConformanceProgram(t, p)
					var checks, orders strings.Builder
					for mask := 0; mask < 512; mask++ {
						want, trace := v990MatrixOracle(tree, leaves, (1<<len(leaves))-1, count, unused, mask)
						args := []coreeval.Value{}
						call := "PipeLangSelect("
						for bit := 0; bit < 9; bit++ {
							flag := mask&(1<<bit) != 0
							args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: flag})
							if bit > 0 {
								call += ","
							}
							call += fmt.Sprint(flag)
						}
						call += ")"
						got, err := prepared.Evaluate(f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("mask %d: %v %v want %q", mask, got, err, want)
						}
						fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
						fmt.Fprintf(&orders, "v990MatrixTrace=nil;%s;if !reflect.DeepEqual(v990MatrixTrace,[]string{%s}){t.Fatal(v990MatrixTrace)}\n", call, quotedStrings(trace))
					}
					v990RunMatrixGo(t, generated, checks.String(), orders.String())
				}
			}
		})
	}
}

func v990Leaves(tree *terminalTree, path string) []string {
	if tree == nil {
		return []string{path}
	}
	return append(v990Leaves(tree.yes, path+"T"), v990Leaves(tree.no, path+"F")...)
}
func v990MatrixMethod(name string, tree *terminalTree, leaves []string, subset, count int, unused bool) string {
	var emit func(*terminalTree, string, string) string
	emit = func(n *terminalTree, path, previous string) string {
		body := ""
		for i := 0; i < count; i++ {
			local := fmt.Sprintf("l%s%d", path, i)
			tag := fmt.Sprintf("%s%d", path, i)
			init := fmt.Sprintf("Echo(%s+%q)", previous, tag+"O")
			if i%2 == 0 {
				init = fmt.Sprintf(`Check(%q,q) ? (Check(%q,r) ? (Check(%q,s) ? Echo(%s+%q) : Echo(%s+%q)) : Echo(%s+%q)) : Echo(%s+%q)`, tag+"q", tag+"r", tag+"s", previous, tag+"T", previous, tag+"M", previous, tag+"N", previous, tag+"F")
			}
			body += "string " + local + "=" + init + ";"
			if !unused || i < count-1 {
				previous = local
			}
		}
		if n != nil {
			return body + fmt.Sprintf(`if(Check(%q,d%d)){%s}else{%s}`, path, len(path)-1, emit(n.yes, path+"T", previous), emit(n.no, path+"F", previous))
		}
		selected := false
		for i, p := range leaves {
			if p == path {
				selected = subset&(1<<i) != 0
			}
		}
		if selected {
			return body + fmt.Sprintf(`return (Check(%q,a) ? Check(%q,b) : Check(%q,c)) ? Echo(%s+%q) : Echo(%s+%q);`, path+"a", path+"b", path+"c", previous, path+"Y", previous, path+"Z")
		}
		// Alternate ordinary and inherited depth-three returns outside the new subset.
		if len(path)%2 == 0 {
			return body + fmt.Sprintf(`return a ? (b ? (c ? Echo(%s+%q) : Echo(%s+%q)) : Echo(%s+%q)) : Echo(%s+%q);`, previous, path+"I", previous, path+"I", previous, path+"I", previous, path+"I")
		}
		return body + fmt.Sprintf(`return Echo(%s+%q);`, previous, path+"I")
	}
	return fmt.Sprintf("public string %s(bool d0,bool d1,bool d2,bool a,bool b,bool c,bool q,bool r,bool s){%s}", name, emit(tree, "R", `""`))
}
func v990MatrixOracle(tree *terminalTree, leaves []string, subset, count int, unused bool, mask int) (string, []string) {
	path, value := "R", ""
	trace := []string{}
	for depth := 0; ; depth++ {
		for i := 0; i < count; i++ {
			tag := fmt.Sprintf("%s%d", path, i)
			suffix := "O"
			if i%2 == 0 {
				trace = append(trace, "C:"+tag+"q")
				suffix = "F"
				if mask&64 != 0 {
					trace = append(trace, "C:"+tag+"r")
					suffix = "N"
					if mask&128 != 0 {
						trace = append(trace, "C:"+tag+"s")
						suffix = "M"
						if mask&256 != 0 {
							suffix = "T"
						}
					}
				}
			}
			next := value + tag + suffix
			trace = append(trace, "E:"+next)
			if !unused || i < count-1 {
				value = next
			}
		}
		if tree == nil {
			break
		}
		trace = append(trace, "C:"+path)
		if mask&(1<<depth) != 0 {
			tree = tree.yes
			path += "T"
		} else {
			tree = tree.no
			path += "F"
		}
	}
	selected := false
	for i, p := range leaves {
		if p == path {
			selected = subset&(1<<i) != 0
		}
	}
	suffix := "I"
	if selected {
		trace = append(trace, "C:"+path+"a")
		choose := mask&32 != 0
		if mask&8 != 0 {
			trace = append(trace, "C:"+path+"b")
			choose = mask&16 != 0
		} else {
			trace = append(trace, "C:"+path+"c")
		}
		suffix = "Z"
		if choose {
			suffix = "Y"
		}
	}
	value += path + suffix
	trace = append(trace, "E:"+value)
	return value, trace
}
func v990RunMatrixGo(t *testing.T, generated []byte, checks, orders string) {
	t.Helper()
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks)))
	observed := string(generated)
	for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v990MatrixTrace=append(v990MatrixTrace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v990MatrixTrace=append(v990MatrixTrace,"C:"+p0)`} {
		if strings.Count(observed, marker) != 1 {
			t.Fatal("missing trace marker")
		}
		observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
	}
	compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v990MatrixTrace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders)))
}
