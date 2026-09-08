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
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("return"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("if"));
assert(helpers.PIPELANG_COMPLETION_KEYWORDS.includes("else"));
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
assert(keywordPattern.includes("return"));
assert(keywordPattern.includes("if"));
assert(keywordPattern.includes("else"));
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
assert(pipeLangReadme.includes("`v0.38.0`"));
assert(pipeLangReadme.includes("only the selected branch executes"));
assert.strictEqual(pipeLangSnippets["PipeLang bounded conditional expression"].prefix, "pipe-conditional");
assert(pipeLangSnippets["PipeLang bounded conditional expression"].description.includes("v0.38.0"));
assert(pipeLangSnippets["PipeLang bounded conditional expression"].body.some((line) => line.includes("?") && line.includes(":")));
assert(pipeLangReadme.includes("`v0.39.0`"));
assert(pipeLangReadme.includes("Initialization is eager"));
assert.strictEqual(pipeLangSnippets["PipeLang immutable local method"].prefix, "pipe-immutable-local");
assert(pipeLangSnippets["PipeLang immutable local method"].description.includes("v0.39.0"));
assert(pipeLangSnippets["PipeLang immutable local method"].body.some((line) => line.includes("return")));
assert(pipeLangReadme.includes("`v0.40.0`"));
assert(pipeLangReadme.includes("once in order"));
assert.strictEqual(pipeLangSnippets["PipeLang ordered immutable locals method"].prefix, "pipe-immutable-locals");
assert(pipeLangSnippets["PipeLang ordered immutable locals method"].description.includes("v0.40.0"));
assert.strictEqual(pipeLangSnippets["PipeLang ordered immutable locals method"].body.filter((line) => line.includes(" = ")).length, 2);
assert(pipeLangReadme.includes("`v0.41.0`"));
assert(pipeLangReadme.includes("sole direct Optional or bounded Result carrier"));
assert.strictEqual(pipeLangSnippets["PipeLang block-scoped propagation method"].prefix, "pipe-block-propagate");
assert(pipeLangSnippets["PipeLang block-scoped propagation method"].description.includes("v0.41.0"));
assert(pipeLangSnippets["PipeLang block-scoped propagation method"].body.some((line) => line.includes("= propagate(")));
assert(pipeLangReadme.includes("`v0.42.0`"));
assert(pipeLangReadme.includes("immediately preceding bounded carrier"));
assert.strictEqual(pipeLangSnippets["PipeLang prior-local helper propagation method"].prefix, "pipe-prior-local-propagate");
assert(pipeLangSnippets["PipeLang prior-local helper propagation method"].description.includes("v0.42.0"));
assert(pipeLangSnippets["PipeLang prior-local helper propagation method"].body.some((line) => line.includes("= ${4:Helper}(${2:input})")));
assert(pipeLangSnippets["PipeLang prior-local helper propagation method"].body.some((line) => line.includes("= propagate(${3:carrier})")));
assert(pipeLangReadme.includes("`v0.43.0`"));
assert(pipeLangReadme.includes("source-ordered `ok(binding)` then `err(binding)`"));
assert.strictEqual(pipeLangSnippets["PipeLang helper-result match method"].prefix, "pipe-helper-result-match");
assert(pipeLangSnippets["PipeLang helper-result match method"].description.includes("v0.43.0"));
assert(pipeLangSnippets["PipeLang helper-result match method"].body.some((line) => line.includes("match(${1:Validate}(${2:input}))")));
assert(pipeLangSnippets["PipeLang helper-result match method"].body.some((line) => line.includes("ok(${5:value})")));
assert(pipeLangSnippets["PipeLang helper-result match method"].body.some((line) => line.includes("err(${6:error})")));
assert(pipeLangReadme.includes("`v0.44.0`"));
assert(pipeLangReadme.includes("every parameter directly once in declaration order"));
assert.strictEqual(pipeLangSnippets["PipeLang general helper-carrier match method"].prefix, "pipe-helper-carrier-match");
assert(pipeLangSnippets["PipeLang general helper-carrier match method"].description.includes("v0.44.0"));
assert(pipeLangSnippets["PipeLang general helper-carrier match method"].body.some((line) => line.includes("match(${2:FindSelection}(${3:rows}, ${4:id}))")));
assert(pipeLangSnippets["PipeLang general helper-carrier match method"].body.some((line) => line.includes("some(${7:row})")));
assert(pipeLangSnippets["PipeLang general helper-carrier match method"].body.some((line) => line.includes("none =>")));
assert(pipeLangReadme.includes("`v0.45.0`"));
assert(pipeLangReadme.includes("first explicitly typed immutable local"));
assert.strictEqual(pipeLangSnippets["PipeLang helper-carrier match local method"].prefix, "pipe-helper-carrier-match-local");
assert(pipeLangSnippets["PipeLang helper-carrier match local method"].description.includes("v0.45.0"));
assert(pipeLangSnippets["PipeLang helper-carrier match local method"].body.some((line) => line.includes("string ${7:selected} = match(${2:FindSelection}(${3:rows}, ${4:id}))")));
assert(pipeLangSnippets["PipeLang helper-carrier match local method"].body.some((line) => line.includes("return ${10:NormalizeName}(${7:selected})")));
assert(pipeLangReadme.includes("`v0.46.0`"));
assert(pipeLangReadme.includes("Result<int, ArithmeticError>"));
assert.strictEqual(pipeLangSnippets["PipeLang checked-arithmetic helper match local method"].prefix, "pipe-checked-arithmetic-helper-match-local");
assert(pipeLangSnippets["PipeLang checked-arithmetic helper match local method"].description.includes("v0.46.0"));
assert(pipeLangSnippets["PipeLang checked-arithmetic helper match local method"].body.some((line) => line.includes("int ${5:selected} = match(${1:Add}(${2:left}, ${3:right}))")));
assert(pipeLangSnippets["PipeLang checked-arithmetic helper match local method"].body.some((line) => line.includes("err(${7:problem}) => 0")));
assert(pipeLangReadme.includes("`v0.47.0`"));
assert(pipeLangReadme.includes("any explicitly typed immutable local"));
assert.strictEqual(pipeLangSnippets["PipeLang later-local helper match method"].prefix, "pipe-later-local-helper-match");
assert(pipeLangSnippets["PipeLang later-local helper match method"].description.includes("v0.47.0"));
assert(pipeLangSnippets["PipeLang later-local helper match method"].body.some((line) => line.includes("string ${7:fallback} = \"\"")));
assert(pipeLangSnippets["PipeLang later-local helper match method"].body.some((line) => line.includes("string ${8:selected} = match(${2:FindSelection}(${3:rows}, ${4:id}))")));
assert(pipeLangSnippets["PipeLang later-local helper match method"].body.some((line) => line.includes("none => ${7:fallback}")));
assert(pipeLangReadme.includes("`v0.48.0`"));
assert(pipeLangReadme.includes("immediately following typed local"));
assert.strictEqual(pipeLangSnippets["PipeLang prior-local carrier match method"].prefix, "pipe-prior-local-carrier-match");
assert(pipeLangSnippets["PipeLang prior-local carrier match method"].description.includes("v0.48.0"));
assert(pipeLangSnippets["PipeLang prior-local carrier match method"].body.some((line) => line.includes("Optional<${1:Row}> ${8:selection} = ${2:FindSelection}(${3:rows}, ${4:id})")));
assert(pipeLangSnippets["PipeLang prior-local carrier match method"].body.some((line) => line.includes("string ${9:selected} = match(${8:selection})")));
assert(pipeLangSnippets["PipeLang prior-local carrier match method"].body.some((line) => line.includes("return ${12:NormalizeName}(${9:selected})")));
assert(pipeLangReadme.includes("`v0.49.0`"));
assert(pipeLangReadme.includes("exactly two non-overlapping"));
assert.strictEqual(pipeLangSnippets["PipeLang bounded two-carrier match method"].prefix, "pipe-two-carrier-matches");
assert(pipeLangSnippets["PipeLang bounded two-carrier match method"].description.includes("v0.49.0"));
assert(pipeLangSnippets["PipeLang bounded two-carrier match method"].body.some((line) => line.includes("Optional<${1:Row}> ${8:firstCarrier} = ${2:FindPrimary}(${3:rows}, ${4:id})")));
assert(pipeLangSnippets["PipeLang bounded two-carrier match method"].body.some((line) => line.includes("string ${9:first} = match(${8:firstCarrier})")));
assert(pipeLangSnippets["PipeLang bounded two-carrier match method"].body.some((line) => line.includes("Optional<${1:Row}> ${12:secondCarrier} = ${6:FindSecondary}(${3:rows}, ${4:id})")));
assert(pipeLangSnippets["PipeLang bounded two-carrier match method"].body.some((line) => line.includes("string ${13:selected} = match(${12:secondCarrier})")));
assert(pipeLangReadme.includes("`v0.50.0`"));
assert(pipeLangReadme.includes("exactly four contiguous locals"));
assert.strictEqual(pipeLangSnippets["PipeLang dependent second-carrier match method"].prefix, "pipe-dependent-second-carrier-match");
assert(pipeLangSnippets["PipeLang dependent second-carrier match method"].description.includes("v0.50.0"));
assert(pipeLangSnippets["PipeLang dependent second-carrier match method"].body.some((line) => line.includes("${6:ConfirmSecondary}(string ${7:first}, List<${1:Row}> ${3:rows}, string ${4:id})")));
assert(pipeLangSnippets["PipeLang dependent second-carrier match method"].body.some((line) => line.includes("${12:secondCarrier} = ${6:ConfirmSecondary}(${7:first}, ${3:rows}, ${4:id})")));
assert(pipeLangReadme.includes("`v0.51.0`"));
assert(pipeLangReadme.includes("contiguous chain of two or more"));
assert.strictEqual(pipeLangSnippets["PipeLang dependent carrier chain method"].prefix, "pipe-dependent-carrier-chain");
assert(pipeLangSnippets["PipeLang dependent carrier chain method"].description.includes("v0.51.0"));
assert(pipeLangSnippets["PipeLang dependent carrier chain method"].body.some((line) => line.includes("${8:FinalizeTertiary}(string ${9:second}, List<${1:Row}> ${3:rows}, string ${4:id})")));
assert(pipeLangSnippets["PipeLang dependent carrier chain method"].body.some((line) => line.includes("${16:thirdCarrier} = ${8:FinalizeTertiary}(${9:second}, ${3:rows}, ${4:id})")));
assert(pipeLangReadme.includes("`v0.52.0`"));
assert(pipeLangReadme.includes("every prior selected local"));
assert.strictEqual(pipeLangSnippets["PipeLang cumulative fan-in carrier chain method"].prefix, "pipe-cumulative-fan-in-chain");
assert(pipeLangSnippets["PipeLang cumulative fan-in carrier chain method"].description.includes("v0.52.0"));
assert(pipeLangSnippets["PipeLang cumulative fan-in carrier chain method"].body.some((line) => line.includes("${8:FinalizeHistory}(string ${7:first}, string ${9:second}, List<${1:Row}> ${3:rows}, string ${4:id})")));
assert(pipeLangSnippets["PipeLang cumulative fan-in carrier chain method"].body.some((line) => line.includes("${16:thirdCarrier} = ${8:FinalizeHistory}(${7:first}, ${9:second}, ${3:rows}, ${4:id})")));
assert(pipeLangReadme.includes("`v0.53.0`"));
assert(pipeLangReadme.includes("every caller parameter directly once in declaration order"));
assert.strictEqual(pipeLangSnippets["PipeLang multi-parameter helper propagation method"].prefix, "pipe-multi-parameter-helper-propagation");
assert(pipeLangSnippets["PipeLang multi-parameter helper propagation method"].description.includes("v0.53.0"));
assert(pipeLangSnippets["PipeLang multi-parameter helper propagation method"].body.some((line) => line.includes("${2:Find}(${3:rows}, ${4:id})")));
assert(pipeLangSnippets["PipeLang multi-parameter helper propagation method"].body.some((line) => line.includes("${8:selected} = propagate(${7:carrier})")));
assert(pipeLangReadme.includes("`v0.54.0`"));
assert(pipeLangReadme.includes("checked-arithmetic helper propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang checked-arithmetic helper propagation method"].prefix, "pipe-checked-arithmetic-helper-propagation");
assert(pipeLangSnippets["PipeLang checked-arithmetic helper propagation method"].description.includes("v0.54.0"));
assert(pipeLangSnippets["PipeLang checked-arithmetic helper propagation method"].body.some((line) => line.includes("Result<int, ArithmeticError> ${5:carrier} = ${1:Add}(${2:left}, ${3:right})")));
assert(pipeLangSnippets["PipeLang checked-arithmetic helper propagation method"].body.some((line) => line.includes("int ${6:value} = propagate(${5:carrier})")));
assert(pipeLangReadme.includes("`v0.55.0`"));
assert(pipeLangReadme.includes("direct-parameter checked propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang direct checked-arithmetic propagation method"].prefix, "pipe-direct-checked-arithmetic-propagation");
assert(pipeLangSnippets["PipeLang direct checked-arithmetic propagation method"].description.includes("v0.55.0"));
assert(pipeLangSnippets["PipeLang direct checked-arithmetic propagation method"].body.some((line) => line.includes("Result<int, ArithmeticError> ${2:carrier}")));
assert(pipeLangSnippets["PipeLang direct checked-arithmetic propagation method"].body.some((line) => line.includes("int ${3:value} = propagate(${2:carrier})")));
assert(pipeLangReadme.includes("`v0.56.0`"));
assert(pipeLangReadme.includes("multi-parameter direct checked propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang multi-parameter direct checked-arithmetic propagation method"].prefix, "pipe-multi-parameter-direct-checked-propagation");
assert(pipeLangSnippets["PipeLang multi-parameter direct checked-arithmetic propagation method"].description.includes("v0.56.0"));
assert(pipeLangSnippets["PipeLang multi-parameter direct checked-arithmetic propagation method"].body.some((line) => line.includes("Result<int, ArithmeticError> ${2:carrier}, int ${3:operand}")));
assert(pipeLangSnippets["PipeLang multi-parameter direct checked-arithmetic propagation method"].body.some((line) => line.includes("return ${4:value} + ${3:operand}")));
assert(pipeLangReadme.includes("`v0.57.0`"));
assert(pipeLangReadme.includes("two-stage checked propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang two-stage checked-arithmetic propagation method"].prefix, "pipe-two-stage-checked-propagation");
assert(pipeLangSnippets["PipeLang two-stage checked-arithmetic propagation method"].description.includes("v0.57.0"));
assert(pipeLangSnippets["PipeLang two-stage checked-arithmetic propagation method"].body.some((line) => line.includes("Result<int, ArithmeticError> ${6:nextCarrier} = ${5:value} + ${3:first}")));
assert(pipeLangSnippets["PipeLang two-stage checked-arithmetic propagation method"].body.some((line) => line.includes("int ${7:next} = propagate(${6:nextCarrier})")));
assert(pipeLangReadme.includes("`v0.58.0`"));
assert(pipeLangReadme.includes("generalized checked-propagation chains"));
assert.strictEqual(pipeLangSnippets["PipeLang generalized checked-arithmetic propagation chain"].prefix, "pipe-checked-propagation-chain");
assert(pipeLangSnippets["PipeLang generalized checked-arithmetic propagation chain"].description.includes("v0.58.0"));
assert(pipeLangSnippets["PipeLang generalized checked-arithmetic propagation chain"].body.some((line) => line.includes("Result<int, ArithmeticError> ${9:thirdCarrier} = ${8:secondValue} - ${4:second}")));
assert(pipeLangSnippets["PipeLang generalized checked-arithmetic propagation chain"].body.some((line) => line.includes("return ${10:thirdValue} * ${5:third}")));
assert(pipeLangReadme.includes("`v0.59.0`"));
assert(pipeLangReadme.includes("bounded cross-payload Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang bounded cross-payload Result propagation method"].prefix, "pipe-cross-payload-result");
assert(pipeLangSnippets["PipeLang bounded cross-payload Result propagation method"].description.includes("v0.59.0"));
assert(pipeLangSnippets["PipeLang bounded cross-payload Result propagation method"].body.some((line) => line.includes("string ${4:source} = propagate(${3:scanned})")));
assert(pipeLangSnippets["PipeLang bounded cross-payload Result propagation method"].body.some((line) => line.includes("return ${5:ParseTokens}(${4:source})")));
assert(pipeLangReadme.includes("`v0.60.0`"));
assert(pipeLangReadme.includes("two-stage bounded cross-payload Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang two-stage cross-payload Result propagation method"].prefix, "pipe-two-stage-cross-payload-result");
assert(pipeLangSnippets["PipeLang two-stage cross-payload Result propagation method"].description.includes("v0.60.0"));
assert(pipeLangSnippets["PipeLang two-stage cross-payload Result propagation method"].body.some((line) => line.includes("Result<List<${5:Token}>, string> ${6:tokenized} = ${7:BuildTokens}(${4:source})")));
assert(pipeLangSnippets["PipeLang two-stage cross-payload Result propagation method"].body.some((line) => line.includes("List<${5:Token}> ${8:tokens} = propagate(${6:tokenized})")));
assert(pipeLangSnippets["PipeLang two-stage cross-payload Result propagation method"].body.some((line) => line.includes("return ${9:BuildSyntax}(${8:tokens})")));
assert(pipeLangReadme.includes("`v0.61.0`"));
assert(pipeLangReadme.includes("generalizes bounded cross-payload Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang generalized cross-payload Result propagation chain"].prefix, "pipe-cross-payload-result-chain");
assert(pipeLangSnippets["PipeLang generalized cross-payload Result propagation chain"].description.includes("v0.61.0"));
assert(pipeLangSnippets["PipeLang generalized cross-payload Result propagation chain"].body.some((line) => line.includes("Result<List<${8:SyntaxNode}>, string> ${9:parsed} = ${10:BuildSyntax}(${7:tokens})")));
assert(pipeLangSnippets["PipeLang generalized cross-payload Result propagation chain"].body.some((line) => line.includes("List<${8:SyntaxNode}> ${11:syntax} = propagate(${9:parsed})")));
assert(pipeLangSnippets["PipeLang generalized cross-payload Result propagation chain"].body.some((line) => line.includes("return ${12:EmitSource}(${11:syntax})")));
assert(pipeLangReadme.includes("`v0.62.0`"));
assert(pipeLangReadme.includes("contextual bounded cross-payload Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang contextual cross-payload Result propagation chain"].prefix, "pipe-contextual-cross-payload-result-chain");
assert(pipeLangSnippets["PipeLang contextual cross-payload Result propagation chain"].description.includes("v0.62.0"));
assert(pipeLangSnippets["PipeLang contextual cross-payload Result propagation chain"].body.some((line) => line.includes("Result<string, string> ${2:scanned}, string ${3:context}")));
assert(pipeLangSnippets["PipeLang contextual cross-payload Result propagation chain"].body.some((line) => line.includes("${7:BuildTokens}(${4:source}, ${3:context})")));
assert(pipeLangSnippets["PipeLang contextual cross-payload Result propagation chain"].body.some((line) => line.includes("return ${13:EmitSource}(${12:syntax}, ${3:context})")));
assert(pipeLangReadme.includes("`v0.63.0`"));
assert(pipeLangReadme.includes("one-stage contextual bounded cross-payload Result"));
assert.strictEqual(pipeLangSnippets["PipeLang one-stage contextual cross-payload Result propagation"].prefix, "pipe-contextual-cross-payload-result");
assert(pipeLangSnippets["PipeLang one-stage contextual cross-payload Result propagation"].description.includes("v0.63.0"));
assert(pipeLangSnippets["PipeLang one-stage contextual cross-payload Result propagation"].body.some((line) => line.includes("Result<string, string> ${3:scanned}, string ${4:context}")));
assert(pipeLangSnippets["PipeLang one-stage contextual cross-payload Result propagation"].body.some((line) => line.includes("return ${6:BuildTokens}(${5:source}, ${4:context})")));
assert(pipeLangReadme.includes("`v0.64.0`"));
assert(pipeLangReadme.includes("one-stage contextual bounded same-payload Result"));
assert.strictEqual(pipeLangSnippets["PipeLang one-stage contextual same-payload Result propagation"].prefix, "pipe-contextual-same-payload-result");
assert(pipeLangSnippets["PipeLang one-stage contextual same-payload Result propagation"].description.includes("v0.64.0"));
assert(pipeLangSnippets["PipeLang one-stage contextual same-payload Result propagation"].body.some((line) => line.includes("Result<string, string> ${1:Normalize}(Result<string, string> ${2:input}, string ${3:context})")));
assert(pipeLangSnippets["PipeLang one-stage contextual same-payload Result propagation"].body.some((line) => line.includes("return ${5:NormalizeValue}(${4:value}, ${3:context})")));
assert(pipeLangReadme.includes("`v0.65.0`"));
assert(pipeLangReadme.includes("exact two-stage contextual bounded Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang exact two-stage contextual bounded Result propagation"].prefix, "pipe-two-stage-contextual-bounded-result");
assert(pipeLangSnippets["PipeLang exact two-stage contextual bounded Result propagation"].description.includes("v0.65.0"));
assert(pipeLangSnippets["PipeLang exact two-stage contextual bounded Result propagation"].body.some((line) => line.includes("Result<${1:string}, string> ${2:Normalize}(Result<${3:string}, string> ${4:input}, string ${5:context})")));
assert(pipeLangSnippets["PipeLang exact two-stage contextual bounded Result propagation"].body.some((line) => line.includes("Result<${7:string}, string> ${8:nextCarrier} = ${9:First}(${6:first}, ${5:context})")));
assert(pipeLangSnippets["PipeLang exact two-stage contextual bounded Result propagation"].body.some((line) => line.includes("return ${11:Second}(${10:second}, ${5:context})")));
assert(pipeLangReadme.includes("`v0.66.0`"));
assert(pipeLangReadme.includes("generalizes contextual bounded Result propagation"));
assert.strictEqual(pipeLangSnippets["PipeLang generalized contextual bounded Result propagation"].prefix, "pipe-generalized-contextual-bounded-result");
assert(pipeLangSnippets["PipeLang generalized contextual bounded Result propagation"].description.includes("v0.66.0"));
assert(pipeLangSnippets["PipeLang generalized contextual bounded Result propagation"].body.some((line) => line.includes("Result<${7:string}, string> ${8:secondCarrier} = ${9:First}(${6:first}, ${5:context})")));
assert(pipeLangSnippets["PipeLang generalized contextual bounded Result propagation"].body.some((line) => line.includes("Result<${11:string}, string> ${12:thirdCarrier} = ${13:Second}(${10:second}, ${5:context})")));
assert(pipeLangSnippets["PipeLang generalized contextual bounded Result propagation"].body.some((line) => line.includes("return ${15:Third}(${14:third}, ${5:context})")));
assert(pipeLangReadme.includes("`v0.67.0`"));
assert(pipeLangReadme.includes("generalizes shared context arity"));
assert.strictEqual(pipeLangSnippets["PipeLang generalized shared-context Result propagation"].prefix, "pipe-generalized-shared-context-result");
assert(pipeLangSnippets["PipeLang generalized shared-context Result propagation"].description.includes("v0.67.0"));
assert(pipeLangSnippets["PipeLang generalized shared-context Result propagation"].body.some((line) => line.includes("string ${5:phase}, string ${6:scope}")));
assert(pipeLangSnippets["PipeLang generalized shared-context Result propagation"].body.some((line) => line.includes("${10:First}(${7:first}, ${5:phase}, ${6:scope})")));
assert(pipeLangSnippets["PipeLang generalized shared-context Result propagation"].body.some((line) => line.includes("return ${12:Second}(${11:second}, ${5:phase}, ${6:scope})")));
assert(pipeLangReadme.includes("`v0.68.0`"));
assert(pipeLangReadme.includes("generalizes one-stage shared context arity"));
assert.strictEqual(pipeLangSnippets["PipeLang generalized one-stage shared-context Result propagation"].prefix, "pipe-generalized-one-stage-shared-context-result");
assert(pipeLangSnippets["PipeLang generalized one-stage shared-context Result propagation"].description.includes("v0.68.0"));
assert(pipeLangSnippets["PipeLang generalized one-stage shared-context Result propagation"].body.some((line) => line.includes("string ${5:phase}, string ${6:scope}")));
assert(pipeLangSnippets["PipeLang generalized one-stage shared-context Result propagation"].body.some((line) => line.includes("return ${8:Next}(${7:value}, ${5:phase}, ${6:scope})")));
assert(pipeLangReadme.includes("`v0.69.0`"));
assert(pipeLangReadme.includes("terminal statement-level `if/else`"));
assert.strictEqual(pipeLangSnippets["PipeLang terminal if/else"].prefix, "pipe-terminal-if");
assert(pipeLangSnippets["PipeLang terminal if/else"].description.includes("v0.69.0"));
assert(pipeLangSnippets["PipeLang terminal if/else"].body.some((line) => line.includes("if (${4:normalize}) { return ${5:cleaned}; }")));
assert(pipeLangSnippets["PipeLang terminal if/else"].body.some((line) => line.includes("else { return ${3:raw}; }")));
assert(pipeLangReadme.includes("`v0.70.0`"));
assert(pipeLangReadme.includes("one lexical immutable local per terminal branch"));
assert.strictEqual(pipeLangSnippets["PipeLang terminal if/else with branch local"].prefix, "pipe-terminal-if-branch-local");
assert(pipeLangSnippets["PipeLang terminal if/else with branch local"].description.includes("v0.70.0"));
assert(pipeLangSnippets["PipeLang terminal if/else with branch local"].body.some((line) => line.includes("${1:string} ${6:selected} = trim(${5:cleaned});")));
assert(pipeLangSnippets["PipeLang terminal if/else with branch local"].body.some((line) => line.includes("return ${6:selected};")));
assert(pipeLangReadme.includes("`v0.71.0`"));
assert(pipeLangReadme.includes("a second ordered lexical immutable local per terminal branch"));
assert.strictEqual(pipeLangSnippets["PipeLang terminal if/else with two branch locals"].prefix, "pipe-terminal-if-branch-locals");
assert(pipeLangSnippets["PipeLang terminal if/else with two branch locals"].description.includes("v0.71.0"));
assert(pipeLangSnippets["PipeLang terminal if/else with two branch locals"].body.some((line) => line.includes("${1:string} ${6:prepared} = trim(${5:cleaned});")));
assert(pipeLangSnippets["PipeLang terminal if/else with two branch locals"].body.some((line) => line.includes("${1:string} ${7:selected} = ${6:prepared};")));
assert(pipeLangReadme.includes("`v0.72.0`"));
assert(pipeLangReadme.includes("any finite source-ordered"));
assert.strictEqual(pipeLangSnippets["PipeLang terminal if/else with a general branch-local sequence"].prefix, "pipe-terminal-if-branch-local-sequence");
assert(pipeLangSnippets["PipeLang terminal if/else with a general branch-local sequence"].description.includes("v0.72.0"));
assert(pipeLangSnippets["PipeLang terminal if/else with a general branch-local sequence"].body.some((line) => line.includes("${1:string} ${8:selected} = ${7:second};")));
assert(pipeLangReadme.includes("`v0.73.0`"));
assert(pipeLangReadme.includes("top-level-local prerequisite"));
assert.strictEqual(pipeLangSnippets["PipeLang direct terminal if/else"].prefix, "pipe-direct-terminal-if");
assert(pipeLangSnippets["PipeLang direct terminal if/else"].description.includes("v0.73.0"));
assert(pipeLangSnippets["PipeLang direct terminal if/else"].body.some((line) => line.includes("if (${4:normalize}) {")));
assert(pipeLangSnippets["PipeLang direct terminal if/else"].body.some((line) => line.includes("else { return ${3:raw}; }")));
assert(pipeLangReadme.includes("`v0.74.0`"));
assert(pipeLangReadme.includes("one bounded nested terminal decision"));
assert.strictEqual(pipeLangSnippets["PipeLang nested terminal if/else"].prefix, "pipe-nested-terminal-if");
assert(pipeLangSnippets["PipeLang nested terminal if/else"].description.includes("v0.74.0"));
assert(pipeLangSnippets["PipeLang nested terminal if/else"].body.some((line) => line.includes("if (${5:normalize}) {")));
assert(pipeLangSnippets["PipeLang nested terminal if/else"].body.some((line) => line.includes("return ${6:cleaned};")));
assert(pipeLangReadme.includes("`v0.75.0`"));
assert(pipeLangReadme.includes("inner terminal leaf"));
assert.strictEqual(pipeLangSnippets["PipeLang nested terminal if/else with inner locals"].prefix, "pipe-nested-terminal-if-inner-locals");
assert(pipeLangSnippets["PipeLang nested terminal if/else with inner locals"].description.includes("v0.75.0"));
assert(pipeLangSnippets["PipeLang nested terminal if/else with inner locals"].body.some((line) => line.includes("${1:string} ${7:normalized}")));
assert(pipeLangSnippets["PipeLang nested terminal if/else with inner locals"].body.some((line) => line.includes("return ${9:selectedFallback};")));
assert(pipeLangReadme.includes("`v0.76.0`"));
assert(pipeLangReadme.includes("before the outer condition"));
assert.strictEqual(pipeLangSnippets["PipeLang root locals before nested terminal if/else"].prefix, "pipe-root-local-nested-terminal-if");
assert(pipeLangSnippets["PipeLang root locals before nested terminal if/else"].description.includes("v0.76.0"));
assert(pipeLangSnippets["PipeLang root locals before nested terminal if/else"].body.some((line) => line.includes("${1:string} ${6:shared}")));
assert(pipeLangSnippets["PipeLang root locals before nested terminal if/else"].body.some((line) => line.includes("if (${4:enabled}) {")));
assert(pipeLangReadme.includes("`v0.77.0`"));
assert(pipeLangReadme.includes("two branches each end"));
assert.strictEqual(pipeLangSnippets["PipeLang symmetric nested terminal if/else"].prefix, "pipe-symmetric-nested-terminal-if");
assert(pipeLangSnippets["PipeLang symmetric nested terminal if/else"].description.includes("v0.77.0"));
assert(pipeLangSnippets["PipeLang symmetric nested terminal if/else"].body.some((line) => line.includes("if (${5:left})")));
assert(pipeLangSnippets["PipeLang symmetric nested terminal if/else"].body.some((line) => line.includes("if (${6:right})")));

