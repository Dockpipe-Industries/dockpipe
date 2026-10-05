package pipelang

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	gotoken "go/token"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/cppbackend"
	"dockpipe/src/lib/pipelang/gobackend"
)

type cppVector struct {
	Method   string
	Args     []any
	Expected string
	Trace    []string
}
type cppCase struct {
	ID, Source string
	Vectors    []cppVector
}

const cppRow = `public Record Row { public string Text; public bool Active; } `

func cppText(s string) string { return "s" + hex.EncodeToString([]byte(s)) }
func cppInt(v int64) string   { return "i" + strconv.FormatInt(v, 10) }
func cppBool(v bool) string {
	if v {
		return "b1"
	}
	return "b0"
}
func cppRecord(v []any) string {
	return "r[" + cppText(v[0].(string)) + ";" + cppBool(v[1].(bool)) + ";]"
}
func cppList(v []any) string {
	r := "l["
	for _, row := range v {
		r += cppRecord(row.([]any)) + ";"
	}
	return r + "]"
}
func cppArithmetic(op string, a, b int64) string {
	x := big.NewInt(a)
	switch op {
	case "Add", "Forward":
		x.Add(x, big.NewInt(b))
	case "Subtract":
		x.Sub(x, big.NewInt(b))
	case "Multiply":
		x.Mul(x, big.NewInt(b))
	case "Negate":
		x.Neg(x)
	}
	if !x.IsInt64() {
		return "err:overflow"
	}
	return "ok:" + cppInt(x.Int64())
}
func cppCorpus() []cppCase {
	cases := []cppCase{
		{ID: "N1", Source: `public Class Root {public Result<int, ArithmeticError> Add(int a,int b)=>a+b; public Result<int, ArithmeticError> Subtract(int a,int b)=>a-b; public Result<int, ArithmeticError> Multiply(int a,int b)=>a*b; public Result<int, ArithmeticError> Negate(int a)=>-a;}`},
		{ID: "N2", Source: `public Class Root {public Result<int, ArithmeticError> Add(int a,int b)=>a+b; public Result<int, ArithmeticError> Forward(int a,int b)=>Add(a,b);}`},
		{ID: "C1", Source: `public Class Root {public bool Mark(bool value)=>value; public bool Choose(bool a,bool b)=>Mark(a) && Mark(b);}`},
		{ID: "C2", Source: `public Class Root {public Result<int, ArithmeticError> Add(int x,int y)=>x+y;public Result<int, ArithmeticError> Negate(int x)=>-x;public Result<int, ArithmeticError> Choose(bool a,bool b,int x,int y) {bool pick = a && b; Result<int, ArithmeticError> selected = pick ? Add(x,y) : Negate(x);return selected;}}`},
		{ID: "A1", Source: cppRow + `public Class Root {public Row Create(string text,bool active)=>new Row {Text=text,Active=active}; public string Project(Row row)=>row.Text; public bool Equal(Row a,Row b)=>a==b; public bool Before(string a,string b)=>a<b;}`},
		{ID: "A2", Source: cppRow + `public Class Root {public List<Row> Append(List<Row> rows,Row row)=>append(rows,row); public int Count(List<Row> rows)=>count(rows); public List<Row> One(Row row)=>list(row); public List<Row> Empty()=>empty_list<Row>();}`},
		{ID: "A3", Source: cppRow + `public Class Root {public Optional<Row> At(List<Row> rows,int index)=>at(rows,index);}`},
		{ID: "A4", Source: cppRow + `public Class Root {public List<Row> Filter(List<Row> rows,string key)=>filter_by(rows,Row.Text,key);}`},
	}
	edges := [][2]int64{{0, 0}, {1, -1}, {-1, 1}, {1<<63 - 1, 1}, {-1 << 63, -1}, {1<<63 - 1, -1}, {-1 << 63, 2}, {3037000500, 3037000500}, {-99, 7}, {42, 6}, {-1 << 63, 0}, {1<<63 - 1, -1}, {-3, -7}, {0, 1<<63 - 1}, {0, -1 << 63}, {1, -1 << 63}, {-1, -1 << 63}, {7, 0}, {-7, 3}, {-1 << 63, -1}}
	texts := []string{"", "plain", "é", "e\u0301", "\x00", strings.Repeat("long-λ", 20), "\U00010000", "\uE000", "same", "same"}
	for i, pair := range edges {
		op := []string{"Add", "Subtract", "Multiply", "Negate"}[i%4]
		args := []any{pair[0], pair[1]}
		if op == "Negate" {
			args = args[:1]
		}
		cases[0].Vectors = append(cases[0].Vectors, cppVector{op, args, cppArithmetic(op, pair[0], pair[1]), []string{op}})
		cases[1].Vectors = append(cases[1].Vectors, cppVector{"Forward", []any{pair[0], pair[1]}, cppArithmetic("Forward", pair[0], pair[1]), []string{"Forward", "Add"}})
		a, b := i%2 == 0, (i/2)%2 == 0
		trace := []string{"Choose", "Mark"}
		if a {
			trace = append(trace, "Mark")
		}
		cases[2].Vectors = append(cases[2].Vectors, cppVector{"Choose", []any{a, b}, cppBool(a && b), trace})
		expected := cppArithmetic("Negate", pair[0], 0)
		if a && b {
			expected = cppArithmetic("Add", pair[0], pair[1])
		}
		arm := "Negate"
		if a && b {
			arm = "Add"
		}
		cases[3].Vectors = append(cases[3].Vectors, cppVector{"Choose", []any{a, b, pair[0], pair[1]}, expected, []string{"Choose", arm}})
		s, t := texts[i%len(texts)], texts[(i+1)%len(texts)]
		row := []any{s, a}
		other := []any{t, b}
		if i == 18 {
			other = append([]any{}, row...)
		}
		method := []string{"Create", "Project", "Equal", "Before"}[i%4]
		var inputs []any
		switch method {
		case "Create":
			inputs = []any{s, a}
			expected = cppRecord(row)
		case "Project":
			inputs = []any{row}
			expected = cppText(s)
		case "Equal":
			inputs = []any{row, other}
			expected = cppBool(reflect.DeepEqual(row, other))
		case "Before":
			inputs = []any{s, t}
			expected = cppBool(s < t)
		}
		cases[4].Vectors = append(cases[4].Vectors, cppVector{method, inputs, expected, []string{method}})
		n := []int{0, 1, 16, 256}[i%4]
		rows := []any{}
		for k := 0; k < n; k++ {
			rows = append(rows, []any{texts[(k+i)%len(texts)], k%2 == 0})
		}
		method = []string{"Append", "Count", "One", "Empty"}[(i/4)%4]
		switch method {
		case "Append":
			inputs = []any{rows, row}
			expected = cppList(append(append([]any{}, rows...), row))
		case "Count":
			inputs = []any{rows}
			expected = cppInt(int64(len(rows)))
		case "One":
			inputs = []any{row}
			expected = cppList([]any{row})
		case "Empty":
			inputs = nil
			expected = cppList(nil)
		}
		cases[5].Vectors = append(cases[5].Vectors, cppVector{method, inputs, expected, []string{method}})
		ix := []int64{-1, 0, int64(n) - 1, int64(n), 1<<63 - 1}[i%5]
		expected = "none"
		if ix >= 0 && ix < int64(n) {
			expected = "some:" + cppRecord(rows[ix].([]any))
		}
		cases[6].Vectors = append(cases[6].Vectors, cppVector{"At", []any{rows, ix}, expected, []string{"At"}})
		selected := []any{}
		for _, v := range rows {
			if v.([]any)[0].(string) == s {
				selected = append(selected, v)
			}
		}
		cases[7].Vectors = append(cases[7].Vectors, cppVector{"Filter", []any{rows, s}, cppList(selected), []string{"Filter"}})
	}
	return cases
}
func cppCoreValue(t coreir.Type, raw any) coreeval.Value {
	v := coreeval.Value{Type: t}
	switch t.Kind {
	case coreir.TypeNumeric:
		v.Int = raw.(int64)
	case coreir.TypePrimitive:
		if t.Primitive == coreir.PrimitiveBool {
			v.Bool = raw.(bool)
		} else {
			v.String = raw.(string)
		}
	case coreir.TypeRecord:
		for i, f := range t.Record.Fields {
			v.Record = append(v.Record, cppCoreValue(f.Type, raw.([]any)[i]))
		}
	case coreir.TypeList:
		v.List = []coreeval.Value{}
		for _, x := range raw.([]any) {
			v.List = append(v.List, cppCoreValue(t.List.Element, x))
		}
	}
	return v
}
func cppWire(v coreeval.Value) string {
	switch v.Type.Kind {
	case coreir.TypeNumeric:
		return cppInt(v.Int)
	case coreir.TypePrimitive:
		if v.Type.Primitive == coreir.PrimitiveBool {
			return cppBool(v.Bool)
		}
		return cppText(v.String)
	case coreir.TypeRecord:
		r := "r["
		for _, f := range v.Record {
			r += cppWire(f) + ";"
		}
		return r + "]"
	case coreir.TypeList:
		r := "l["
		for _, f := range v.List {
			r += cppWire(f) + ";"
		}
		return r + "]"
	case coreir.TypeOptional:
		if !v.Optional.Present {
			return "none"
		}
		return "some:" + cppWire(*v.Optional.Value)
	}
	panic("unsupported oracle type")
}
func cppInput(b *bytes.Buffer, t coreir.Type, x any) {
	switch t.Kind {
	case coreir.TypeNumeric:
		binary.Write(b, binary.LittleEndian, x.(int64))
	case coreir.TypePrimitive:
		if t.Primitive == coreir.PrimitiveBool {
			if x.(bool) {
				b.WriteByte(1)
			} else {
				b.WriteByte(0)
			}
		} else {
			s := x.(string)
			binary.Write(b, binary.LittleEndian, uint32(len(s)))
			b.WriteString(s)
		}
	case coreir.TypeRecord:
		for i, f := range t.Record.Fields {
			cppInput(b, f.Type, x.([]any)[i])
		}
	case coreir.TypeList:
		xs := x.([]any)
		binary.Write(b, binary.LittleEndian, uint32(len(xs)))
		for _, v := range xs {
			cppInput(b, t.List.Element, v)
		}
	}
}
func TestCPPPilotExport(t *testing.T) {
	root := os.Getenv("PIPELANG_CPP_PILOT_OUTPUT")
	if root == "" {
		t.Skip("explicit task-owned pilot output required")
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	corpus := cppCorpus()
	data, _ := json.MarshalIndent(corpus, "", "  ")
	if e := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
	for _, c := range corpus {
		t.Run(c.ID, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", c.ID+".pipe", c.Source)}, nil)
			input.LanguageContract = map[string]LanguageContract{"N1": PipeLangLanguageContractV1130, "N2": PipeLangLanguageContractV1130, "C1": PipeLangLanguageContractV1130, "C2": PipeLangLanguageContractV1130, "A1": PipeLangLanguageContractV120, "A2": PipeLangLanguageContractV170, "A3": PipeLangLanguageContractV200, "A4": PipeLangLanguageContractV220}[c.ID]
			analysis := AnalyzeSemanticModuleSet(input)
			if e := analysis.Error(); e != nil {
				t.Fatal(e)
			}
			p := coreir.Program{LanguageContract: string(input.LanguageContract), CompilerContract: coreir.CompilerContractV1}
			seen := map[string]bool{}
			top := map[string]coreir.Function{}
			for _, v := range c.Vectors {
				if _, ok := top[v.Method]; ok {
					continue
				}
				method := semanticMethodNamed(t, analysis, v.Method)
				h, e := LowerSemanticMethodToHIR(analysis, method.Identity)
				if e != nil {
					t.Fatal(e)
				}
				co, e := LowerHIRToCore(h)
				if e != nil {
					t.Fatal(e)
				}
				for _, f := range co.Functions {
					if !seen[f.Identity.Path] {
						seen[f.Identity.Path] = true
						p.Functions = append(p.Functions, f)
					}
					if f.Name == v.Method {
						top[v.Method] = f
					}
				}
			}
			sort.Slice(p.Functions, func(i, j int) bool { return p.Functions[i].Identity.Path < p.Functions[j].Identity.Path })
			generated, e := cppbackend.GenerateWithNames(p)
			if e != nil {
				t.Fatal(e)
			}
			again, e := cppbackend.Generate(p)
			if e != nil || !bytes.Equal(again, generated.Source) {
				t.Fatal("nondeterministic C++")
			}
			goGenerated, e := gobackend.GenerateWithNames(p)
			if e != nil {
				t.Fatal(e)
			}
			dir := filepath.Join(root, c.ID)
			if e := os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
			write := func(n string, b []byte) {
				if e := os.WriteFile(filepath.Join(dir, n), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			co, _ := json.MarshalIndent(p, "", "  ")
			write("core.json", co)
			write("source.pipe", []byte(c.Source))
			write("generated.hpp", generated.Source)
			write("generated.go", bytes.Replace(goGenerated.Source, []byte("package pipelanggenerated"), []byte("package main"), 1))
			goNames, cppNames := map[string]string{}, map[string]string{}
			for _, f := range p.Functions {
				for _, b := range goGenerated.Functions {
					if b.Identity.Path == f.Identity.Path {
						goNames[f.Name] = b.Name
					}
				}
				for _, b := range generated.Functions {
					if b.Identity.Path == f.Identity.Path {
						cppNames[f.Name] = b.Name
					}
				}
			}
			methods := []string{}
			for k := range top {
				methods = append(methods, k)
			}
			sort.Strings(methods)
			var fixture bytes.Buffer
			binary.Write(&fixture, binary.LittleEndian, uint32(len(c.Vectors)))
			expected := []string{}
			for _, v := range c.Vectors {
				f := top[v.Method]
				args := []coreeval.Value{}
				idx := sort.SearchStrings(methods, v.Method)
				fixture.WriteByte(byte(idx))
				for i, a := range v.Args {
					args = append(args, cppCoreValue(f.Parameters[i].Type, a))
					cppInput(&fixture, f.Parameters[i].Type, a)
				}
				o, e := coreeval.EvaluateProgram(p, f.Identity, args)
				if e != nil {
					t.Fatal(e)
				}
				actual := ""
				if f.ReturnType.Kind == coreir.TypeResult {
					if o.OK {
						actual = "ok:" + cppWire(o.Value)
					} else {
						actual = "err:" + string(o.Error)
					}
				} else {
					if !o.OK {
						t.Fatal("unexpected failure")
					}
					actual = cppWire(o.Value)
				}
				if actual != v.Expected {
					t.Fatalf("%s evaluator=%s oracle=%s", v.Method, actual, v.Expected)
				}
				expected = append(expected, v.Expected+"|"+strings.Join(v.Trace, ","))
			}
			write("inputs.bin", fixture.Bytes())
			out, _ := json.MarshalIndent(expected, "", "  ")
			write("expected.json", out)
			gm, cm := cppPilotMains(t, p, methods, top, goNames, cppNames, goGenerated.Source)
			write("main.go", []byte(gm))
			write("main.cpp", []byte(cm))
			// Test-only function-entry observations. Neither production emitter has hooks.
			gs := string(bytes.Replace(goGenerated.Source, []byte("package pipelanggenerated"), []byte("package main"), 1))
			fs := gotoken.NewFileSet()
			tree, e := goparser.ParseFile(fs, "generated.go", gs, 0)
			if e != nil {
				t.Fatal(e)
			}
			type insertion struct {
				pos  int
				text string
			}
			edits := []insertion{}
			for _, d := range tree.Decls {
				if f, ok := d.(*ast.FuncDecl); ok {
					for method, n := range goNames {
						if n == f.Name.Name {
							edits = append(edits, insertion{fs.Position(f.Body.Lbrace).Offset + 1, "\npilotTrace=append(pilotTrace," + strconv.Quote(method) + ");\n"})
						}
					}
				}
			}
			sort.Slice(edits, func(i, j int) bool { return edits[i].pos > edits[j].pos })
			for _, ed := range edits {
				gs = gs[:ed.pos] + ed.text + gs[ed.pos:]
			}
			write("traced.go", []byte(gs))
			cs := string(generated.Source)
			for method, n := range cppNames {
				pattern := regexp.MustCompile(`(?m)^inline [^\n]+\b` + regexp.QuoteMeta(n) + `\([^;\n]*\) \{`)
				matches := pattern.FindAllStringIndex(cs, -1)
				if len(matches) != 1 {
					t.Fatalf("C++ definition %s matches %d", method, len(matches))
				}
				pos := matches[0][1]
				cs = cs[:pos] + "\n::pilot_trace(" + strconv.Quote(method) + ");\n" + cs[pos:]
			}
			write("traced.hpp", []byte("void pilot_trace(const char*);\n"+cs))
			t.Logf("%s: 20 frontend/HIR/Core/evaluator/oracle vectors exported", c.ID)
		})
	}
}

func cppPilotMains(t *testing.T, p coreir.Program, methods []string, top map[string]coreir.Function, gn, cn map[string]string, source []byte) (string, string) {
	// Derive Go host types from declarations actually allocated by its backend.
	fs := gotoken.NewFileSet()
	tree, e := goparser.ParseFile(fs, "generated.go", source, 0)
	if e != nil {
		t.Fatal(e)
	}
	types := map[string]string{}
	paramTypes := map[string][]string{}
	for _, d := range tree.Decls {
		if f, ok := d.(*ast.FuncDecl); ok {
			for method, n := range gn {
				if f.Name.Name != n {
					continue
				}
				for _, field := range f.Type.Params.List {
					var b bytes.Buffer
					format.Node(&b, fs, field.Type)
					paramTypes[method] = append(paramTypes[method], b.String())
				}
			}
		}
	}
	var register func(coreir.Type, string)
	register = func(ct coreir.Type, gt string) {
		j, _ := json.Marshal(ct)
		types[string(j)] = gt
		if ct.Kind == coreir.TypeList {
			register(ct.List.Element, strings.TrimPrefix(gt, "[]"))
		}
	}
	for m, f := range top {
		for i, param := range f.Parameters {
			register(param.Type, paramTypes[m][i])
		}
	}
	gm := cppGoMainSupport
	cm := cppMainSupport
	records := map[string]coreir.Type{}
	var collect func(reflect.Value)
	collect = func(v reflect.Value) {
		if v.Kind() == reflect.Pointer {
			if !v.IsNil() {
				collect(v.Elem())
			}
			return
		}
		if v.Type() == reflect.TypeOf(coreir.Type{}) {
			ct := v.Interface().(coreir.Type)
			if ct.Kind == coreir.TypeRecord {
				n, _ := cppbackend.TypeName(ct)
				records[n] = ct
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				collect(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				collect(v.Index(i))
			}
		}
	}
	collect(reflect.ValueOf(p))
	keys := []string{}
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, n := range keys {
		ct := records[n]
		cm += "namespace pipelang { inline std::string wire(const " + n + "& x){return std::string(\"r[\")"
		for i := range ct.Record.Fields {
			cm += fmt.Sprintf("+wire(x.f%d)+\";\"", i)
		}
		cm += "+\"]\";} }\n"
		cm += "template<> struct Reader<pipelang::" + n + "> {static pipelang::" + n + " get(std::istream& in){pipelang::" + n + " x;"
		for i, f := range ct.Record.Fields {
			tn, _ := cppbackend.TypeName(f.Type)
			cm += fmt.Sprintf("x.f%d=Reader<%s>::get(in);", i, tn)
		}
		cm += "return x;}};\n"
	}
	gm += "func main(){defer func(){if recover()!=nil{os.Exit(3)}}();r,e:=os.Open(os.Args[1]);if e!=nil{panic(e)};defer r.Close();var n uint32;must(binary.Read(r,binary.LittleEndian,&n));if n>1000{panic(\"bound\")};for i:=uint32(0);i<n;i++{pilotTrace=nil;tag:=read[uint8](r);switch tag {\n"
	cm += "int main(int argc,char** argv){try {if(argc!=2)return 3;std::ifstream in(argv[1],std::ios::binary);auto n=Reader<std::uint32_t>::get(in);if(n>1000)throw std::runtime_error(\"bound\");for(std::uint32_t i=0;i<n;++i){trace.clear();auto tag=Reader<std::uint8_t>::get(in);switch(tag){\n"
	for ix, m := range methods {
		f := top[m]
		gm += fmt.Sprintf("case %d:\n", ix)
		cm += fmt.Sprintf("case %d:{\n", ix)
		args := []string{}
		for i, a := range f.Parameters {
			n := fmt.Sprintf("a%d", i)
			args = append(args, n)
			gm += fmt.Sprintf("%s:=read[%s](r)\n", n, paramTypes[m][i])
			ct, _ := cppbackend.TypeName(a.Type)
			cm += fmt.Sprintf("auto %s=Reader<%s>::get(in);\n", n, ct)
		}
		gm += "v:=" + gn[m] + "(" + strings.Join(args, ",") + ");fmt.Println(wire(reflect.ValueOf(v))+\"|\"+strings.Join(pilotTrace,\",\"))\n"
		cm += "auto v=pipelang::" + cn[m] + "(" + strings.Join(args, ",") + ");std::cout<<pipelang::wire(v)<<\"|\"<<trace<<'\\n';break;}\n"
	}
	gm += "default:panic(\"tag\")}};var tail [1]byte;if n,_:=r.Read(tail[:]);n!=0{panic(\"trailing bytes\")}}\n"
	cm += "default:throw std::runtime_error(\"tag\");}}if(in.peek()!=std::char_traits<char>::eof())throw std::runtime_error(\"trailing bytes\");return 0;}catch(const std::exception&){return 3;}}\n"
	return gm, cm
}

const cppGoMainSupport = `package main
import("encoding/binary";"encoding/hex";"fmt";"io";"os";"reflect";"strings")
var pilotTrace []string
func must(e error){if e!=nil{panic(e)}}
func read[T any](r io.Reader)T{var x T;fill(r,reflect.ValueOf(&x).Elem());return x}
func fill(r io.Reader,v reflect.Value){switch v.Kind(){case reflect.Uint8:var b [1]byte;_,e:=io.ReadFull(r,b[:]);must(e);v.SetUint(uint64(b[0]));case reflect.Int64:var n int64;must(binary.Read(r,binary.LittleEndian,&n));v.SetInt(n);case reflect.Bool:b:=read[uint8](r);if b>1{panic("bool")};v.SetBool(b==1);case reflect.String:var n uint32;must(binary.Read(r,binary.LittleEndian,&n));if n>1<<20{panic("bound")};b:=make([]byte,n);_,e:=io.ReadFull(r,b);must(e);v.SetString(string(b));case reflect.Struct:for i:=0;i<v.NumField();i++{fill(r,v.Field(i))};case reflect.Slice:var n uint32;must(binary.Read(r,binary.LittleEndian,&n));if n>4096{panic("bound")};v.Set(reflect.MakeSlice(v.Type(),int(n),int(n)));for i:=0;i<int(n);i++{fill(r,v.Index(i))};default:panic("unsupported input")}}
func wire(v reflect.Value)string{if v.Kind()==reflect.Interface{return wire(v.Elem())};switch v.Kind(){case reflect.Bool:if v.Bool(){return "b1"};return "b0";case reflect.Int64:return fmt.Sprint("i",v.Int());case reflect.String:return "s"+hex.EncodeToString([]byte(v.String()));case reflect.Slice:r:="l[";for i:=0;i<v.Len();i++{r+=wire(v.Index(i))+";"};return r+"]";case reflect.Struct:if f:=v.FieldByName("OK");f.IsValid(){if f.Bool(){return "ok:"+wire(v.FieldByName("Value"))};return "err:"+v.FieldByName("Error").String()};if strings.HasPrefix(v.Type().Name(),"pipelangOptionalNone"){return "none"};if strings.HasPrefix(v.Type().Name(),"pipelangOptionalSome"){return "some:"+wire(v.Field(0))};r:="r[";for i:=0;i<v.NumField();i++{r+=wire(v.Field(i))+";"};return r+"]"};panic("unsupported output")}
`
const cppMainSupport = `#include "active.hpp"
#include <fstream>
#include <iostream>
#include <cstring>
std::string trace;
void pilot_trace(const char* name){if(!trace.empty())trace+=",";trace+=name;}
namespace pipelang {
inline std::string wire(bool v){return v?"b1":"b0";}
inline std::string wire(std::int64_t v){return "i"+std::to_string(v);}
inline std::string wire(const std::string& v){const char* h="0123456789abcdef";std::string out="s";for(unsigned char c:v){out+=h[c>>4];out+=h[c&15];}return out;}
template<class T> std::string wire(const std::vector<T>&);
template<class T> std::string wire(const Optional<T>&);
template<class T> std::string wire(const Result<T>&);
template<class T> std::string wire(const std::vector<T>& v){std::string out="l[";for(const auto& x:v)out+=wire(x)+";";return out+"]";}
template<class T> std::string wire(const Optional<T>& v){return v.present?"some:"+wire(v.value):"none";}
template<class T> std::string wire(const Result<T>& v){if(v.ok)return "ok:"+wire(v.value);return v.error==Error::overflow?"err:overflow":"err:division_by_zero";}
}
template<class T> struct Reader {static T get(std::istream& in){static_assert(std::is_integral<T>::value,"primitive reader");unsigned char b[sizeof(T)];if(!in.read(reinterpret_cast<char*>(b),sizeof(T)))throw std::runtime_error("short input");std::uint64_t v=0;for(std::size_t i=0;i<sizeof(T);i++)v|=std::uint64_t(b[i])<<(8*i);T out;std::memcpy(&out,&v,sizeof(T));return out;}};
template<> struct Reader<bool>{static bool get(std::istream& in){auto b=Reader<std::uint8_t>::get(in);if(b>1)throw std::runtime_error("bool");return b==1;}};
template<> struct Reader<std::string>{static std::string get(std::istream& in){auto n=Reader<std::uint32_t>::get(in);if(n>1<<20)throw std::runtime_error("bound");std::string s(n,'\0');if(!in.read(&s[0],n))throw std::runtime_error("short input");return s;}};
template<class T> struct Reader<std::vector<T>>{static std::vector<T> get(std::istream& in){auto n=Reader<std::uint32_t>::get(in);if(n>4096)throw std::runtime_error("bound");std::vector<T> out;for(std::uint32_t i=0;i<n;i++)out.push_back(Reader<T>::get(in));return out;}};
using namespace pipelang;
`

func TestCPPPilotCoreRefusals(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "negative.pipe", checkedAddSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV1130
	analysis := AnalyzeSemanticModuleSet(input)
	if e := analysis.Error(); e != nil {
		t.Fatal(e)
	}
	method := semanticMethodNamed(t, analysis, "Add")
	h, e := LowerSemanticMethodToHIR(analysis, method.Identity)
	if e != nil {
		t.Fatal(e)
	}
	base, e := LowerHIRToCore(h)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(base)
	changes := []func(*coreir.Program){
		func(p *coreir.Program) { p.LanguageContract = "v0.112.0" },
		func(p *coreir.Program) { p.CompilerContract = "unknown" },
		func(p *coreir.Program) { p.Functions[0].Body.Kind = "unknown" },
		func(p *coreir.Program) { p.Functions[0].Body.Binary = nil },
		func(p *coreir.Program) { p.Functions[0].Body.Binary.Left = nil },
		func(p *coreir.Program) { p.Functions[0].Body.Binary.Operator = coreir.OperatorDivide },
		func(p *coreir.Program) { p.Functions[0].Parameters[0].Type = coreir.BinaryFloat(64) },
		func(p *coreir.Program) { p.Functions[0].Parameters[0].Type = coreir.SignedInteger(32) },
		func(p *coreir.Program) { p.Functions[0].Parameters[0].Position = 9 },
		func(p *coreir.Program) { p.Functions[0].Identity.Path = "" },
		func(p *coreir.Program) { p.Functions = append(p.Functions, p.Functions[0]) },
		func(p *coreir.Program) { *p.Functions[0].Body.Binary.Left.Parameter = 99 },
		func(p *coreir.Program) { p.Functions[0].Body.Type = coreir.SignedInteger(64) },
		func(p *coreir.Program) { p.Functions[0].Body.Binary.Left = &p.Functions[0].Body },
		func(p *coreir.Program) { p.Functions[0].Body.Kind = coreir.ExprTextTrim },
		func(p *coreir.Program) { p.Functions[0].ReturnType = coreir.Type{Kind: coreir.TypeEnum} },
	}
	for i, change := range changes {
		var p coreir.Program
		if e := json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		change(&p)
		source, e := cppbackend.Generate(p)
		if e == nil || len(source) != 0 {
			t.Fatalf("negative Core %d was emitted", i)
		}
	}
	// The pilot plan's integer division was outside the accepted source contract.
	invalid := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "division.pipe", `public Class Root {public Result<int, ArithmeticError> Divide(int a,int b)=>a/b;}`)}, nil)
	invalid.LanguageContract = PipeLangLanguageContractV1130
	if AnalyzeSemanticModuleSet(invalid).Error() == nil {
		t.Fatal("integer division unexpectedly accepted; revise capability manifest")
	}
}
