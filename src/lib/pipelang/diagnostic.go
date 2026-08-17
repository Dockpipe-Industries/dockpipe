package pipelang

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type DiagnosticCode string
type DiagnosticCategory string
type DiagnosticSeverity string

const (
	CategorySource     DiagnosticCategory = "source"
	CategoryLexical    DiagnosticCategory = "lexical"
	CategorySyntax     DiagnosticCategory = "syntax"
	CategorySemantic   DiagnosticCategory = "semantic"
	CategoryEvaluation DiagnosticCategory = "evaluation"

	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"

	CodeInvalidUTF8             DiagnosticCode = "PL0001"
	CodeDuplicateSource         DiagnosticCode = "PL0002"
	CodeUnexpectedChar          DiagnosticCode = "PL1001"
	CodeInvalidNumber           DiagnosticCode = "PL1002"
	CodeInvalidEscape           DiagnosticCode = "PL1003"
	CodeUnterminatedText        DiagnosticCode = "PL1004"
	CodeUnexpectedToken         DiagnosticCode = "PL2001"
	CodeExpectedToken           DiagnosticCode = "PL2002"
	CodeInvalidProgram          DiagnosticCode = "PL3001"
	CodeInvalidDecl             DiagnosticCode = "PL3002"
	CodeDuplicateDecl           DiagnosticCode = "PL3003"
	CodeInvalidMember           DiagnosticCode = "PL3004"
	CodeDuplicateMember         DiagnosticCode = "PL3005"
	CodeInvalidType             DiagnosticCode = "PL3006"
	CodeUnknownInterface        DiagnosticCode = "PL3007"
	CodeConformance             DiagnosticCode = "PL3008"
	CodeExpressionType          DiagnosticCode = "PL3009"
	CodeEntrySelection          DiagnosticCode = "PL3010"
	CodeInvalidModule           DiagnosticCode = "PL3011"
	CodeDuplicateModule         DiagnosticCode = "PL3012"
	CodeInvalidLock             DiagnosticCode = "PL3013"
	CodeUndeclaredImport        DiagnosticCode = "PL3014"
	CodeUnknownImport           DiagnosticCode = "PL3015"
	CodeAmbiguousImport         DiagnosticCode = "PL3016"
	CodeImportCycle             DiagnosticCode = "PL3017"
	CodePrivateImport           DiagnosticCode = "PL3018"
	CodeInvalidSemanticID       DiagnosticCode = "PL3019"
	CodeDuplicateSemanticID     DiagnosticCode = "PL3020"
	CodeDuplicateSemanticTarget DiagnosticCode = "PL3021"
	CodeMissingSemanticID       DiagnosticCode = "PL3022"
	CodeInvalidProjection       DiagnosticCode = "PL3023"
	CodeDeprecatedName          DiagnosticCode = "PL3024"
	CodeSemanticMigrationCycle  DiagnosticCode = "PL3025"
	CodeHIRLowering             DiagnosticCode = "PL3026"
	CodeCoreLowering            DiagnosticCode = "PL3027"
	CodeNumericSemantics        DiagnosticCode = "PL3028"
	CodeInvocation              DiagnosticCode = "PL4001"
	CodeEvaluation              DiagnosticCode = "PL4002"
)

type RelatedSpan struct {
	Span               Span               `json:"span"`
	Message            string             `json:"message"`
	SemanticIDs        []SemanticID       `json:"semantic_ids,omitempty"`
	SemanticIdentities []SemanticIdentity `json:"semantic_identities,omitempty"`
}

type Diagnostic struct {
	Code               DiagnosticCode     `json:"code"`
	Category           DiagnosticCategory `json:"category"`
	Severity           DiagnosticSeverity `json:"severity"`
	Message            string             `json:"message"`
	Primary            Span               `json:"primary"`
	Related            []RelatedSpan      `json:"related,omitempty"`
	SemanticIDs        []SemanticID       `json:"semantic_ids,omitempty"`
	SemanticIdentities []SemanticIdentity `json:"semantic_identities,omitempty"`
}

type Diagnostics []Diagnostic