assert(pipeLangReadme.includes("`v0.78.0`"));
const rootlessSymmetric = pipeLangSnippets["PipeLang rootless symmetric nested terminal if/else"];
assert.strictEqual(rootlessSymmetric.prefix, "pipe-rootless-symmetric-nested-terminal-if");
assert(rootlessSymmetric.description.includes("v0.78.0"));
assert(rootlessSymmetric.body[1].includes("if (${4:outer})"));
assert(rootlessSymmetric.body.some((line) => line.includes("if (${5:left})")));
assert(rootlessSymmetric.body.some((line) => line.includes("if (${6:right})")));

assert(pipeLangReadme.includes("`v0.79.0`"));
assert(pipeLangReadme.includes("exactly one of its four terminal leaves"));
const boundedDepthThree = pipeLangSnippets["PipeLang bounded depth-three terminal if/else"];
assert.strictEqual(boundedDepthThree.prefix, "pipe-bounded-depth-three-terminal-if");
assert(boundedDepthThree.description.includes("v0.79.0"));
assert(boundedDepthThree.body.some((line) => line.includes("if (${4:outer})")));
assert(boundedDepthThree.body.some((line) => line.includes("if (${5:left})")));
assert(boundedDepthThree.body.some((line) => line.includes("if (${6:deep})")));
assert(boundedDepthThree.body.some((line) => line.includes("if (${7:right})")));

