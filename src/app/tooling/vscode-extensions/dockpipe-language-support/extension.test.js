const assert = require("assert");
const fs = require("fs");
const Module = require("module");
const path = require("path");

class Position {
  constructor(line, character) {
    this.line = line;
    this.character = character;
  }
}

class Range {
  constructor(start, end) {
    this.start = start;
    this.end = end;
  }
}

class Diagnostic {
  constructor(range, message, severity) {
    this.range = range;
    this.message = message;
    this.severity = severity;
  }
}

class Location {
  constructor(uri, range) {
    this.uri = uri;
    this.range = range;
  }
}

class DiagnosticRelatedInformation {
  constructor(location, message) {
    this.location = location;
    this.message = message;
  }
}

class SemanticTokensLegend {
  constructor(tokenTypes, tokenModifiers) {
    this.tokenTypes = tokenTypes;
    this.tokenModifiers = tokenModifiers;
  }
}

const vscode = {
  DiagnosticSeverity: { Error: 0, Warning: 1 },
  Position,
  Range,
  Diagnostic,
  Location,
  DiagnosticRelatedInformation,
  SemanticTokensLegend,
  Uri: { file: (fileName) => ({ fsPath: fileName }) }
};

const originalLoad = Module._load;
Module._load = function load(request, parent, isMain) {
  if (request === "vscode") return vscode;
  return originalLoad.call(this, request, parent, isMain);
};
const helpers = require("./extension").__test;
Module._load = originalLoad;

const diagnostics = helpers.pipeLangEditorDiagnostics(
  { fileName: "/tmp/demo.pipe" },
  [
    {
      code: "PL2002",
      category: "syntax",
      severity: "error",
      message: "expected ;",
      primary: {
        file: "/tmp/demo.pipe",
        start: { line: 2, column: 3, utf16_column: 4 },
        end: { line: 2, column: 4, utf16_column: 5 }
      },
      related: [
        {
          message: "first declaration",
          range: {
            file: "/tmp/other.pipe",
            start: { line: 1, column: 1, utf16_column: 1 },
            end: { line: 1, column: 2, utf16_column: 2 }
          }
        }
      ]
    },
    {
      code: "PL1001",
      severity: "error",
      message: "sibling diagnostic",
      primary: { file: "/tmp/other.pipe", start: {}, end: {} }
    }
  ]
);

assert.strictEqual(diagnostics.length, 1);
assert.strictEqual(diagnostics[0].code, "PL2002");
assert.strictEqual(diagnostics[0].source, "PipeLang");
assert.strictEqual(diagnostics[0].range.start.line, 1);
assert.strictEqual(diagnostics[0].range.start.character, 3);
assert.strictEqual(diagnostics[0].range.end.character, 4);
assert.strictEqual(diagnostics[0].relatedInformation.length, 1);
assert.strictEqual(diagnostics[0].relatedInformation[0].location.uri.fsPath, "/tmp/other.pipe");

assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("Result"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("ArithmeticError"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("Record"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("new"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("Optional"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("some"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("none"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("has_value"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("value_or"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("empty_list"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("list"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("count"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("append"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("find_by"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("filter_by"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("filter"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("contains_casefolded"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("filter_contains_casefolded"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("filter_joined_contains_casefolded"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("sort_by_ordinal"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("trim"));
const grammar = JSON.parse(fs.readFileSync(path.join(__dirname, "syntaxes", "pipelang.tmLanguage.json"), "utf8"));
const keywordPattern = grammar.repository.keywords.patterns[0].match;
assert(keywordPattern.includes("Record"));
assert(keywordPattern.includes("new"));
const typePattern = grammar.repository.types.patterns[0].match;
assert(typePattern.includes("Result"));
assert(typePattern.includes("ArithmeticError"));
assert(typePattern.includes("Optional"));
const builtinPattern = grammar.repository.keywords.patterns[3].match;
assert(builtinPattern.includes("some"));
assert(builtinPattern.includes("none"));
assert(builtinPattern.includes("has_value"));
assert(builtinPattern.includes("value_or"));
assert(builtinPattern.includes("empty_list"));
assert(builtinPattern.includes("list"));
assert(builtinPattern.includes("count"));
assert(builtinPattern.includes("append"));
assert(builtinPattern.includes("find_by"));
assert(builtinPattern.includes("filter_by"));
assert(builtinPattern.includes("filter"));
assert(builtinPattern.includes("contains_casefolded"));
assert(builtinPattern.includes("filter_contains_casefolded"));
assert(builtinPattern.includes("filter_joined_contains_casefolded"));
assert(builtinPattern.includes("sort_by_ordinal"));
assert(builtinPattern.includes("ok"));
assert(builtinPattern.includes("err"));
assert(builtinPattern.includes("is_ok"));
assert(builtinPattern.includes("success_or"));
assert(builtinPattern.includes("failure_or"));
assert(builtinPattern.includes("trim"));
const pipeLangReadme = fs.readFileSync(path.join(__dirname, "README.md"), "utf8");
assert(pipeLangReadme.includes("v0.7.0"));
assert(pipeLangReadme.includes("identical Result parameter/return"));
assert(pipeLangReadme.includes("v0.8.0"));
assert(pipeLangReadme.includes("ordinal `string` ordering"));
assert(pipeLangReadme.includes("v0.9.0"));
assert(pipeLangReadme.includes("primitive immutable records"));
assert(pipeLangReadme.includes("v0.10.0"));
assert(pipeLangReadme.includes("one-hop read-only `parameter.Field` projection"));
assert(pipeLangReadme.includes("v0.11.0"));
assert(pipeLangReadme.includes("declaration-ordered `new Row { Id = id, ... }`"));
assert(pipeLangReadme.includes("v0.12.0"));
assert(pipeLangReadme.includes("structural `left == right` or `left != right`"));
assert(pipeLangReadme.includes("v0.13.0"));
assert(pipeLangReadme.includes("primitive `Optional<T>`"));
assert(pipeLangReadme.includes("v0.14.0"));
assert(pipeLangReadme.includes("`value_or(Optional<T>, T) -> T`"));
assert(pipeLangReadme.includes("v0.15.0"));
assert(pipeLangReadme.includes("`empty_list<R>()`, `list(value)`, and direct `List<R>` identity transport"));
assert(pipeLangReadme.includes("v0.16.0"));
assert(pipeLangReadme.includes("`count(List<R>) -> int`"));
assert(pipeLangReadme.includes("v0.17.0"));
assert(pipeLangReadme.includes("`append(List<R>, R) -> List<R>`"));
assert(pipeLangReadme.includes("v0.18.0"));
assert(pipeLangReadme.includes("`Optional<R>`"));
assert(pipeLangReadme.includes("v0.19.0"));
assert(pipeLangReadme.includes("`Result<List<R>, string>`"));
assert(pipeLangReadme.includes("v0.20.0"));
assert(pipeLangReadme.includes("`at(List<R>, int) -> Optional<R>`"));
assert(pipeLangReadme.includes("v0.21.0"));
assert(pipeLangReadme.includes("`find_by(List<R>, R.Field, string) -> Optional<R>`"));
assert(pipeLangReadme.includes("v0.22.0"));
assert(pipeLangReadme.includes("`filter_by(List<R>, R.Field, string) -> List<R>`"));
assert(pipeLangReadme.includes("`contains_casefolded(string, string) -> bool`"));
assert(pipeLangReadme.includes("`filter_contains_casefolded(List<R>, R.Field, string) -> List<R>`"));
const operatorPattern = grammar.repository.operators.patterns[0].match;
assert(operatorPattern.includes("\\-"));
assert(operatorPattern.includes("*"));
assert(operatorPattern.includes("/"));
assert(operatorPattern.includes("<="));
assert(operatorPattern.includes(">="));
assert(operatorPattern.includes("."));
assert(operatorPattern.includes("=="));
assert(operatorPattern.includes("!="));
const pipeLangSnippets = JSON.parse(fs.readFileSync(path.join(__dirname, "snippets", "pipelang.json"), "utf8"));
assert.strictEqual(pipeLangSnippets["PipeLang Record Field Projection"].prefix, "pipe-record-field");
assert(pipeLangSnippets["PipeLang Record Field Projection"].description.includes("v0.10.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record Construction"].prefix, "pipe-record-new");
assert(pipeLangSnippets["PipeLang Record Construction"].description.includes("v0.11.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record Equality"].prefix, "pipe-record-equality");
assert(pipeLangSnippets["PipeLang Record Equality"].description.includes("v0.12.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Primitive Optional"].prefix, "pipe-optional");
assert(pipeLangSnippets["PipeLang Primitive Optional"].description.includes("v0.13.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Optional Defaulting"].prefix, "pipe-optional-value-or");
assert(pipeLangSnippets["PipeLang Optional Defaulting"].description.includes("v0.14.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record List"].prefix, "pipe-record-list");
assert(pipeLangSnippets["PipeLang Record List"].description.includes("v0.15.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Count"].prefix, "pipe-record-list-count");
assert(pipeLangSnippets["PipeLang Record List Count"].description.includes("v0.16.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Append"].prefix, "pipe-record-list-append");
assert(pipeLangSnippets["PipeLang Record List Append"].description.includes("v0.17.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record List At"].prefix, "pipe-record-list-at");
assert(pipeLangSnippets["PipeLang Record List At"].description.includes("v0.20.0"));
assert(pipeLangSnippets["PipeLang Record List At"].body.some((line) => line.includes("at(")));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Find By Text"].prefix, "pipe-record-list-find-by-text");
assert(pipeLangSnippets["PipeLang Record List Find By Text"].description.includes("v0.21.0"));
assert(pipeLangSnippets["PipeLang Record List Find By Text"].body.some((line) => line.includes("find_by(")));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Filter By Text"].prefix, "pipe-record-list-filter-by-text");
assert(pipeLangSnippets["PipeLang Record List Filter By Text"].description.includes("v0.22.0"));
assert(pipeLangSnippets["PipeLang Record List Filter By Text"].body.some((line) => line.includes("filter_by(")));
assert.strictEqual(pipeLangSnippets["PipeLang Case-Folded Text Contains"].prefix, "pipe-text-contains-casefolded");
assert(pipeLangSnippets["PipeLang Case-Folded Text Contains"].body.some((line) => line.includes("contains_casefolded(")));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Case-Folded Filter"].prefix, "pipe-record-list-filter-contains-casefolded");
assert(pipeLangSnippets["PipeLang Record List Case-Folded Filter"].description.includes("v0.24.0"));
assert(pipeLangSnippets["PipeLang Record List Case-Folded Filter"].body.some((line) => line.includes("filter_contains_casefolded(")));
assert.strictEqual(pipeLangSnippets["PipeLang Primitive Record Optional"].prefix, "pipe-record-optional");
assert(pipeLangSnippets["PipeLang Primitive Record Optional"].description.includes("v0.18.0"));
assert(pipeLangSnippets["PipeLang Primitive Record Optional"].body.some((line) => line.includes("value_or")));
assert.strictEqual(pipeLangSnippets["PipeLang Snapshot Result"].prefix, "pipe-snapshot-result");
assert(pipeLangSnippets["PipeLang Snapshot Result"].description.includes("v0.19.0"));
assert(pipeLangSnippets["PipeLang Snapshot Result"].body.some((line) => line.includes("success_or")));
assert(pipeLangReadme.includes("v0.25.0"));
assert(pipeLangReadme.includes("Result<string, string>"));
assert.strictEqual(pipeLangSnippets["PipeLang Text Result"].prefix, "pipe-text-result");
assert(pipeLangSnippets["PipeLang Text Result"].description.includes("v0.25.0"));
assert(pipeLangSnippets["PipeLang Text Result"].body.some((line) => line.includes("failure_or")));
assert(pipeLangReadme.includes("v0.26.0"));
assert(pipeLangReadme.includes("`trim(string) -> string`"));
assert(pipeLangReadme.includes("`filter_joined_contains_casefolded(List<R>, R.Name, R.State, R.Image, R.Ports, R.Created, string) -> List<R>`"));
assert(pipeLangReadme.includes("`sort_by_ordinal(List<R>, R.Field) -> List<R>`"));
assert(pipeLangReadme.includes("`v0.29.0`"));
assert(pipeLangReadme.includes("`filter_joined_contains_casefolded(List<R>, R.Field1, R.Field2, ..., string) -> List<R>`"));
assert(pipeLangReadme.includes("`v0.30.0`"));
assert(pipeLangReadme.includes("`sort_by_ordinal(List<R>, R.Field1, R.Field2, ...) -> List<R>`"));
assert(pipeLangReadme.includes("`v0.31.0`"));
assert(pipeLangReadme.includes("`filter(List<R>, PredicateName, P1, ...) -> List<R>`"));
assert.strictEqual(pipeLangSnippets["PipeLang Text Trim"].prefix, "pipe-text-trim");
assert(pipeLangSnippets["PipeLang Text Trim"].description.includes("v0.26.0"));
assert(pipeLangSnippets["PipeLang Text Trim"].body.some((line) => line.includes("trim(")));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Joined Case-Folded Filter"].prefix, "pipe-record-list-filter-joined-contains-casefolded");
assert(pipeLangSnippets["PipeLang Record List Joined Case-Folded Filter"].body.some((line) => line.includes("filter_joined_contains_casefolded(")));
assert(pipeLangSnippets["PipeLang Record List Joined Case-Folded Filter"].description.includes("v0.29.0"));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Ordinal Sort"].prefix, "pipe-record-list-sort-by-ordinal");
assert(pipeLangSnippets["PipeLang Record List Ordinal Sort"].description.includes("v0.28.0"));
assert(pipeLangSnippets["PipeLang Record List Ordinal Sort"].body.some((line) => line.includes("sort_by_ordinal(")));
assert.strictEqual(pipeLangSnippets["PipeLang Record List Multi-Key Ordinal Sort"].prefix, "pipe-record-list-sort-by-ordinals");
assert(pipeLangSnippets["PipeLang Record List Multi-Key Ordinal Sort"].description.includes("v0.30.0"));
assert(pipeLangSnippets["PipeLang Record List Multi-Key Ordinal Sort"].body.some((line) => line.includes("sort_by_ordinal(")));
assert.strictEqual(pipeLangSnippets["PipeLang Named Record Predicate Filter"].prefix, "pipe-record-list-filter-predicate");
assert(pipeLangSnippets["PipeLang Named Record Predicate Filter"].description.includes("v0.31.0"));
assert(pipeLangSnippets["PipeLang Named Record Predicate Filter"].body.some((line) => line.includes("filter(")));
assert(pipeLangReadme.includes("`v0.36.0`"));
assert(pipeLangReadme.includes("same-class pure"));
assert.strictEqual(pipeLangSnippets["PipeLang same-class pure call"].prefix, "pipe-pure-call");
assert(pipeLangSnippets["PipeLang same-class pure call"].description.includes("v0.36.0"));
assert(pipeLangSnippets["PipeLang same-class pure call"].body.some((line) => line.includes("Order") && line.includes("Filter")));
assert(pipeLangReadme.includes("`v0.37.0`"));
assert(pipeLangReadme.includes("match-arm bodies"));
assert.strictEqual(pipeLangSnippets["PipeLang general pure-call composition"].prefix, "pipe-pure-call-compose");
assert(pipeLangSnippets["PipeLang general pure-call composition"].description.includes("v0.37.0"));
assert(pipeLangSnippets["PipeLang general pure-call composition"].body.some((line) => line.includes("match(") && line.includes("Normalize")));