func (d Diagnostics) HasErrors() bool {
	for _, diagnostic := range d {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (d Diagnostics) Sort() {
	for i := range d {
		sort.SliceStable(d[i].Related, func(a, b int) bool {
			return compareSpan(d[i].Related[a].Span, d[i].Related[b].Span) < 0
		})
	}
	sort.SliceStable(d, func(i, j int) bool {
		if cmp := compareSpan(d[i].Primary, d[j].Primary); cmp != 0 {
			return cmp < 0
		}
		if d[i].Severity != d[j].Severity {
			return d[i].Severity < d[j].Severity
		}
		if d[i].Code != d[j].Code {
			return d[i].Code < d[j].Code
		}
		return d[i].Message < d[j].Message
	})
}

type DiagnosticError struct {
	Sources     *SourceSet
	Diagnostics Diagnostics
}

func (e *DiagnosticError) Error() string {
	if e == nil || len(e.Diagnostics) == 0 {
		return "PipeLang compilation failed"
	}
	return RenderDiagnostic(e.Sources, e.Diagnostics[0])
}

func AsDiagnostics(err error) (Diagnostics, bool) {
	var diagnosticErr *DiagnosticError
	if !errors.As(err, &diagnosticErr) || diagnosticErr == nil {
		return nil, false
	}
	return append(Diagnostics(nil), diagnosticErr.Diagnostics...), true
}

func RenderDiagnostic(sources *SourceSet, diagnostic Diagnostic) string {
	location := string(diagnostic.Primary.File)
	if diagnostic.Primary.IsValid() {
		resolved := sources.Range(diagnostic.Primary)
		location = fmt.Sprintf("%s:%d:%d", resolved.File, resolved.Start.Line, resolved.Start.Column)
	}
	if location == "" {
		location = "<input>"
	}
	return fmt.Sprintf("%s: %s[%s]: %s", location, diagnostic.Severity, diagnostic.Code, diagnostic.Message)
}

type ResolvedRelatedSpan struct {
	Range              SourceRange        `json:"range"`
	Message            string             `json:"message"`
	SemanticIDs        []SemanticID       `json:"semantic_ids,omitempty"`
	SemanticIdentities []SemanticIdentity `json:"semantic_identities,omitempty"`
}

type ResolvedDiagnostic struct {
	Code               DiagnosticCode        `json:"code"`
	Category           DiagnosticCategory    `json:"category"`
	Severity           DiagnosticSeverity    `json:"severity"`
	Message            string                `json:"message"`
	Primary            SourceRange           `json:"primary"`
	Related            []ResolvedRelatedSpan `json:"related,omitempty"`
	SemanticIDs        []SemanticID          `json:"semantic_ids,omitempty"`
	SemanticIdentities []SemanticIdentity    `json:"semantic_identities,omitempty"`
}

func ResolveDiagnostics(sources *SourceSet, diagnostics Diagnostics) []ResolvedDiagnostic {
	ordered := append(Diagnostics(nil), diagnostics...)
	ordered.Sort()
	out := make([]ResolvedDiagnostic, 0, len(ordered))
	for _, diagnostic := range ordered {
		resolved := ResolvedDiagnostic{
			Code: diagnostic.Code, Category: diagnostic.Category, Severity: diagnostic.Severity,
			Message: diagnostic.Message, Primary: sources.Range(diagnostic.Primary), SemanticIDs: append([]SemanticID(nil), diagnostic.SemanticIDs...), SemanticIdentities: append([]SemanticIdentity(nil), diagnostic.SemanticIdentities...),
		}
		for _, related := range diagnostic.Related {
			resolved.Related = append(resolved.Related, ResolvedRelatedSpan{Range: sources.Range(related.Span), Message: related.Message, SemanticIDs: append([]SemanticID(nil), related.SemanticIDs...), SemanticIdentities: append([]SemanticIdentity(nil), related.SemanticIdentities...)})
		}
		out = append(out, resolved)
	}
	return out
}

func DiagnosticsJSON(sources *SourceSet, diagnostics Diagnostics) ([]byte, error) {
	payload := struct {
		Schema      int                  `json:"schema"`
		Diagnostics []ResolvedDiagnostic `json:"diagnostics"`
	}{Schema: 1, Diagnostics: ResolveDiagnostics(sources, diagnostics)}
	return json.MarshalIndent(payload, "", "  ")
}

func diagnosticError(sources *SourceSet, diagnostics Diagnostics) error {
	if len(diagnostics) == 0 {
		return nil
	}
	diagnostics.Sort()
	return &DiagnosticError{Sources: sources, Diagnostics: diagnostics}
}

func oneDiagnostic(sources *SourceSet, code DiagnosticCode, category DiagnosticCategory, span Span, message string, related ...RelatedSpan) error {
	return diagnosticError(sources, Diagnostics{{Code: code, Category: category, Severity: SeverityError, Message: message, Primary: span, Related: related}})
}

func prefixDiagnostic(err error, prefix string) error {
	var diagnosticErr *DiagnosticError
	if errors.As(err, &diagnosticErr) && diagnosticErr != nil {
		diagnostics := append(Diagnostics(nil), diagnosticErr.Diagnostics...)
		for i := range diagnostics {
			diagnostics[i].Message = prefix + diagnostics[i].Message
		}
		return diagnosticError(diagnosticErr.Sources, diagnostics)
	}
	return fmt.Errorf("%s%w", prefix, err)
}

func compareSpan(a, b Span) int {
	if a.File != b.File {
		return strings.Compare(string(a.File), string(b.File))
	}
	if a.Start != b.Start {
		return a.Start - b.Start
	}
	return a.End - b.End
}

func quoted(value string) string {
	return strconv.Quote(value)
}