assert(pipeLangReadme.includes("`v0.80.0`"));
const twoExpanded = pipeLangSnippets["PipeLang two-expanded terminal if/else"];
assert.strictEqual(twoExpanded.prefix, "pipe-two-expanded-terminal-if");
assert(twoExpanded.description.includes("v0.80.0"));
assert.strictEqual(twoExpanded.body.filter((line) => line.includes("if (")).length, 5);
assert.strictEqual(twoExpanded.body.filter((line) => line.includes("return ")).length, 6);

assert(pipeLangReadme.includes("`v0.81.0`"));
const terminalTree = pipeLangSnippets["PipeLang terminal tree through depth three"];
assert.strictEqual(terminalTree.prefix, "pipe-terminal-tree");
assert(terminalTree.description.includes("v0.81.0"));
assert.strictEqual(terminalTree.body.filter((line) => line.includes("if (")).length, 3);
assert.strictEqual(terminalTree.body.filter((line) => line.includes("return ")).length, 4);

assert(pipeLangReadme.includes("`v0.82.0`"));
const conditionalLocalTree = pipeLangSnippets["PipeLang conditional local in terminal tree"];
assert.strictEqual(conditionalLocalTree.prefix, "pipe-conditional-local-tree");
assert(conditionalLocalTree.description.includes("v0.82.0"));
assert.strictEqual(conditionalLocalTree.body.filter((line) => line.includes(" ? ")).length, 1);
assert(conditionalLocalTree.body.some((line) => line.includes("string selected = normalize ? trim(raw) : raw;")));

