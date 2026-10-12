package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestV1150AssignmentValues(t *testing.T) {
	cases := []struct {
		name, body string
		want       [2]int64
	}{
		{"reassign", `mutable int x=1; x=2; return x;`, [2]int64{2, 2}},
		{"delayed_mutable", `mutable int x; x=3; x=4; return x;`, [2]int64{4, 4}},
		{"delayed_immutable", `int x; if(flag){x=5;}else{x=6;} return x;`, [2]int64{6, 5}},
		{"early_true", `int x; if(flag){return 7;}else{x=8;} return x;`, [2]int64{8, 7}},
		{"early_false", `int x; if(flag){x=9;}else{return 10;} return x;`, [2]int64{10, 9}},
		{"early_join", `int x; if(flag){x=11; return x;} x=12; return x;`, [2]int64{12, 11}},
		{"nested_write", `mutable int x=13; {x=14; if(flag){x=15;}} return x;`, [2]int64{14, 15}},
		{"nested_delayed", `int x; {{x=16;}} return x;`, [2]int64{16, 16}},
		{"overwrite_partial", `mutable int x; if(flag){x=17;} x=18; return x;`, [2]int64{18, 18}},
		{"branch_isolation", `int x; if(flag){x=19;}else{x=20;} return x;`, [2]int64{20, 19}},
		{"snapshot", `mutable int x=21; int old=x; x=22; return old;`, [2]int64{21, 21}},
		{"scope_reuse", `{mutable int x=1; x=2;} int x; x=23; return x;`, [2]int64{23, 23}},
		{"unused_delayed", `int x; mutable string text; return 24;`, [2]int64{24, 24}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, `public Class Choices{public int Select(bool flag){`+c.body+`}}`, []string{"Select"})
			f := coreFunctionNamed(t, p, "Select")
			prepared, err := coreeval.PrepareProgram(p)
			if err != nil {
				t.Fatal(err)
			}
			// Alternate repeated invocations to expose leaked mutable invocation state.
			for _, i := range []int{0, 1, 1, 0} {
				args := []coreeval.Value{{Type: f.Parameters[0].Type, Bool: i == 1}}
				before, _ := json.Marshal(args)
				got, err := prepared.Evaluate(f.Identity, args)
				if err != nil || !got.OK || got.Value.Int != c.want[i] {
					t.Fatal(got, err, c.want[i])
				}
				after, _ := json.Marshal(args)
				if !bytes.Equal(before, after) {
					t.Fatal("caller arguments mutated")
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			again, err := gobackend.Generate(p)
			if err != nil || !bytes.Equal(generated, again) {
				t.Fatal("unstable generation", err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package pipelanggenerated
import "testing"
func TestValues(t *testing.T){for i:=0;i<3;i++{if PipeLangSelect(false)!=%d||PipeLangSelect(true)!=%d{t.Fatal("assignment result")}}}`, c.want[0], c.want[1])))
		})
	}
}

func TestV1150AssignmentRefusals(t *testing.T) {
	cases := []struct{ body, diagnostic string }{
		{`ArithmeticError x; return 1;`, "outside the executable value contract"},
		{`Choices x; return 1;`, "outside the executable value contract"},
		{`int x; return x;`, "before definite assignment"},
		{`mutable int x; x=x; return x;`, "before definite assignment"},
		{`mutable int x=x; return x;`, "before definite assignment"},
		{`int x; if(flag){x=1;} return x;`, "before definite assignment"},
		{`int x; if(flag){x=1;} x=2; return x;`, "already be assigned"},
		{`int x=1; x=2; return x;`, "already be assigned"},
		{`int x; {x=1;} x=2; return x;`, "already be assigned"},
		{`flag=false; return 1;`, "already be assigned"},
		{`mutable int x=1; x=true; return x;`, "type mismatch"},
		{`x=1; return x;`, "in-scope local"},
		{`{mutable int x=1;} x=2; return x;`, "in-scope local"},
		{`mutable int x=1; {int x=2;} return x;`, "shadows"},
		{`mutable bool x; if(x){return 1;} return 2;`, "before definite assignment"},
		{`int x; if(flag){x=1;}else{int y=x; return y;} return x;`, "before definite assignment"},
		{`int x; if(flag){x=1;} if(flag){return x;} return 0;`, "before definite assignment"},
		{`return 1; mutable int x=2;`, "unreachable"},
		{`mutable int x=1; x+=1; return x;`, ""},
		{`mutable int x=1; while(flag){x=2;} return x;`, ""},
	}
	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			a := blockAnalysis(`public Class Choices{public int Select(bool flag){`+c.body+`}}`, PipeLangLanguageContractV1150)
			if a.Error() == nil || !strings.Contains(a.Error().Error(), c.diagnostic) {
				t.Fatalf("wanted %q, got %v", c.diagnostic, a.Error())
			}
			if _, ok := AsDiagnostics(a.Error()); !ok {
				t.Fatal("unstructured diagnostic")
			}
		})
	}
	for _, body := range []string{`mutable int x=1; return x;`, `int x; x=1; return x;`, `int x=1; x=2; return x;`} {
		if blockAnalysis(`public Class Choices{public int Select(){`+body+`}}`, PipeLangLanguageContractV1140).Error() == nil {
			t.Fatal("old source contract admitted assignment")
		}
	}
}

func TestV1150AssignmentCoreRefusals(t *testing.T) {
	_, original := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, `public Class Choices{public int Select(bool flag){mutable int x; if(flag){x=1;}else{x=2;} x=3; return x;}}`, []string{"Select"})
	encoded, _ := json.Marshal(original)
	cases := []struct {
		name string
		edit func(*coreir.Program)
	}{
		{"old_contract", func(p *coreir.Program) { p.LanguageContract = coreir.LanguageContractV1140 }},
		{"missing_branch", func(p *coreir.Program) {
			b := p.Functions[0].Body.Block
			b.Statements[1].Else = nil
			b.Statements = append(b.Statements[:2], b.Statements[3:]...)
		}},
		{"immutable_twice", func(p *coreir.Program) { p.Functions[0].Body.Block.Statements[0].Mutable = false }},
		{"parameter", func(p *coreir.Program) { v := 0; p.Functions[0].Body.Block.Statements[2].Target = &v }},
		{"out_of_scope", func(p *coreir.Program) { v := 2; p.Functions[0].Body.Block.Statements[2].Target = &v }},
		{"negative", func(p *coreir.Program) { v := -1; p.Functions[0].Body.Block.Statements[2].Target = &v }},
		{"missing_target", func(p *coreir.Program) { p.Functions[0].Body.Block.Statements[2].Target = nil }},
		{"missing_value", func(p *coreir.Program) { p.Functions[0].Body.Block.Statements[2].Value = nil }},
		{"extra_mutable", func(p *coreir.Program) { p.Functions[0].Body.Block.Statements[3].Mutable = true }},
		{"extra_target", func(p *coreir.Program) { v := 1; p.Functions[0].Body.Block.Statements[3].Target = &v }},
		{"cycle", func(p *coreir.Program) { b := p.Functions[0].Body.Block; b.Statements[1].Then = b }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p coreir.Program
			if err := json.Unmarshal(encoded, &p); err != nil {
				t.Fatal(err)
			}
			c.edit(&p)
			if coreir.ValidateProgram(p) == nil {
				t.Fatal("malformed Core accepted")
			}
			if _, err := coreeval.PrepareProgram(p); err == nil {
				t.Fatal("evaluator admitted malformed Core")
			}
			if _, err := gobackend.Generate(p); err == nil {
				t.Fatal("backend admitted malformed Core")
			}
		})
	}
}
