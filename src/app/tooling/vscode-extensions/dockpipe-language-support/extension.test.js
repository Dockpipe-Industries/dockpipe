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