assert(pipeLangReadme.includes("`v0.83.0`"));
const twoConditionalLocals = pipeLangSnippets["PipeLang two conditional locals in terminal tree"];
assert.strictEqual(twoConditionalLocals.prefix, "pipe-two-conditional-locals");
assert(twoConditionalLocals.description.includes("v0.83.0"));
assert.strictEqual(twoConditionalLocals.body.filter((line) => line.includes(" ? ")).length, 2);
assert(twoConditionalLocals.body.some((line) => line.includes('suffix && normalized != "" ? normalized + "!" : normalized')));

assert(pipeLangReadme.includes("`v0.84.0`"));
const finiteConditionalLocals = pipeLangSnippets["PipeLang finite conditional locals in terminal tree"];
assert.strictEqual(finiteConditionalLocals.prefix, "pipe-finite-conditional-locals");
assert(finiteConditionalLocals.description.includes("v0.84.0"));
assert.strictEqual(finiteConditionalLocals.body.filter((line) => line.includes(" ? ")).length, 3);
assert(finiteConditionalLocals.body.some((line) => line.includes('enabled && selected != "" ? selected + "?" : selected')));

assert(pipeLangReadme.includes("`v0.85.0`"));
const straightConditionalLocals = pipeLangSnippets["PipeLang straight-line conditional locals"];
assert.strictEqual(straightConditionalLocals.prefix, "pipe-straight-conditional-locals");
assert(straightConditionalLocals.description.includes("v0.85.0"));
assert.strictEqual(straightConditionalLocals.body.filter((line) => line.includes(" ? ")).length, 3);
assert(straightConditionalLocals.body.some((line) => line.trim() === "return final;"));
assert(!straightConditionalLocals.body.some((line) => line.includes("if (")));

