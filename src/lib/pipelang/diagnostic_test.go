package pipelang

import (
	"strings"
	"testing"
)

func TestParseFileAssignsDurableSpansToEveryLegacyASTPath(t *testing.T) {
	src := []byte(`[Label = "demo"]
public Interface IConfig {
  public string Name;
  public bool Ready(int threshold);
}
public Class Config : IConfig {
  public string Name = "dock";
  public bool Ready(int threshold) => (threshold > 0) && true;
}`)
	program, err := ParseFile("models/config.pipe", src)
	if err != nil {
		t.Fatal(err)
	}
	spans := []Span{program.Span, program.Interfaces[0].Span, program.Interfaces[0].Annotations[0].Span,
		program.Interfaces[0].Fields[0].Span, program.Interfaces[0].Methods[0].Span,
		program.Interfaces[0].Methods[0].Params[0].Span, program.Classes[0].Span,
		program.Classes[0].Fields[0].Span, program.Classes[0].Fields[0].Default.SourceSpan(),
		program.Classes[0].Methods[0].Span, program.Classes[0].Methods[0].Params[0].Span,
		program.Classes[0].Methods[0].Body.SourceSpan(),
	}
	for i, span := range spans {
		if !span.IsValid() || span.File != "models/config.pipe" || span.End <= span.Start {
			t.Fatalf("span[%d] = %#v", i, span)
		}
	}
	body := program.Classes[0].Methods[0].Body.(*BinaryExpr)
	if !body.Left.SourceSpan().IsValid() || !body.Right.SourceSpan().IsValid() {
		t.Fatalf("child expression spans: left=%#v right=%#v", body.Left.SourceSpan(), body.Right.SourceSpan())
	}
}

func TestLexerAssignsFileAwareSpanToEveryToken(t *testing.T) {
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "tokens.pipe", Data: []byte("Class C { int N = 1; }")}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	tokens, err := lex(sources, sources.Files()[0])
	if err != nil {
		t.Fatal(err)
	}
	for i, token := range tokens {
		if !token.span.IsValid() || token.span.File != "tokens.pipe" {
			t.Fatalf("token[%d]=%#v", i, token)
		}
		if token.kind != tokEOF && token.span.End <= token.span.Start {
			t.Fatalf("empty token span[%d]=%#v", i, token)
		}
	}
}

func TestSourceSetRejectsInvalidUTF8WithResolvedLocation(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"z.pipe": []byte("Class Z {}"),
		"a.pipe": {'C', 'l', 'a', 's', 's', ' ', 0xff},
	})
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	if diagnostic.Code != CodeInvalidUTF8 || diagnostic.Category != CategorySource || diagnostic.Primary.File != "a.pipe" || diagnostic.Primary.Start != 6 {
		t.Fatalf("diagnostic=%#v", diagnostic)
	}
	resolved := ResolveDiagnostics(analysis.Sources, analysis.Diagnostics)[0]
	if resolved.Primary.Start.Line != 1 || resolved.Primary.Start.Column != 7 || resolved.Primary.Start.UTF16Column != 7 {
		t.Fatalf("resolved=%#v", resolved.Primary)
	}
}

func TestAnalyzeFilesOrdersDiagnosticsByFileAndSpan(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"z.pipe": []byte("Class Z { string Name = ; }"),
		"a.pipe": []byte("Class A { string Name = @; }"),
	})
	if len(analysis.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	if analysis.Diagnostics[0].Primary.File != "a.pipe" || analysis.Diagnostics[1].Primary.File != "z.pipe" {
		t.Fatalf("diagnostic order=%#v", analysis.Diagnostics)
	}
	if analysis.Diagnostics[0].Code != CodeUnexpectedChar || analysis.Diagnostics[1].Code != CodeUnexpectedToken {
		t.Fatalf("diagnostic codes=%#v", analysis.Diagnostics)
	}
}

func TestAnalyzeFilesIncludesRelatedSpanForDuplicateDeclaration(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"a.pipe": []byte("Class Same {}"),
		"b.pipe": []byte("Class Same {}"),
	})
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	if diagnostic.Code != CodeDuplicateDecl || diagnostic.Primary.File != "b.pipe" || len(diagnostic.Related) != 1 || diagnostic.Related[0].Span.File != "a.pipe" {
		t.Fatalf("diagnostic=%#v", diagnostic)
	}
}

func TestCompilerErrorUsesStructuredDiagnosticContract(t *testing.T) {
	_, err := CompileFiles(map[string][]byte{"broken.pipe": []byte("Class Broken { int Value = nope; }")}, "Broken")
	if err == nil {
		t.Fatal("expected error")
	}
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeExpressionType {
		t.Fatalf("error=%T %v diagnostics=%#v", err, err, diagnostics)
	}
	if got := err.Error(); !strings.Contains(got, "broken.pipe:1:") || !strings.Contains(got, "PL3009") {
		t.Fatalf("rendered error=%q", got)
	}
}

func TestResolvedPositionUsesUTF16Columns(t *testing.T) {
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "unicode.pipe", Data: []byte("\"😀\" @")}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	position := sources.Range(Span{File: "unicode.pipe", Start: len("\"😀\" "), End: len("\"😀\" ") + 1}).Start
	if position.Column != 5 || position.UTF16Column != 6 {
		t.Fatalf("position=%#v", position)
	}
}
