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
  constructor(start, end, endLine, endCharacter) {
    if (typeof start === "number") {
      this.start = new Position(start, end);
      this.end = new Position(endLine, endCharacter);
    } else {
      this.start = start;
      this.end = end;
    }
  }
}

class CompletionItem {
  constructor(label, kind) {
    this.label = label;
    this.kind = kind;
  }
}

class MarkdownString {
  constructor(value = "") {
    this.value = value;
  }

  appendMarkdown(value) {
    this.value += value;
  }
}

class Hover {
  constructor(contents, range) {
    this.contents = contents;
    this.range = range;
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
  CompletionItemKind: { EnumMember: 20 },
  CompletionItem,
  MarkdownString,
  Hover,
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

// Host-shell suggestions use the same lightweight YAML context as the providers.
const shellWorkflowLines = [
  "steps:",
  "  - kind: host",
  "    shell: ",
  "  - group:",
  "      mode: async",
  "      tasks:",
  "        - kind: host",
  "          shell: pwsh",
  "finally:",
  "  - kind: host",
  "    shell: powershell"
];
const shellWorkflow = {
  lineCount: shellWorkflowLines.length,
  lineAt(line) {
    return { text: shellWorkflowLines[line] };
  }
};
const shellInfos = helpers.analyzeYamlStructure(shellWorkflow);
assert(helpers.DOCKPIPE_STEP_KEYS.includes("shell"));
for (const line of [2, 7, 10]) {
  const completions = helpers.hostShellCompletionItems(shellInfos[line], shellWorkflowLines[line]);
  assert.deepStrictEqual(completions.map((item) => item.label), ["bash", "powershell", "pwsh"]);
  assert(completions.every((item) => item.kind === vscode.CompletionItemKind.EnumMember));
}
assert.strictEqual(helpers.hostShellCompletionItems({ kind: "stepKey", key: "cmd" }, "    cmd: "), null);
assert.strictEqual(helpers.hostShellCompletionItems({ kind: "topLevelKey", key: "shell" }, "shell: "), null);
assert.strictEqual(helpers.hostShellCompletionItems(shellInfos[2], "    shell"), null);
const shellKeyHover = helpers.hoverForWorkflowKey("shell", helpers.STEP_KEY_DETAILS);
assert(shellKeyHover.contents.value.includes("Windows PowerShell on Windows"));
assert(shellKeyHover.contents.value.includes("Bash on Unix"));
assert(shellKeyHover.contents.value.includes("conflicting overrides fail"));
const windowsShellHover = helpers.hostShellValueHover(shellInfos[10], "powershell");
assert(windowsShellHover.contents.value.includes("powershell.exe"));
assert(windowsShellHover.contents.value.includes("only on Windows"));
const pwshHover = helpers.hostShellValueHover(shellInfos[7], "pwsh");
assert(pwshHover.contents.value.includes("does not fall back"));
assert.strictEqual(helpers.hostShellValueHover(shellInfos[7], "cmd"), null);
assert.strictEqual(helpers.hostShellValueHover({ kind: "stepKey", key: "cmd" }, "pwsh"), null);

// A caller may select an installed/pinned engine schema without a sibling-source dependency.
if (process.env.DOCKPIPE_WORKFLOW_SCHEMA) {
  const schema = JSON.parse(fs.readFileSync(process.env.DOCKPIPE_WORKFLOW_SCHEMA, "utf8"));
  const completions = helpers.hostShellCompletionItems(shellInfos[2], shellWorkflowLines[2]);
  assert.deepStrictEqual(completions.map((item) => item.label).sort(), schema.$defs.workflow_step.properties.shell.enum.slice().sort());
}

const source = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
assert(!source.includes("core_from"));
assert(!source.includes("src/core"));
assert(!source.includes('language: "pipelang"'));
assert(source.includes("dockpipe_version"));
const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, "package.json"), "utf8"));
assert.strictEqual(manifest.publisher + "." + manifest.name, "dockpipe.dockpipe-language-support");
assert(manifest.extensionDependencies.includes("dockpipe.pipelang-language-support"));
console.log("Dockpipe editor authoring tests passed");