const conditionalReturn = Object.values(pipeLangSnippets).find((snippet) => snippet.prefix === "pipe-conditional-return");
assert(conditionalReturn);
assert(conditionalReturn.description.includes("v0.86.0"));
assert(conditionalReturn.body.join("\n").includes('return enabled && selected != "" ? selected : normalized;'));
assert(pipeLangReadme.includes("`v0.86.0`"));

const terminalLeafReturn = Object.values(pipeLangSnippets).find((snippet) => snippet.prefix === "pipe-terminal-leaf-return");
assert(terminalLeafReturn);
assert(terminalLeafReturn.description.includes("v0.87.0"));
assert(terminalLeafReturn.body.join("\n").includes('return clean ? selected : raw;'));
assert(terminalLeafReturn.body.join("\n").includes('return finish ? normalized : raw;'));
assert(pipeLangReadme.includes("`v0.87.0`"));

const nestedReturn = Object.values(pipeLangSnippets).find((snippet) => snippet.prefix === "pipe-nested-return");
assert(nestedReturn);
assert(nestedReturn.description.includes("v0.88.0"));
assert(nestedReturn.body.join("\n").includes('return enabled ? (clean ? normalized : raw) : (fallback ? "fallback" : "");'));
assert(!nestedReturn.body.some((line) => line.includes("if (")));
assert(pipeLangReadme.includes("`v0.88.0`"));

const nestedLeafReturn = Object.values(pipeLangSnippets).find((snippet) => snippet.prefix === "pipe-nested-leaf-return");
assert(nestedLeafReturn);
assert(nestedLeafReturn.description.includes("v0.89.0"));
assert(nestedLeafReturn.body.join("\n").includes('if (enabled) {'));
assert(nestedLeafReturn.body.join("\n").includes('return clean ? (fallback ? normalized : raw) : (fallback ? raw : "");'));
assert(pipeLangReadme.includes("`v0.89.0`"));

const nestedInitializer = pipeLangSnippets["Nested straight-line initializers"];
assert.strictEqual(nestedInitializer.prefix, "pipe-nested-initializers");
assert(nestedInitializer.description.includes("v0.90.0"));
assert(nestedInitializer.body.join("\n").includes("string selected = enabled ? (clean ?"));
assert(pipeLangReadme.includes("`v0.90.0`"));

const nestedTreeInitializer = pipeLangSnippets["Nested terminal initializers"];
assert.strictEqual(nestedTreeInitializer.prefix, "pipe-nested-tree-initializers");
assert(nestedTreeInitializer.description.includes("v0.91.0"));
assert(nestedTreeInitializer.body.join("\n").includes("if (enabled) {"));
assert(nestedTreeInitializer.body.join("\n").includes("string selected = clean ? (fallback ?"));
assert(pipeLangReadme.includes("`v0.91.0`"));

const nestedArrow = pipeLangSnippets["Nested expression-bodied method"];
assert.strictEqual(nestedArrow.prefix, "pipe-nested-arrow");
assert(nestedArrow.description.includes("v0.92.0"));
assert(nestedArrow.body.join("\n").includes("=>"));
assert(nestedArrow.body.join("\n").includes("outer ? (left ?"));
assert(pipeLangReadme.includes("`v0.92.0`"));

const depthThreeReturn = pipeLangSnippets["Depth-three straight-line return"];
assert.strictEqual(depthThreeReturn.prefix, "pipe-depth-three-return");
assert(depthThreeReturn.description.includes("v0.93.0"));
assert(depthThreeReturn.body.join("\n").includes("return a ? (b ? (c ?"));
assert(pipeLangReadme.includes("`v0.93.0`"));

const depthThreeLeafReturn = pipeLangSnippets["Depth-three terminal-leaf return"];
assert.strictEqual(depthThreeLeafReturn.prefix, "pipe-depth-three-leaf-return");
assert(depthThreeLeafReturn.description.includes("v0.94.0"));
assert(depthThreeLeafReturn.body.join("\n").includes("if (enabled) {"));
assert(depthThreeLeafReturn.body.join("\n").includes("return a ? (b ? (c ?"));
assert(pipeLangReadme.includes("`v0.94.0`"));

const depthThreeArrow = pipeLangSnippets["Depth-three expression-bodied method"];
assert.strictEqual(depthThreeArrow.prefix, "pipe-depth-three-arrow");
assert(depthThreeArrow.description.includes("v0.95.0"));
assert(depthThreeArrow.body.join("\n").includes('a ? (b ? (c ? trim(raw) : raw) : "fallback") : raw;'));
assert(pipeLangReadme.includes("`v0.95.0`"));

const depthThreeInitializer = pipeLangSnippets["Depth-three straight-line initializer"];
assert.strictEqual(depthThreeInitializer.prefix, "pipe-depth-three-initializer");
assert(depthThreeInitializer.description.includes("v0.96.0"));
assert(depthThreeInitializer.body.join("\n").includes('string selected = a ? (b ? (c ? trim(raw) : raw) : "fallback") : raw;'));
assert(pipeLangReadme.includes("`v0.96.0`"));

const terminalInitializer = pipeLangSnippets["Depth-three terminal initializer"];
assert.strictEqual(terminalInitializer.prefix, "pipe-depth-three-terminal-initializer");
assert(terminalInitializer.description.includes("v0.97.0"));
assert(terminalInitializer.body.join("\n").includes("if (a) {"));
assert(terminalInitializer.body.join("\n").includes('string selected = b ? (c ? (a ?'));
assert(pipeLangReadme.includes("`v0.97.0`"));
