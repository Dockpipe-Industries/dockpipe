#!/usr/bin/env node
const fs = require("fs");
const vm = require("vm");
const path = require("path");

const sourcePath = path.resolve(__dirname, "..", "webview", "chat.js");
const scriptText = fs.readFileSync(sourcePath, "utf8");

function runCodexAdapterConfigFixtures() {
  const manifest = JSON.parse(fs.readFileSync(path.resolve(__dirname, "..", "package.json"), "utf8"));
  const adapter = manifest?.contributes?.configuration?.properties?.["pipeon.codex.sessionAdapter"];
  if (
    adapter?.default !== "codex_app_server"
    || JSON.stringify(adapter?.enum) !== JSON.stringify(["codex_exec", "codex_app_server"])
    || adapter?.scope !== "resource"
  ) {
    throw new Error(`Pipeon Codex adapter configuration drifted: ${JSON.stringify(adapter)}`);
  }
}

async function runSessionAdapterHostFixtures() {
  const Module = require("module");
  const originalLoad = Module._load;
  const configuration = {
    effectiveValue: undefined,
    inspected: {
      key: "pipeon.codex.sessionAdapter",
      defaultValue: "codex_app_server",
      globalValue: undefined,
      workspaceValue: undefined,
      workspaceFolderValue: undefined,
    },
  };
  const vscodeMock = {
    Uri: { file: (fsPath) => ({ fsPath }) },
    workspace: {
      workspaceFolders: [{ uri: { fsPath: "C:\\fixture\\workspace" } }],
      getConfiguration(section) {
        if (section !== "pipeon") {
          throw new Error(`Unexpected configuration section: ${section}`);
        }
        return {
          get(key, fallback) {
            if (key !== "codex.sessionAdapter") {
              throw new Error(`Unexpected configuration key: ${key}`);
            }
            return configuration.effectiveValue === undefined ? fallback : configuration.effectiveValue;
          },
          inspect(key) {
            if (key !== "codex.sessionAdapter") {
              throw new Error(`Unexpected inspected key: ${key}`);
            }
            return { ...configuration.inspected };
          },
        };
      },
    },
  };
  Module._load = function(request, parent, isMain) {
    if (request === "vscode") {
      return vscodeMock;
    }
    return originalLoad.call(this, request, parent, isMain);
  };
  let adapterTest;
  try {
    adapterTest = require(path.resolve(__dirname, "..", "extension.js")).__sessionAdapterTest;
  } finally {
    Module._load = originalLoad;
  }
  if (!adapterTest) {
    throw new Error("Extension session-adapter fixture seam is unavailable.");
  }

  const setConfiguration = (effectiveValue, explicitKey = "", explicitValue = undefined) => {
    configuration.effectiveValue = effectiveValue;
    configuration.inspected = {
      key: "pipeon.codex.sessionAdapter",
      defaultValue: "codex_app_server",
      globalValue: undefined,
      workspaceValue: undefined,
      workspaceFolderValue: undefined,
    };
    if (explicitKey) {
      configuration.inspected[explicitKey] = explicitValue;
    }
  };
  const storedState = (sessions) => ({ activeSessionId: sessions[0]?.id, sessions });
  const storedSession = (id, adapterEvidence = Symbol.for("omitted")) => {
    const session = {
      id,
      title: "Chat",
      createdAt: "2026-08-04T00:00:00.000Z",
      updatedAt: "2026-08-04T00:00:00.000Z",
      messages: [],
    };
    if (adapterEvidence !== Symbol.for("omitted")) {
      session.codexSessionAdapter = adapterEvidence;
    }
    return session;
  };
  const expectRejected = (name, fn) => {
    let rejected = false;
    try {
      fn();
    } catch {
      rejected = true;
    }
    if (!rejected) {
      throw new Error(`Invalid retained adapter evidence was accepted: ${name}`);
    }
  };

  setConfiguration(undefined);
  if (adapterTest.configuredCodexSessionAdapter("C:\\fixture\\workspace") !== "codex_app_server") {
    throw new Error("Pipeon configuration fallback is not exactly codex_app_server.");
  }
  const defaultSession = adapterTest.createSession("default session");
  if (defaultSession.codexSessionAdapter !== "codex_app_server") {
    throw new Error("A new session without an explicit override did not retain codex_app_server.");
  }

  setConfiguration("codex_exec", "globalValue", "codex_exec");
  const escapedSession = adapterTest.createSession("exec escape hatch");
  if (escapedSession.codexSessionAdapter !== "codex_exec") {
    throw new Error("A new session did not retain the explicit codex_exec escape hatch.");
  }

  setConfiguration("codex_app_server");
  const legacyDefault = storedState([storedSession("legacy-default")]);
  if (!adapterTest.storedChatStateNeedsCodexSessionAdapterMigration(legacyDefault)) {
    throw new Error("A legacy session without adapter evidence did not require one-time migration.");
  }
  const migratedDefault = adapterTest.normalizeStoredChatState(legacyDefault, "C:\\fixture\\workspace");
  if (migratedDefault.sessions[0].codexSessionAdapter !== "codex_exec") {
    throw new Error("A legacy session without an explicit override did not retain the historical codex_exec default.");
  }
  const persistedMigration = adapterTest.compactChatStoreForPersistence(migratedDefault);
  if (
    persistedMigration.sessions[0].codexSessionAdapter !== "codex_exec"
    || adapterTest.storedChatStateNeedsCodexSessionAdapterMigration(persistedMigration)
  ) {
    throw new Error("Legacy adapter migration was not persisted as a one-time closed value.");
  }

  setConfiguration("codex_app_server", "workspaceValue", "codex_app_server");
  const migratedExplicit = adapterTest.normalizeStoredChatState(
    storedState([storedSession("legacy-explicit")]),
    "C:\\fixture\\workspace"
  );
  if (migratedExplicit.sessions[0].codexSessionAdapter !== "codex_app_server") {
    throw new Error("A legacy session did not retain its explicit codex_app_server override.");
  }

  const pinnedState = storedState([
    storedSession("stored-exec", "codex_exec"),
    storedSession("stored-app-server", "codex_app_server"),
  ]);
  setConfiguration("codex_app_server");
  const firstReload = adapterTest.normalizeStoredChatState(pinnedState, "C:\\fixture\\workspace");
  setConfiguration("codex_exec", "workspaceFolderValue", "codex_exec");
  const secondReload = adapterTest.normalizeStoredChatState(
    adapterTest.compactChatStoreForPersistence(firstReload),
    "C:\\fixture\\workspace"
  );
  if (
    secondReload.sessions[0].codexSessionAdapter !== "codex_exec"
    || secondReload.sessions[1].codexSessionAdapter !== "codex_app_server"
  ) {
    throw new Error("Stored adapter choices changed across reload or later configuration changes.");
  }

  setConfiguration("codex_app_server");
  const beforeChange = adapterTest.createSession("before change");
  setConfiguration("codex_exec", "workspaceValue", "codex_exec");
  const afterChange = adapterTest.createSession("after change");
  if (
    adapterTest.retainedCodexSessionAdapter(beforeChange) !== "codex_app_server"
    || adapterTest.retainedCodexSessionAdapter(afterChange) !== "codex_exec"
  ) {
    throw new Error("A configuration change altered an existing session or failed to affect a later new session.");
  }

  for (const session of [beforeChange, afterChange]) {
    const retained = adapterTest.retainedCodexSessionAdapter(session);
    for (let turn = 0; turn < 2; turn += 1) {
      const prepared = adapterTest.buildProviderPoolChatToolArguments(
        "C:\\fixture\\workspace",
        `turn ${turn}`,
        {},
        { provider: "codex", model: "config", sessionId: session.id, sessionAdapter: retained }
      );
      if (prepared.toolArguments.session_adapter !== retained) {
        throw new Error("A Codex turn did not send its session's retained adapter exactly.");
      }
    }
  }

  for (const [name, evidence] of [
    ["omitted", Symbol.for("omitted")],
    ["empty", ""],
    ["unknown", "unknown"],
    ["malformed", { value: "codex_exec" }],
    ["extended", "codex_exec "],
    ["substituted", { toString: () => "codex_app_server" }],
  ]) {
    const session = storedSession(`invalid-${name}`, evidence);
    expectRejected(name, () => adapterTest.retainedCodexSessionAdapter(session));
    expectRejected(`${name} request`, () => adapterTest.buildProviderPoolChatToolArguments(
      "C:\\fixture\\workspace",
      "question",
      {},
      { provider: "codex", model: "config", sessionId: session.id, sessionAdapter: session.codexSessionAdapter }
    ));
    if (name !== "omitted") {
      expectRejected(`${name} persisted`, () => adapterTest.normalizeStoredChatState(
        storedState([session]),
        "C:\\fixture\\workspace"
      ));
    }
  }

  const routeCases = [
    ["codex", "codex_app_server", "normal question", "direct_chat", [1, 1, 1]],
    ["codex", "codex_exec", "normal question", "direct_chat", [0, 0, 0]],
    ["codex", "codex_app_server", "/status", "direct_chat", [0, 0, 0]],
    ["ollama", "codex_app_server", "normal question", "direct_chat", [0, 0, 0]],
    ["claude", "codex_app_server", "normal question", "direct_chat", [0, 0, 0]],
    ["codex", "codex_app_server", "workflow", "workflow", [0, 0, 0]],
    ["codex", "codex_app_server", "prepared action", "prepared_action", [0, 0, 0]],
    ["codex", "codex_app_server", "bounded worker", "bounded_worker", [0, 0, 0]],
  ];
  for (const [provider, adapter, text, routeKind, expected] of routeCases) {
    const counts = [0, 0, 0];
    const owner = {
      beginApprovalInvocation() { counts[0] += 1; return { id: "approval" }; },
      beginUserInputInvocation() { counts[1] += 1; return { id: "input", providerPoolChatId: "chat" }; },
      beginCancellationInvocation(_sessionId, chatId) {
        if (chatId !== "chat") throw new Error("Cancellation did not share the interactive chat invocation.");
        counts[2] += 1;
        return { id: "cancel" };
      },
    };
    adapterTest.beginTransientInteractiveInvocations(owner, "session", provider, adapter, text, routeKind);
    if (JSON.stringify(counts) !== JSON.stringify(expected)) {
      throw new Error(`Interactive monitors drifted for ${provider}/${adapter}/${routeKind}/${text}: ${JSON.stringify(counts)}`);
    }
  }

  const appServerResult = (state, metadata = {}) => ({
    providerPoolState: state,
    metadata: {
      session_adapter: "codex_app_server",
      terminal_summary: state === "ready" ? "turn_completed" : state,
      outcome_unknown: false,
      ...metadata,
    },
    text: "raw-response-secret",
    correlation: "raw-correlation-secret",
    approval: { decision: "raw-approval-secret" },
    userInput: { answer: "raw-input-secret" },
    cancellation: { reason: "raw-cancellation-secret" },
    fallback: { reason: "raw-fallback-secret" },
  });
  const snapshotSession = storedSession("snapshot-success", "codex_app_server");
  const successfulResult = appServerResult("ready", {
    selected_model: "gpt-5.6-terra",
    selected_reasoning_effort: "high",
    approval_policy: "human-review",
    sandbox: "workspace-write",
    raw_metadata: "raw-metadata-secret",
    session_id: "raw-provider-session-secret",
  });
  if (!adapterTest.updateCodexAppServerPostTurnSnapshot(snapshotSession, "codex", "normal question", successfulResult)) {
    throw new Error("A retained App Server direct-chat result did not create a post-turn snapshot.");
  }
  const expectedSnapshot = {
    adapter: "codex_app_server",
    state: "completed",
    outcomeUnknown: false,
    terminalSummaryId: "turn_completed",
    modelRef: "gpt-5.6-terra",
    reasoningRef: "high",
    approvalRef: "human-review",
    sandboxRef: "workspace-write",
  };
  if (JSON.stringify(snapshotSession.codexAppServerPostTurnSnapshot) !== JSON.stringify(expectedSnapshot)) {
    throw new Error(`Successful App Server snapshot was not exact: ${JSON.stringify(snapshotSession.codexAppServerPostTurnSnapshot)}`);
  }
  const projectedSnapshot = adapterTest.projectCodexAppServerPostTurnSnapshot(snapshotSession);
  if (
    JSON.stringify(projectedSnapshot) !== JSON.stringify({
      state: "completed",
      outcomeUnknown: false,
      terminalSummaryId: "turn_completed",
      modelRef: "gpt-5.6-terra",
      reasoningRef: "high",
      approvalRef: "human-review",
      sandboxRef: "workspace-write",
    })
    || Object.prototype.hasOwnProperty.call(projectedSnapshot, "adapter")
    || Object.prototype.hasOwnProperty.call(projectedSnapshot, "codexSessionAdapter")
  ) {
    throw new Error(`Webview snapshot projection was not separately allowlisted: ${JSON.stringify(projectedSnapshot)}`);
  }
  const snapshotLeakCheck = JSON.stringify({ snapshot: snapshotSession.codexAppServerPostTurnSnapshot, projectedSnapshot });
  for (const forbidden of ["raw-response-secret", "raw-correlation-secret", "raw-approval-secret", "raw-input-secret", "raw-cancellation-secret", "raw-fallback-secret", "raw-metadata-secret", "raw-provider-session-secret"]) {
    if (snapshotLeakCheck.includes(forbidden)) {
      throw new Error(`App Server snapshot retained forbidden result evidence: ${forbidden}`);
    }
  }

  const snapshotBeforeReload = JSON.stringify(snapshotSession.codexAppServerPostTurnSnapshot);
  const persistedSnapshotState = adapterTest.compactChatStoreForPersistence(storedState([snapshotSession]));
  setConfiguration("codex_exec", "workspaceFolderValue", "codex_exec");
  const reloadedSnapshotState = adapterTest.normalizeStoredChatState(persistedSnapshotState, "C:\\fixture\\workspace");
  if (
    JSON.stringify(reloadedSnapshotState.sessions[0].codexAppServerPostTurnSnapshot) !== snapshotBeforeReload
    || reloadedSnapshotState.sessions[0].codexSessionAdapter !== "codex_app_server"
  ) {
    throw new Error("App Server snapshot or retained adapter changed across persistence, reload, or later configuration change.");
  }

  for (const [responseState, expectedState, terminalSummary] of [
    ["failed", "failed", "turn_failed"],
    ["cancelled", "cancelled", "cancelled"],
  ]) {
    const session = storedSession(`snapshot-${responseState}`, "codex_app_server");
    adapterTest.updateCodexAppServerPostTurnSnapshot(session, "codex", "normal question", appServerResult(responseState, { terminal_summary: terminalSummary }));
    if (session.codexAppServerPostTurnSnapshot?.state !== expectedState || session.codexAppServerPostTurnSnapshot?.outcomeUnknown !== false) {
      throw new Error(`${responseState} App Server response did not retain its exact neutral projection.`);
    }
  }
  const unknownSession = storedSession("snapshot-unknown", "codex_app_server");
  adapterTest.updateCodexAppServerPostTurnSnapshot(unknownSession, "codex", "normal question", appServerResult("failed", {
    terminal_summary: "transport_closed",
    outcome_unknown: true,
  }));
  const unknownPersisted = adapterTest.compactChatStoreForPersistence(storedState([unknownSession]));
  const unknownReloaded = adapterTest.normalizeStoredChatState(unknownPersisted, "C:\\fixture\\workspace");
  if (
    unknownReloaded.sessions[0].codexAppServerPostTurnSnapshot?.state !== "recovery_required"
    || unknownReloaded.sessions[0].codexAppServerPostTurnSnapshot?.outcomeUnknown !== true
  ) {
    throw new Error("Unknown-outcome evidence did not survive reload as recovery-required.");
  }

  const recoveryRequiredSession = unknownReloaded.sessions[0];
  recoveryRequiredSession.messages = [{ id: "retained-message", role: "assistant", text: "retained", format: "markdown", createdAt: "2026-08-04T00:00:00.000Z" }];
  const recoveryRequiredBefore = JSON.stringify(recoveryRequiredSession);
  if (!adapterTest.shouldBlockCodexAppServerRecoveryRequiredTurn(recoveryRequiredSession, "codex", "normal question", "direct_chat")) {
    throw new Error("Exact retained recovery-required App Server evidence did not activate the host turn guard.");
  }
  const guardedOwner = Object.create(adapterTest.PipeonChatViewProvider.prototype);
  guardedOwner.chatStore = {
    activeSessionId: recoveryRequiredSession.id,
    sessions: [recoveryRequiredSession],
    chatProvider: "codex",
  };
  guardedOwner.channel = {
    appendLine() { throw new Error("A blocked host ask wrote a diagnostic."); },
    error() { throw new Error("A blocked host ask wrote an error diagnostic."); },
  };
  const downstreamCalls = [];
  guardedOwner.saveAndRefresh = async () => { downstreamCalls.push("save"); };
  guardedOwner.beginApprovalInvocation = () => { downstreamCalls.push("approval"); };
  guardedOwner.beginUserInputInvocation = () => { downstreamCalls.push("user-input"); };
  guardedOwner.beginCancellationInvocation = () => { downstreamCalls.push("cancellation"); };
  for (let attempt = 0; attempt < 2; attempt += 1) {
    await guardedOwner.ask("C:\\fixture\\workspace", "normal question", "ask", "balanced", [], recoveryRequiredSession.id, "codex", "config");
  }
  if (JSON.stringify(recoveryRequiredSession) !== recoveryRequiredBefore || downstreamCalls.length !== 0) {
    throw new Error(`Repeated directly posted host asks mutated retained state or reached downstream turn activity: ${JSON.stringify(downstreamCalls)}`);
  }

  adapterTest.clearSessionMessages(recoveryRequiredSession);
  const clearedRecoveryRequiredBefore = JSON.stringify(recoveryRequiredSession);
  setConfiguration("codex_exec", "workspaceFolderValue", "codex_exec");
  await guardedOwner.ask("C:\\fixture\\workspace", "normal question", "ask", "balanced", [], recoveryRequiredSession.id, "codex", "config");
  if (JSON.stringify(recoveryRequiredSession) !== clearedRecoveryRequiredBefore || downstreamCalls.length !== 0) {
    throw new Error("Clearing messages or changing configuration unblocked or mutated the retained recovery-required session.");
  }

  const guardCases = [
    ["completed", false],
    ["failed", false],
    ["cancelled", false],
  ];
  for (const [state, expected] of guardCases) {
    const session = storedSession(`guard-${state}`, "codex_app_server");
    session.codexAppServerPostTurnSnapshot = {
      adapter: "codex_app_server",
      state,
      outcomeUnknown: false,
    };
    if (adapterTest.shouldBlockCodexAppServerRecoveryRequiredTurn(session, "codex", "normal question", "direct_chat") !== expected) {
      throw new Error(`${state} App Server snapshot incorrectly activated the host turn guard.`);
    }
  }
  const guardRouteCases = [
    [storedSession("guard-missing", "codex_app_server"), "codex", "normal question", "direct_chat"],
    [storedSession("guard-exec", "codex_exec"), "codex", "normal question", "direct_chat"],
    [recoveryRequiredSession, "codex", "/status", "direct_chat"],
    [recoveryRequiredSession, "ollama", "normal question", "direct_chat"],
    [recoveryRequiredSession, "claude", "normal question", "direct_chat"],
    [recoveryRequiredSession, "codex", "workflow", "workflow"],
    [recoveryRequiredSession, "codex", "prepared action", "prepared_action"],
    [recoveryRequiredSession, "codex", "bounded worker", "bounded_worker"],
  ];
  for (const [session, provider, text, routeKind] of guardRouteCases) {
    if (adapterTest.shouldBlockCodexAppServerRecoveryRequiredTurn(session, provider, text, routeKind)) {
      throw new Error(`${provider}/${routeKind}/${text} incorrectly activated the App Server recovery-required turn guard.`);
    }
  }

  const sparseSession = storedSession("snapshot-sparse", "codex_app_server");
  adapterTest.updateCodexAppServerPostTurnSnapshot(sparseSession, "codex", "normal question", appServerResult("failed", {
    terminal_summary: "turn_failed",
  }));
  for (const absent of ["modelRef", "reasoningRef", "approvalRef", "sandboxRef"]) {
    if (Object.prototype.hasOwnProperty.call(sparseSession.codexAppServerPostTurnSnapshot, absent)) {
      throw new Error(`Missing App Server policy evidence was inferred: ${absent}`);
    }
  }

  const noSnapshotSession = storedSession("snapshot-legacy-none", "codex_app_server");
  const noSnapshotReloaded = adapterTest.normalizeStoredChatState(storedState([noSnapshotSession]), "C:\\fixture\\workspace");
  if (
    Object.prototype.hasOwnProperty.call(noSnapshotReloaded.sessions[0], "codexAppServerPostTurnSnapshot")
    || adapterTest.projectCodexAppServerPostTurnSnapshot(noSnapshotReloaded.sessions[0]) !== null
  ) {
    throw new Error("A session without snapshot evidence gained inferred App Server history.");
  }

  const invalidSnapshotFixtures = [
    ["empty", {}],
    ["unknown", { ...expectedSnapshot, state: "unknown" }],
    ["malformed", []],
    ["extended", { ...expectedSnapshot, extra: "forbidden" }],
    ["substituted", { ...expectedSnapshot, adapter: "codex_exec" }],
    ["substituted-summary", { ...expectedSnapshot, terminalSummaryId: "cancelled" }],
    ["oversized", { ...expectedSnapshot, terminalSummaryId: "a".repeat(129) }],
    ["non-string", { ...expectedSnapshot, modelRef: 42 }],
    ["empty-string", { ...expectedSnapshot, terminalSummaryId: "" }],
  ];
  for (const [name, snapshot] of invalidSnapshotFixtures) {
    const session = storedSession(`snapshot-invalid-${name}`, "codex_app_server");
    session.codexAppServerPostTurnSnapshot = snapshot;
    expectRejected(`snapshot ${name}`, () => adapterTest.normalizeStoredChatState(storedState([session]), "C:\\fixture\\workspace"));
  }

  const clearSnapshotBefore = JSON.stringify(snapshotSession.codexAppServerPostTurnSnapshot);
  const clearAdapterBefore = snapshotSession.codexSessionAdapter;
  snapshotSession.messages = [{ id: "message-to-clear" }];
  adapterTest.clearSessionMessages(snapshotSession);
  if (
    snapshotSession.messages.length !== 0
    || snapshotSession.codexSessionAdapter !== clearAdapterBefore
    || JSON.stringify(snapshotSession.codexAppServerPostTurnSnapshot) !== clearSnapshotBefore
  ) {
    throw new Error("Clearing ordinary messages changed the retained adapter or App Server snapshot.");
  }

  const nonSnapshotRoutes = [
    [storedSession("snapshot-exec", "codex_exec"), "codex", "normal question", "direct_chat"],
    [storedSession("snapshot-slash", "codex_app_server"), "codex", "/codex explicit", "direct_chat"],
    [storedSession("snapshot-ollama", "codex_app_server"), "ollama", "normal question", "direct_chat"],
    [storedSession("snapshot-claude", "codex_app_server"), "claude", "normal question", "direct_chat"],
    [storedSession("snapshot-workflow", "codex_app_server"), "codex", "workflow", "workflow"],
    [storedSession("snapshot-prepared", "codex_app_server"), "codex", "prepared action", "prepared_action"],
    [storedSession("snapshot-worker", "codex_app_server"), "codex", "bounded worker", "bounded_worker"],
  ];
  for (const [session, provider, text, routeKind] of nonSnapshotRoutes) {
    if (adapterTest.updateCodexAppServerPostTurnSnapshot(session, provider, text, successfulResult, routeKind) || Object.prototype.hasOwnProperty.call(session, "codexAppServerPostTurnSnapshot")) {
      throw new Error(`${provider}/${routeKind}/${text} created or altered an App Server snapshot.`);
    }
  }

  const compact = adapterTest.compactChatStoreForPersistence({
    activeSessionId: beforeChange.id,
    sessions: [beforeChange],
    transientApproval: { secret: "approval-secret" },
    transientUserInput: { secret: "input-secret" },
    transientCancellation: { secret: "cancellation-secret" },
    prompt: "prompt-secret",
    response: "response-secret",
    correlation: "correlation-secret",
    fallback: "fallback-secret",
  });
  const serialized = JSON.stringify(compact);
  for (const forbidden of ["transientApproval", "transientUserInput", "transientCancellation", "approval-secret", "input-secret", "cancellation-secret", "prompt-secret", "response-secret", "correlation-secret", "fallback-secret"]) {
    if (serialized.includes(forbidden)) {
      throw new Error(`Session persistence captured transient state: ${forbidden}`);
    }
  }
}

class FakeElement {
  constructor(id) {
    this.id = id;
    this.value = "";
    this.disabled = false;
    this.checked = false;
    this.innerHTML = "";
    this.textContent = "";
    this.style = {};
    this.scrollHeight = 100;
    this.scrollTop = 0;
    this.clientHeight = 80;
    this.dataset = {};
    this.className = "";
    this.listeners = new Map();
    this.attributes = new Map();
    this.queryResults = new Map();
    const syncClassName = (set) => {
      this.className = Array.from(set).join(" ");
    };
    const classSet = new Set();
    this.classList = {
      add: (...tokens) => {
        for (const token of tokens) {
          if (token) classSet.add(token);
        }
        syncClassName(classSet);
      },
      remove: (...tokens) => {
        for (const token of tokens) {
          classSet.delete(token);
        }
        syncClassName(classSet);
      },
      toggle: (token, force) => {
        if (!token) return false;
        let present = classSet.has(token);
        if (typeof force === "boolean") {
          if (force) classSet.add(token);
          else classSet.delete(token);
          present = force;
        } else if (present) {
          classSet.delete(token);
          present = false;
        } else {
          classSet.add(token);
          present = true;
        }
        syncClassName(classSet);
        return present;
      },
    };
  }

  addEventListener(type, handler) {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, []);
    }
    this.listeners.get(type).push(handler);
  }

  focus() {}

  closest() {
    return null;
  }

  getAttribute(name) {
    return this.attributes.get(name) || this[name] || null;
  }

  setAttribute(name, value) {
    this.attributes.set(name, value);
  }

  querySelectorAll(selector) {
    return this.queryResults.get(selector) || [];
  }
}

const elements = new Map();
function getElement(id) {
  if (!elements.has(id)) {
    elements.set(id, new FakeElement(id));
  }
  return elements.get(id);
}

const windowListeners = new Map();
const posted = [];
const savedViewStates = [];

const context = {
  console,
  acquireVsCodeApi() {
    return {
      postMessage(message) {
        posted.push(message);
      },
      getState() {
        return null;
      },
      setState(state) {
        savedViewStates.push(state);
      },
    };
  },
  document: {
    getElementById(id) {
      return getElement(id);
    },
    querySelector(selector) {
      if (selector === ".composerWrap") {
        return getElement("composerWrap");
      }
      if (selector === ".header") {
        return getElement("header");
      }
      if (selector === ".headerActions") {
        return getElement("headerActions");
      }
      return null;
    },
    querySelectorAll(selector) {
      if (selector === ".primitiveTile") {
        const dockpipe = getElement("primitive-dockpipe");
        dockpipe.setAttribute("data-primitive-type", "dockpipe");
        const model = getElement("primitive-model");
        model.setAttribute("data-primitive-type", "model");
        const loop = getElement("primitive-loop");
        loop.setAttribute("data-primitive-type", "loop");
        return [dockpipe, model, loop];
      }
      return [];
    },
    body: new FakeElement("body"),
  },
  window: {
    addEventListener(type, handler) {
      if (!windowListeners.has(type)) {
        windowListeners.set(type, []);
      }
      windowListeners.get(type).push(handler);
    },
  },
  HTMLElement: FakeElement,
};

vm.createContext(context);
getElement("pipeon-initial-state").textContent = JSON.stringify({
  shellVersion: "reasoning-templates-v1",
  messages: [{
    id: "m1",
    role: "assistant",
    html: "<p>hi</p>",
    run: {
      artifactDir: "scope:package:dorkpipe:edit/req_123",
      artifactVersion: "v2",
      state: "prepared",
      validationStatus: "patch_applies",
      structuredEditCount: 1,
      structuredEdits: [{
        id: "replace_range-src-index-ts-1",
        op: "replace_range",
        language: "typescript",
        targetFile: "src/index.ts",
        description: "Update function renderSettings in src/index.ts.",
        target: { kind: "function", symbolName: "renderSettings", symbolKind: "function" },
        range: { startLine: 10, oldLineCount: 3, newLineCount: 4 },
      }],
      traceEvents: [{
        requestId: "req_123",
        phase: "edit",
        eventType: "planning",
        label: "Planning edit strategy",
        metadata: { candidate_count: 2 },
      }],
    },
  }],
  reasoningTemplates: [{ id: "dockpipe.default", name: "DorkPipe Default", locked: true, builtIn: true }],
  activeTemplate: { id: "dockpipe.default", name: "DorkPipe Default" },
  activeTemplateId: "dockpipe.default",
  modelStore: { entries: [{ id: "dorkpipe.default", label: "Default Local Model" }] },
  transientApproval: {
    uiReference: "approval-ui-fixture",
    reason: "command_execution",
    allowedDecisions: ["approve", "deny"],
    state: "pending",
  },
  transientCancellation: {
    uiReference: "cancellation-ui-fixture",
    state: "pending",
  },
  isBusy: true,
});
new vm.Script(scriptText, { filename: "pipeon-webview-inline.js" }).runInContext(context);

const stages = posted.filter((item) => item && item.type === "diag").map((item) => item.stage);
const ready = posted.some((item) => item && item.type === "webviewReady");
const clientError = posted.find((item) => item && item.type === "clientError");

if (clientError) {
  throw new Error(`Webview reported client error: ${clientError.kind}: ${clientError.message}`);
}

if (!ready) {
  throw new Error(`Webview did not report ready. Diag stages: ${stages.join(", ")}`);
}

const requiredStages = [
  "script-start",
  "vscode-api-ready",
  "dom-ready",
  "initial-state-parsed",
  "listeners-attached",
  "initial-render-complete",
  "ready-sent",
];

for (const stage of requiredStages) {
  if (!stages.includes(stage)) {
    throw new Error(`Missing diag stage: ${stage}. Seen: ${stages.join(", ")}`);
  }
}

const initialStateDiag = posted.find((item) => item && item.type === "diag" && item.stage === "initial-state-parsed");
if (!initialStateDiag || !initialStateDiag.extra || initialStateDiag.extra.messages !== 1 || initialStateDiag.extra.templates !== 1) {
  throw new Error(`Initial state was not parsed as the expected object. Diag: ${JSON.stringify(initialStateDiag)}`);
}

const readyMessage = posted.find((item) => item && item.type === "webviewReady");
if (!readyMessage || readyMessage.shellVersion !== "reasoning-templates-v1") {
  throw new Error(`Webview ready message did not carry the expected shell version. Got: ${JSON.stringify(readyMessage)}`);
}

const transcript = getElement("transcript");
if (!transcript.innerHTML.includes("Codex requests permission to run a command.")) {
  throw new Error("Blocked App Server chat did not render the neutral approval card.");
}
if (!transcript.innerHTML.includes(">Approve</button>") || !transcript.innerHTML.includes(">Deny</button>")) {
  throw new Error("Command approval card did not render its exact allowed decisions.");
}
for (const forbidden of ["process_incarnation_id", "connection_id", "request_id", "provider rpc", "rm -rf", "prompt-secret"]) {
  if (transcript.innerHTML.toLowerCase().includes(forbidden.toLowerCase())) {
    throw new Error(`Approval DOM leaked forbidden content: ${forbidden}`);
  }
}

const prompt = getElement("prompt");
const mode = getElement("modeSelect");
const profile = getElement("modelProfileSelect");
prompt.value = "hello";
mode.value = "ask";
profile.value = "balanced";
const promptKeydown = prompt.listeners.get("keydown") || [];
for (const handler of promptKeydown) {
  handler({
    key: "Enter",
    isComposing: false,
    altKey: false,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    preventDefault() {},
    stopPropagation() {},
  });
}

const askMessage = posted.find((item) => item && item.type === "ask");
if (!askMessage) {
  throw new Error("Enter key path did not emit an ask message.");
}

const approvalCard = new FakeElement("approval-card");
const approveButton = new FakeElement("approval-approve");
const denyButton = new FakeElement("approval-deny");
approveButton.setAttribute("data-approval-reference", "approval-ui-fixture");
approveButton.setAttribute("data-approval-decision", "approve");
denyButton.setAttribute("data-approval-reference", "approval-ui-fixture");
denyButton.setAttribute("data-approval-decision", "deny");
approvalCard.queryResults.set("button[data-approval-decision]", [approveButton, denyButton]);
approveButton.closest = (selector) => selector === "[data-approval-decision]" ? approveButton : (selector === "[data-approval-card]" ? approvalCard : null);
denyButton.closest = (selector) => selector === "[data-approval-decision]" ? denyButton : (selector === "[data-approval-card]" ? approvalCard : null);
for (const handler of transcript.listeners.get("click") || []) {
  handler({ target: approveButton });
  handler({ target: approveButton });
}
const approvalMessages = posted.filter((item) => item && item.type === "approvalDecision");
if (approvalMessages.length !== 1 || approvalMessages[0].uiReference !== "approval-ui-fixture" || approvalMessages[0].decision !== "approve") {
  throw new Error(`Approval click did not emit exactly one safe host message. Got: ${JSON.stringify(approvalMessages)}`);
}
if (!approveButton.disabled || !denyButton.disabled) {
  throw new Error("Approval click did not disable both controls immediately.");
}

if (!transcript.innerHTML.includes("Cancel this Codex turn") || !transcript.innerHTML.includes(">Cancel</button>")) {
  throw new Error("Active App Server chat did not render the neutral cancellation control.");
}
for (const forbidden of ["process_incarnation_id", "connection_id", "interaction_id", "session_id", "user_requested", "cancellation_scope", "cancellation_intent"]) {
  if (transcript.innerHTML.includes(forbidden)) {
    throw new Error(`Cancellation DOM leaked host-only content: ${forbidden}`);
  }
}
const cancellationCard = new FakeElement("cancellation-card");
const cancellationButton = new FakeElement("cancellation-button");
cancellationButton.setAttribute("data-cancellation-reference", "cancellation-ui-fixture");
cancellationCard.queryResults.set("button[data-cancellation-request]", [cancellationButton]);
cancellationButton.closest = (selector) => selector === "[data-cancellation-request]" ? cancellationButton : (selector === "[data-cancellation-card]" ? cancellationCard : null);
for (const handler of transcript.listeners.get("click") || []) {
  handler({ target: cancellationButton });
  handler({ target: cancellationButton });
}
const cancellationMessages = posted.filter((item) => item && item.type === "cancellationIntent");
if (
  cancellationMessages.length !== 1
  || JSON.stringify(cancellationMessages[0]) !== JSON.stringify({ type: "cancellationIntent", uiReference: "cancellation-ui-fixture" })
) {
  throw new Error(`Cancellation click did not emit exactly one UI-reference-only host message: ${JSON.stringify(cancellationMessages)}`);
}
if (!cancellationButton.disabled) {
  throw new Error("Cancellation click did not disable the control immediately.");
}

const settingsBtn = getElement("settingsBtn");
const header = getElement("header");
const studioSurface = getElement("studioSurface");
const settingsStudio = getElement("settingsStudio");
const templateStudio = getElement("templateStudio");
const modelStudio = getElement("modelStudio");
const openModelManagerBtn = getElement("openModelManagerBtn");
const studioBackBtn = getElement("studioBackBtn");
const headerActions = getElement("headerActions");
const runStudio = getElement("runStudio");
if (!settingsBtn.listeners.has("click")) {
  throw new Error("Settings button did not attach a click handler.");
}
for (const handler of settingsBtn.listeners.get("click") || []) {
  handler({});
}
const openStudioMessage = posted.find((item) => item && item.type === "openReasoningStudio");
if (!openStudioMessage || openStudioMessage.mode !== "settings") {
  throw new Error(`Settings click did not request the main settings studio. Got: ${JSON.stringify(openStudioMessage)}`);
}
const messageHandlers = windowListeners.get("message") || [];
for (const handler of messageHandlers) {
  handler({ data: { type: "forceOpenSettings", mode: "settings" } });
}
if (studioSurface.className.includes("hidden")) {
  throw new Error("Force-opening settings did not reveal the main studio surface.");
}
if (settingsStudio.className.includes("hidden")) {
  throw new Error("Force-opening settings did not reveal the settings summary surface.");
}
if (!header.className.includes("hidden")) {
  throw new Error("Settings mode should hide the top chat header.");
}
if (!headerActions.className.includes("hidden")) {
  throw new Error("Settings mode should hide the chat header actions.");
}
if (!studioBackBtn.className.includes("hidden")) {
  throw new Error("Settings mode should hide the back button.");
}
if (!openModelManagerBtn.listeners.has("click")) {
  throw new Error("Model manager button did not attach a click handler.");
}
for (const handler of openModelManagerBtn.listeners.get("click") || []) {
  handler({});
}
if (studioSurface.className.includes("hidden")) {
  throw new Error("Model manager action should keep the workflow handoff studio visible.");
}
if (settingsStudio.className.includes("hidden")) {
  throw new Error("Model manager action should stay on the workflow handoff surface.");
}
if (!modelStudio.className.includes("hidden") || !templateStudio.className.includes("hidden")) {
  throw new Error("Parked model/template surfaces should remain hidden.");
}
if (!studioBackBtn.className.includes("hidden")) {
  throw new Error("Workflow handoff should keep the back button hidden.");
}

for (const handler of messageHandlers) {
  handler({ data: { type: "focusRunInspector", messageId: "m1" } });
}
if (runStudio.className.includes("hidden")) {
  throw new Error("Focusing the run inspector did not reveal the run studio surface.");
}
if (studioBackBtn.className.includes("hidden")) {
  throw new Error("Run inspector should show the back button.");
}

const baseWebviewState = JSON.parse(getElement("pipeon-initial-state").textContent);
const renderWebviewState = (state) => {
  for (const handler of messageHandlers) {
    handler({ data: { type: "state", state } });
  }
};
const completedDisplay = {
  state: "completed",
  outcomeUnknown: false,
  terminalSummaryId: "turn_completed",
  modelRef: "gpt-5.6-terra",
  reasoningRef: "high",
  approvalRef: "human-review",
  sandboxRef: "workspace-write",
};
renderWebviewState({
  ...baseWebviewState,
  messages: [],
  transientApproval: null,
  transientUserInput: null,
  transientCancellation: null,
  appServerStatus: completedDisplay,
});
for (const expected of ["Codex App Server status", "Completed", "turn_completed", "gpt-5.6-terra", "high", "human-review", "workspace-write"]) {
  if (!transcript.innerHTML.includes(expected)) {
    throw new Error(`Valid App Server status omitted display evidence: ${expected}`);
  }
}
for (const [state, label] of [["failed", "Failed"], ["cancelled", "Interrupted"]]) {
  renderWebviewState({
    ...baseWebviewState,
    messages: [],
    transientApproval: null,
    transientUserInput: null,
    transientCancellation: null,
    appServerStatus: { state, outcomeUnknown: false, terminalSummaryId: state === "failed" ? "turn_failed" : "cancelled" },
  });
  if (!transcript.innerHTML.includes(`data-provider-status="${state}"`) || !transcript.innerHTML.includes(label)) {
    throw new Error(`${state} App Server status did not render its exact neutral projection.`);
  }
}
const unknownDisplayState = {
  ...baseWebviewState,
  messages: [],
  transientApproval: null,
  transientUserInput: null,
  transientCancellation: null,
  appServerStatus: {
    state: "recovery_required",
    outcomeUnknown: true,
    terminalSummaryId: "transport_closed",
  },
};
renderWebviewState(unknownDisplayState);
renderWebviewState(JSON.parse(JSON.stringify(unknownDisplayState)));
if (!transcript.innerHTML.includes("Outcome unknown — recovery is required.")) {
  throw new Error("Unknown App Server outcome did not render its persistent recovery-required warning.");
}
for (const forbiddenClaim of ["ready", "idle", "completed", "cancelled", "recovered", "retry"]) {
  if (transcript.innerHTML.toLowerCase().includes(forbiddenClaim)) {
    throw new Error(`Unknown-outcome warning made a forbidden lifecycle claim: ${forbiddenClaim}`);
  }
}
const send = getElement("send");
const chatProvider = getElement("chatProviderSelect");
const recoveryDisplayBefore = JSON.stringify(unknownDisplayState.appServerStatus);
renderWebviewState({ ...unknownDisplayState, chatProvider: "codex", chatModel: "config", isBusy: false });
prompt.value = "another ordinary turn";
for (const handler of prompt.listeners.get("input") || []) handler({});
if (!send.disabled || prompt.disabled) {
  throw new Error("Recovery-required Codex state did not disable only the existing Send action for ordinary text.");
}
const askCountBeforeBlockedKey = posted.filter((item) => item && item.type === "ask").length;
for (const handler of promptKeydown) {
  handler({
    key: "Enter",
    isComposing: false,
    altKey: false,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    preventDefault() {},
    stopPropagation() {},
  });
}
if (
  posted.filter((item) => item && item.type === "ask").length !== askCountBeforeBlockedKey
  || prompt.value !== "another ordinary turn"
  || JSON.stringify(unknownDisplayState.appServerStatus) !== recoveryDisplayBefore
) {
  throw new Error("A disabled recovery-required Send path posted an ask, cleared the draft, or changed stored status state.");
}

prompt.value = "/status";
for (const handler of prompt.listeners.get("input") || []) handler({});
if (send.disabled) {
  throw new Error("Changing a recovery-required Codex draft to a slash command did not re-enable Send.");
}
prompt.value = "ordinary text";
chatProvider.value = "ollama";
for (const handler of chatProvider.listeners.get("change") || []) handler({});
if (send.disabled || JSON.stringify(unknownDisplayState.appServerStatus) !== recoveryDisplayBefore) {
  throw new Error("Choosing Ollama did not re-enable Send without changing recovery-required status state.");
}
chatProvider.value = "claude";
for (const handler of chatProvider.listeners.get("change") || []) handler({});
if (send.disabled || JSON.stringify(unknownDisplayState.appServerStatus) !== recoveryDisplayBefore) {
  throw new Error("Choosing Claude did not re-enable Send without changing recovery-required status state.");
}

for (const appServerStatus of [
  completedDisplay,
  { state: "failed", outcomeUnknown: false, terminalSummaryId: "turn_failed" },
  { state: "cancelled", outcomeUnknown: false, terminalSummaryId: "cancelled" },
  undefined,
  null,
  "recovery_required",
  {},
  { ...completedDisplay, state: "unknown" },
  { ...unknownDisplayState.appServerStatus, extra: "extended" },
  { ...unknownDisplayState.appServerStatus, terminalSummaryId: "a".repeat(129) },
  { ...unknownDisplayState.appServerStatus, modelRef: 42 },
]) {
  prompt.value = "ordinary text";
  renderWebviewState({ ...baseWebviewState, chatProvider: "codex", chatModel: "config", isBusy: false, appServerStatus });
  if (send.disabled) {
    throw new Error(`Non-guard App Server display projection disabled Send: ${JSON.stringify(appServerStatus)}`);
  }
}
for (const invalidDisplay of [
  null,
  {},
  { ...completedDisplay, state: "unknown" },
  { ...completedDisplay, extra: "extended" },
  { ...completedDisplay, adapter: "codex_app_server" },
  { ...completedDisplay, terminalSummaryId: "cancelled" },
  { ...completedDisplay, terminalSummaryId: "a".repeat(129) },
  { ...completedDisplay, modelRef: 42 },
  { ...completedDisplay, rawMetadata: "raw-metadata-secret" },
]) {
  renderWebviewState({
    ...baseWebviewState,
    messages: [],
    transientApproval: null,
    transientUserInput: null,
    transientCancellation: null,
    appServerStatus: invalidDisplay,
  });
  if (transcript.innerHTML.includes("Codex App Server status")) {
    throw new Error(`Invalid App Server webview projection rendered: ${JSON.stringify(invalidDisplay)}`);
  }
}
for (const [state, expectedCopy] of [
  ["submitting", "Delivering cancellation intent…"],
  ["delivered", "Cancellation intent delivered; waiting for Codex."],
  ["transport_error", "Cancellation delivery is uncertain. No retry will be attempted."],
]) {
  for (const handler of messageHandlers) {
    handler({
      data: {
        type: "state",
        state: {
          ...baseWebviewState,
          transientApproval: null,
          transientCancellation: { uiReference: `cancellation-${state}`, state },
        },
      },
    });
  }
  if (!transcript.innerHTML.includes(expectedCopy) || transcript.innerHTML.includes(">Cancel</button>")) {
    throw new Error(`Cancellation ${state} projection rendered the wrong copy or left the control enabled.`);
  }
}
for (const forbiddenClaim of ["turn cancelled", "cancellation succeeded", "controller accepted", "interrupt delivered", "turn completed", "ready", "idle", "recovered"]) {
  if (transcript.innerHTML.toLowerCase().includes(forbiddenClaim)) {
    throw new Error(`Cancellation delivery copy made a terminal claim: ${forbiddenClaim}`);
  }
}
for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...baseWebviewState,
        transientApproval: null,
        transientCancellation: { uiReference: "extended-cancellation", state: "pending", reason: "user_requested" },
      },
    },
  });
}
if (transcript.innerHTML.includes("Cancel this Codex turn")) {
  throw new Error("Extended cancellation projection was rendered by the webview.");
}
const hostileSummary = '<img src=x onerror="prompt-secret"> Choose one & only one.';
for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...baseWebviewState,
        transientApproval: null,
        transientUserInput: {
          uiReference: "user-input-select-one",
          kind: "select_one",
          summary: hostileSummary,
          options: [
            { uiOptionReference: "ui-option-a", label: '<script>opaque-option-a</script>' },
            { uiOptionReference: "ui-option-b", label: "Second & safe" },
          ],
          state: "pending",
        },
      },
    },
  });
}
if (!transcript.innerHTML.includes("&lt;img src=x onerror=&quot;prompt-secret&quot;&gt;") || transcript.innerHTML.includes("<script>")) {
  throw new Error("User-input summary or option labels were not escaped safely.");
}
if (transcript.innerHTML.includes(" checked") || !transcript.innerHTML.includes("Choose exactly one option.")) {
  throw new Error("Single-choice prompt rendered an automatic/default answer or the wrong rule.");
}
for (const forbidden of ["process_incarnation_id", "prompt_ref", "option_ref", "opaque-provider-option", "provider_payload"]) {
  if (transcript.innerHTML.includes(forbidden)) {
    throw new Error(`User-input DOM leaked host-only content: ${forbidden}`);
  }
}

const selectOneCard = new FakeElement("user-input-select-one-card");
const selectOneSubmit = new FakeElement("user-input-select-one-submit");
const selectOneOption = new FakeElement("user-input-select-one-option");
selectOneSubmit.setAttribute("data-user-input-reference", "user-input-select-one");
selectOneOption.setAttribute("data-user-input-reference", "user-input-select-one");
selectOneOption.value = "ui-option-b";
selectOneOption.checked = true;
selectOneCard.queryResults.set("input[data-user-input-option]", [selectOneOption]);
selectOneCard.queryResults.set("button, input, textarea", [selectOneSubmit, selectOneOption]);
selectOneSubmit.closest = (selector) => selector === "[data-user-input-submit]" ? selectOneSubmit : (selector === "[data-user-input-card]" ? selectOneCard : null);
for (const handler of transcript.listeners.get("click") || []) {
  handler({ target: selectOneSubmit });
  handler({ target: selectOneSubmit });
}
const selectOneMessages = posted.filter((item) => item && item.type === "userInputResponse" && item.uiReference === "user-input-select-one");
if (selectOneMessages.length !== 1 || JSON.stringify(selectOneMessages[0]) !== JSON.stringify({
  type: "userInputResponse",
  uiReference: "user-input-select-one",
  responseKind: "select_one",
  uiOptionReferences: ["ui-option-b"],
})) {
  throw new Error(`Single-choice submission was not one exact safe message: ${JSON.stringify(selectOneMessages)}`);
}
if (!selectOneSubmit.disabled || !selectOneOption.disabled) {
  throw new Error("Single-choice submit did not disable every control immediately.");
}

for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...baseWebviewState,
        transientApproval: null,
        transientUserInput: {
          uiReference: "user-input-select-many",
          kind: "select_many",
          summary: "Choose up to two.",
          options: [
            { uiOptionReference: "ui-many-a", label: "A" },
            { uiOptionReference: "ui-many-b", label: "B" },
            { uiOptionReference: "ui-many-c", label: "C" },
          ],
          maxSelections: 2,
          state: "pending",
        },
      },
    },
  });
}
const selectManyCard = new FakeElement("user-input-select-many-card");
const selectManySubmit = new FakeElement("user-input-select-many-submit");
const manyA = new FakeElement("user-input-many-a");
const manyB = new FakeElement("user-input-many-b");
const manyC = new FakeElement("user-input-many-c");
for (const [element, value, checked] of [[manyA, "ui-many-a", true], [manyB, "ui-many-b", true], [manyC, "ui-many-c", false]]) {
  element.setAttribute("data-user-input-reference", "user-input-select-many");
  element.value = value;
  element.checked = checked;
  element.closest = (selector) => selector === "[data-user-input-option]" ? element : (selector === "[data-user-input-card]" ? selectManyCard : null);
}
selectManySubmit.setAttribute("data-user-input-reference", "user-input-select-many");
selectManySubmit.closest = (selector) => selector === "[data-user-input-submit]" ? selectManySubmit : (selector === "[data-user-input-card]" ? selectManyCard : null);
selectManyCard.queryResults.set("input[data-user-input-option]", [manyA, manyB, manyC]);
selectManyCard.queryResults.set("button[data-user-input-submit]", [selectManySubmit]);
selectManyCard.queryResults.set("button, input, textarea", [selectManySubmit, manyA, manyB, manyC]);
for (const handler of transcript.listeners.get("change") || []) {
  handler({ target: manyB });
}
if (!manyC.disabled || selectManySubmit.disabled) {
  throw new Error("Multiple-choice maximum did not disable additional selections while keeping the exact submit valid.");
}
for (const handler of transcript.listeners.get("click") || []) {
  handler({ target: selectManySubmit });
}
const selectManyMessage = posted.find((item) => item && item.type === "userInputResponse" && item.uiReference === "user-input-select-many");
if (!selectManyMessage || JSON.stringify(selectManyMessage.uiOptionReferences) !== JSON.stringify(["ui-many-a", "ui-many-b"])) {
  throw new Error(`Multiple-choice submission did not preserve exact distinct UI option references: ${JSON.stringify(selectManyMessage)}`);
}

for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...baseWebviewState,
        transientApproval: null,
        transientUserInput: {
          uiReference: "user-input-text",
          kind: "text",
          summary: "Enter exact text.",
          options: [],
          maxTextBytes: 12,
          state: "pending",
        },
      },
    },
  });
}
const textCard = new FakeElement("user-input-text-card");
const textSubmit = new FakeElement("user-input-text-submit");
const textArea = new FakeElement("user-input-text-area");
const byteCounter = new FakeElement("user-input-byte-counter");
const exactText = "  café  ";
textArea.value = exactText;
textArea.setAttribute("data-user-input-reference", "user-input-text");
textArea.closest = (selector) => selector === "[data-user-input-text]" ? textArea : (selector === "[data-user-input-card]" ? textCard : null);
textSubmit.setAttribute("data-user-input-reference", "user-input-text");
textSubmit.closest = (selector) => selector === "[data-user-input-submit]" ? textSubmit : (selector === "[data-user-input-card]" ? textCard : null);
textCard.queryResults.set("[data-user-input-byte-count]", [byteCounter]);
textCard.queryResults.set("button[data-user-input-submit]", [textSubmit]);
textCard.queryResults.set("[data-user-input-text]", [textArea]);
textCard.queryResults.set("button, input, textarea", [textSubmit, textArea]);
for (const handler of transcript.listeners.get("input") || []) {
  handler({ target: textArea });
}
if (byteCounter.textContent !== "9" || textSubmit.disabled) {
  throw new Error(`UTF-8 byte counter or exact text validation failed: count=${byteCounter.textContent}`);
}
for (const handler of transcript.listeners.get("click") || []) {
  handler({ target: textSubmit });
}
const textMessage = posted.find((item) => item && item.type === "userInputResponse" && item.uiReference === "user-input-text");
if (!textMessage || textMessage.text !== exactText || textArea.value !== "" || !textArea.disabled || !textSubmit.disabled) {
  throw new Error(`Text response was changed, retained in the DOM, or left enabled: ${JSON.stringify(textMessage)}`);
}
for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...baseWebviewState,
        transientApproval: null,
        transientUserInput: {
          uiReference: "user-input-text",
          state: "delivered",
        },
      },
    },
  });
}
if (!transcript.innerHTML.includes("Response delivered; waiting for Codex.") || transcript.innerHTML.includes(exactText) || transcript.innerHTML.includes("textarea")) {
  throw new Error("Delivered text response remained rendered or used completion semantics.");
}

for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...JSON.parse(getElement("pipeon-initial-state").textContent),
        transientApproval: {
          uiReference: "approval-ui-deny-only",
          reason: "declared_permission",
          allowedDecisions: ["deny"],
          state: "pending",
        },
      },
    },
  });
}
if (!transcript.innerHTML.includes("Codex requests an additional declared permission.") || transcript.innerHTML.includes(">Approve</button>") || !transcript.innerHTML.includes(">Deny</button>")) {
  throw new Error("Declared-permission request did not render as deny-only.");
}
if ((transcript.innerHTML.match(/data-approval-card=/g) || []).length !== 1) {
  throw new Error("Repeated approval rendering produced duplicate cards.");
}
for (const handler of messageHandlers) {
  handler({
    data: {
      type: "state",
      state: {
        ...JSON.parse(getElement("pipeon-initial-state").textContent),
        messages: [{ id: "denied", role: "assistant", html: "<p>approval_denied</p>" }],
        transientApproval: null,
        isBusy: false,
      },
    },
  });
}
if (!transcript.innerHTML.includes("approval_denied") || transcript.innerHTML.toLowerCase().includes("success")) {
  throw new Error("Denied approval did not render the original fail-closed result.");
}
for (const state of savedViewStates) {
  const serialized = JSON.stringify(state);
  if (
    serialized.includes("approval-ui")
    || serialized.includes("transientApproval")
    || serialized.includes("user-input-")
    || serialized.includes("transientUserInput")
    || serialized.includes("cancellation-ui")
    || serialized.includes("transientCancellation")
    || serialized.includes(exactText)
    || serialized.includes("correlation")
  ) {
    throw new Error(`Webview persistence captured transient control state: ${serialized}`);
  }
}

async function runApprovalHostFixtures() {
  const Module = require("module");
  const originalLoad = Module._load;
  Module._load = function(request, parent, isMain) {
    if (request === "vscode") {
      return {};
    }
    return originalLoad.call(this, request, parent, isMain);
  };
  let approvalTest;
  try {
    approvalTest = require(path.resolve(__dirname, "..", "extension.js")).__approvalTest;
  } finally {
    Module._load = originalLoad;
  }
  if (!approvalTest) {
    throw new Error("Extension approval fixture seam is unavailable.");
  }

  const correlation = {
    process_incarnation_id: "corr-process-secret",
    connection_id: "corr-connection-secret",
    session_id: "corr-session-secret",
    interaction_id: "corr-interaction-secret",
    activity_id: "corr-activity-secret",
    request_id: "corr-request-secret",
    decision_id: "corr-decision-secret",
  };
  const request = {
    correlation,
    reason: "command_execution",
    allowed_decisions: ["approve", "deny"],
  };
  let providerContentRejected = false;
  try {
    approvalTest.normalizeNeutralApprovalRequest({ ...request, provider_payload: "must-not-cross" });
  } catch {
    providerContentRejected = true;
  }
  if (!providerContentRejected) {
    throw new Error("Provider content crossed the closed neutral approval request boundary.");
  }
  let referenceCount = 0;
  const registry = new approvalTest.TransientApprovalRegistry(() => referenceCount++ === 0 ? "host-ui-reference" : "host-ui-new-reference");
  const invocation = registry.begin("pipeon-session-a");
  registry.retain(invocation.id, request);
  const projection = registry.project("pipeon-session-a");
  const serializedProjection = JSON.stringify(projection);
  if (!projection || serializedProjection.includes("corr-") || serializedProjection.includes("correlation") || serializedProjection.includes("provider")) {
    throw new Error(`Host projection leaked correlation or provider content: ${serializedProjection}`);
  }
  if (registry.prepareDecision("pipeon-session-b", "host-ui-reference", "approve") !== null) {
    throw new Error("Cross-session approval was not rejected before MCP.");
  }
  if (registry.prepareDecision("pipeon-session-a", "substituted-reference", "approve") !== null) {
    throw new Error("Substituted approval reference was not rejected before MCP.");
  }
  if (registry.prepareDecision("pipeon-session-a", "host-ui-reference", "remember") !== null) {
    throw new Error("Disallowed approval decision was not rejected before MCP.");
  }

  let readCount = 0;
  let foundCount = 0;
  let impliedDecisionCount = 0;
  const monitorStatus = await approvalTest.monitorProviderPoolApproval({
    isActive: () => true,
    cadenceMs: 0,
    lifetimeMs: 1000,
    readRequest: async () => {
      readCount += 1;
      if (readCount < 3) {
        throw new Error("MCP RPC -32000: no exact provider-pool approval request is pending");
      }
      return request;
    },
    onRequest: () => { foundCount += 1; },
    onTransportFailure: () => { throw new Error("Expected pre-request miss was treated as a transport failure."); },
  });
  if (monitorStatus !== "found" || readCount !== 3 || foundCount !== 1 || impliedDecisionCount !== 0) {
    throw new Error("Approval monitor did not perform bounded, non-consuming reads exactly once.");
  }

  let decideCalls = 0;
  const delivered = await approvalTest.deliverTransientApprovalDecision(
    registry,
    "pipeon-session-a",
    "host-ui-reference",
    "approve",
    async (_toolName, payload) => {
      decideCalls += 1;
      if (JSON.stringify(payload.correlation) !== JSON.stringify(correlation) || payload.decision !== "approve") {
        throw new Error("Host did not submit the unchanged retained correlation and selected decision.");
      }
      return { delivered: true };
    }
  );
  if (delivered !== "delivered" || decideCalls !== 1 || registry.project("pipeon-session-a")?.state !== "delivered") {
    throw new Error("One approval did not map to one exact MCP decision while leaving the invocation active.");
  }
  const duplicate = await approvalTest.deliverTransientApprovalDecision(
    registry,
    "pipeon-session-a",
    "host-ui-reference",
    "approve",
    async () => { decideCalls += 1; return { delivered: true }; }
  );
  if (duplicate !== "rejected" || decideCalls !== 1) {
    throw new Error("Duplicate approval reached MCP.");
  }
  registry.end(invocation.id);
  if (registry.project("pipeon-session-a") !== null) {
    throw new Error("Completed invocation retained an approval request.");
  }
  if (registry.prepareDecision("pipeon-session-a", "host-ui-reference", "approve") !== null) {
    throw new Error("Stale approval reference was not rejected before MCP.");
  }
  const nextInvocation = registry.begin("pipeon-session-a");
  registry.retain(nextInvocation.id, request);
  if (registry.prepareDecision("pipeon-session-a", "host-ui-reference", "approve") !== null) {
    throw new Error("Cross-chat approval reference was retargeted to a later invocation.");
  }
  registry.end(nextInvocation.id);
  const reloadedRegistry = new approvalTest.TransientApprovalRegistry(() => "unused");
  if (reloadedRegistry.project("pipeon-session-a") !== null) {
    throw new Error("Extension reload restored transient approval state.");
  }

  const errorRegistry = new approvalTest.TransientApprovalRegistry(() => "error-ui-reference");
  const errorInvocation = errorRegistry.begin("pipeon-session-error");
  errorRegistry.retain(errorInvocation.id, request);
  let ambiguousCalls = 0;
  const ambiguous = await approvalTest.deliverTransientApprovalDecision(
    errorRegistry,
    "pipeon-session-error",
    "error-ui-reference",
    "deny",
    async () => { ambiguousCalls += 1; throw new Error("ambiguous transport failure"); }
  );
  const ambiguousRetry = await approvalTest.deliverTransientApprovalDecision(
    errorRegistry,
    "pipeon-session-error",
    "error-ui-reference",
    "approve",
    async () => { ambiguousCalls += 1; return { delivered: true }; }
  );
  if (ambiguous !== "transport_error" || ambiguousRetry !== "rejected" || ambiguousCalls !== 1 || errorRegistry.project("pipeon-session-error")?.state !== "transport_error") {
    throw new Error("Ambiguous decision failure retried, fell back, or re-enabled the request.");
  }

  const denyRegistry = new approvalTest.TransientApprovalRegistry(() => "deny-ui-reference");
  const denyInvocation = denyRegistry.begin("pipeon-session-deny");
  denyRegistry.retain(denyInvocation.id, {
    correlation,
    reason: "declared_permission",
    allowed_decisions: ["deny"],
  });
  if (denyRegistry.prepareDecision("pipeon-session-deny", "deny-ui-reference", "approve") !== null) {
    throw new Error("Deny-only request accepted approval before MCP.");
  }

  const missingRegistry = new approvalTest.TransientApprovalRegistry(() => "missing-ui-reference");
  const missingInvocation = missingRegistry.begin("pipeon-session-missing");
  missingRegistry.retain(missingInvocation.id, request);
  if (missingRegistry.project("pipeon-session-missing")?.state !== "pending") {
    throw new Error("Missing decision did not leave the original chat pending.");
  }

  const compact = approvalTest.compactChatStoreForPersistence({
    activeSessionId: "pipeon-session-a",
    sessions: [{ id: "pipeon-session-a", title: "Chat", createdAt: "2026-08-03T00:00:00.000Z", updatedAt: "2026-08-03T00:00:00.000Z", codexSessionAdapter: "codex_app_server", messages: [] }],
    transientApproval: { request, uiReference: "must-not-persist" },
  });
  const serializedCompact = JSON.stringify(compact);
  if (serializedCompact.includes("transientApproval") || serializedCompact.includes("must-not-persist") || serializedCompact.includes("corr-")) {
    throw new Error(`Persisted chat state captured transient approval data: ${serializedCompact}`);
  }
}

async function runUserInputHostFixtures() {
  const Module = require("module");
  const originalLoad = Module._load;
  Module._load = function(request, parent, isMain) {
    if (request === "vscode") {
      return {};
    }
    return originalLoad.call(this, request, parent, isMain);
  };
  let userInputTest;
  try {
    userInputTest = require(path.resolve(__dirname, "..", "extension.js")).__userInputTest;
  } finally {
    Module._load = originalLoad;
  }
  if (!userInputTest) {
    throw new Error("Extension user-input fixture seam is unavailable.");
  }

  const correlation = {
    process_incarnation_id: "input-process-secret",
    connection_id: "input-connection-secret",
    session_id: "input-session-secret",
    interaction_id: "input-interaction-secret",
    activity_id: "input-activity-secret",
    request_id: "input-request-secret",
    decision_id: "input-decision-secret",
  };
  const promptFor = (kind) => kind === "text" ? {
    correlation: { ...correlation },
    prompt_ref: "opaque-prompt-text",
    kind,
    summary: "Enter exact text.",
    max_text_bytes: 12,
  } : {
    correlation: { ...correlation },
    prompt_ref: `opaque-prompt-${kind}`,
    kind,
    summary: kind === "select_one" ? "Choose one." : "Choose several.",
    options: [
      { option_ref: "opaque-option-a", label: "Alpha" },
      { option_ref: "opaque-option-b", label: "Beta" },
      { option_ref: "opaque-option-c", label: "Gamma" },
    ],
    max_selections: kind === "select_one" ? 1 : 2,
  };
  const expectRejected = (name, fn) => {
    let rejected = false;
    try {
      fn();
    } catch {
      rejected = true;
    }
    if (!rejected) {
      throw new Error(`Invalid user-input fixture was accepted: ${name}`);
    }
  };

  for (const kind of ["select_one", "select_many", "text"]) {
    userInputTest.normalizeNeutralUserInputPrompt(promptFor(kind));
  }
  const invalidPrompts = [
    ["unknown key", { ...promptFor("text"), provider_payload: "forbidden" }],
    ["missing key", (() => { const value = promptFor("text"); delete value.summary; return value; })()],
    ["incomplete correlation", { ...promptFor("text"), correlation: { ...correlation, request_id: "" } }],
    ["unknown correlation key", { ...promptFor("text"), correlation: { ...correlation, provider_id: "forbidden" } }],
    ["invalid kind", { ...promptFor("text"), kind: "confirm" }],
    ["irrelevant text options", { ...promptFor("text"), options: [] }],
    ["irrelevant selection text bound", { ...promptFor("select_one"), max_text_bytes: 0 }],
    ["duplicate option refs", (() => { const value = promptFor("select_many"); value.options[1].option_ref = value.options[0].option_ref; return value; })()],
    ["empty options", { ...promptFor("select_one"), options: [] }],
    ["too many options", { ...promptFor("select_many"), options: Array.from({ length: 17 }, (_, index) => ({ option_ref: `option-${index}`, label: `Option ${index}` })) }],
    ["single selection bound", { ...promptFor("select_one"), max_selections: 2 }],
    ["multiple selection bound", { ...promptFor("select_many"), max_selections: 4 }],
    ["text bound zero", { ...promptFor("text"), max_text_bytes: 0 }],
    ["text bound excessive", { ...promptFor("text"), max_text_bytes: 4097 }],
    ["summary byte overflow", { ...promptFor("text"), summary: "é".repeat(256) + "a" }],
    ["label byte overflow", (() => { const value = promptFor("select_one"); value.options[0].label = "é".repeat(64) + "a"; return value; })()],
    ["summary control", { ...promptFor("text"), summary: "bad\nsummary" }],
    ["label NUL", (() => { const value = promptFor("select_one"); value.options[0].label = "bad\0label"; return value; })()],
    ["summary surrogate", { ...promptFor("text"), summary: "bad\ud800summary" }],
    ["label surrogate", (() => { const value = promptFor("select_one"); value.options[0].label = "bad\udc00label"; return value; })()],
    ["opaque ref whitespace", { ...promptFor("text"), prompt_ref: "opaque prompt" }],
  ];
  for (const [name, value] of invalidPrompts) {
    expectRejected(name, () => userInputTest.normalizeNeutralUserInputPrompt(value));
  }
  userInputTest.normalizeNeutralUserInputPrompt({ ...promptFor("text"), summary: "é".repeat(256) });
  userInputTest.normalizeNeutralUserInputPrompt({ ...promptFor("text"), max_text_bytes: 4096 });

  const optionReferences = ["ui-option-a", "ui-option-b", "ui-option-c"];
  let optionIndex = 0;
  const registry = new userInputTest.TransientUserInputRegistry(
    () => "user-input-ui",
    () => optionReferences[optionIndex++]
  );
  const invocation = registry.begin("pipeon-session-a");
  const sourcePrompt = promptFor("select_many");
  registry.retain(invocation.id, sourcePrompt);
  sourcePrompt.correlation.request_id = "mutated-request";
  sourcePrompt.prompt_ref = "mutated-prompt";
  sourcePrompt.options[0].option_ref = "mutated-option";
  sourcePrompt.options[0].label = "Mutated label";
  const projection = registry.project("pipeon-session-a");
  const serializedProjection = JSON.stringify(projection);
  if (
    !projection
    || projection.options[0].label !== "Alpha"
    || serializedProjection.includes("input-process-secret")
    || serializedProjection.includes("input-request-secret")
    || serializedProjection.includes("prompt_ref")
    || serializedProjection.includes("opaque-option")
    || serializedProjection.includes("option_ref")
    || serializedProjection.includes("correlation")
  ) {
    throw new Error(`User-input projection leaked host-only data or was not defensive: ${serializedProjection}`);
  }
  if (new Set(projection.options.map((option) => option.uiOptionReference)).size !== 3) {
    throw new Error("Selectable options did not receive distinct random UI references.");
  }
  if (registry.prepareResponse("pipeon-session-b", "user-input-ui", "select_many", { uiOptionReferences: ["ui-option-a"] }) !== null) {
    throw new Error("Cross-session user-input response was not rejected before MCP.");
  }
  if (registry.prepareResponse("pipeon-session-a", "substituted-ui", "select_many", { uiOptionReferences: ["ui-option-a"] }) !== null) {
    throw new Error("Cross-UI-reference response was not rejected before MCP.");
  }
  for (const [name, kind, selected] of [
    ["wrong kind", "select_one", ["ui-option-a"]],
    ["unknown option", "select_many", ["unknown-ui-option"]],
    ["duplicate option", "select_many", ["ui-option-a", "ui-option-a"]],
    ["excessive options", "select_many", ["ui-option-a", "ui-option-b", "ui-option-c"]],
  ]) {
    if (registry.prepareResponse("pipeon-session-a", "user-input-ui", kind, { uiOptionReferences: selected }) !== null) {
      throw new Error(`${name} reached MCP.`);
    }
  }
  const mapped = registry.prepareResponse("pipeon-session-a", "user-input-ui", "select_many", { uiOptionReferences: ["ui-option-c", "ui-option-a"] });
  if (!mapped || JSON.stringify(mapped.selected_option_refs) !== JSON.stringify(["opaque-option-c", "opaque-option-a"])) {
    throw new Error(`Random UI option references did not map to exact retained opaque refs: ${JSON.stringify(mapped)}`);
  }
  if (registry.prepareResponse("pipeon-session-a", "user-input-ui", "select_many", { uiOptionReferences: ["ui-option-a"] }) !== null) {
    throw new Error("Duplicate submission was not rejected before MCP.");
  }
  registry.end(invocation.id);
  if (registry.project("pipeon-session-a") !== null) {
    throw new Error("Completed invocation retained user-input state.");
  }

  const secondRegistry = new userInputTest.TransientUserInputRegistry(() => "second-ui", () => "second-option-ui");
  const firstInvocation = secondRegistry.begin("pipeon-session-second");
  secondRegistry.retain(firstInvocation.id, promptFor("text"));
  const competingInvocation = secondRegistry.begin("pipeon-session-second");
  expectRejected("second concurrent prompt", () => secondRegistry.retain(competingInvocation.id, promptFor("text")));
  secondRegistry.end(competingInvocation.id);
  secondRegistry.end(firstInvocation.id);

  const staleRegistry = new userInputTest.TransientUserInputRegistry(
    (() => { let count = 0; return () => `stale-ui-${++count}`; })(),
    () => "stale-option-ui"
  );
  const staleInvocation = staleRegistry.begin("pipeon-session-stale");
  staleRegistry.retain(staleInvocation.id, promptFor("text"));
  staleRegistry.end(staleInvocation.id);
  const nextInvocation = staleRegistry.begin("pipeon-session-stale");
  staleRegistry.retain(nextInvocation.id, promptFor("text"));
  if (staleRegistry.prepareResponse("pipeon-session-stale", "stale-ui-1", "text", { text: "safe" }) !== null) {
    throw new Error("Stale cross-chat user-input reference was retargeted.");
  }
  staleRegistry.invocations.delete(nextInvocation.id);
  if (staleRegistry.prepareResponse("pipeon-session-stale", "stale-ui-2", "text", { text: "safe" }) !== null) {
    throw new Error("Cross-invocation user-input response was accepted without its live invocation.");
  }

  const responseCases = [
    ["blank", "   "],
    ["NUL", "bad\0text"],
    ["control", "bad\ntext"],
    ["surrogate", "bad\ud800text"],
    ["byte overflow", "é".repeat(7)],
  ];
  for (const [name, text] of responseCases) {
    const textRegistry = new userInputTest.TransientUserInputRegistry(() => `text-ui-${name}`, () => "unused-option-ui");
    const textInvocation = textRegistry.begin(`pipeon-text-${name}`);
    textRegistry.retain(textInvocation.id, promptFor("text"));
    if (textRegistry.prepareResponse(`pipeon-text-${name}`, `text-ui-${name}`, "text", { text }) !== null) {
      throw new Error(`Invalid text response reached MCP: ${name}`);
    }
    textRegistry.end(textInvocation.id);
  }

  const deliveryRegistry = new userInputTest.TransientUserInputRegistry(() => "delivery-ui", () => "unused-option-ui");
  const deliveryInvocation = deliveryRegistry.begin("pipeon-delivery");
  deliveryRegistry.retain(deliveryInvocation.id, promptFor("text"));
  let abortCount = 0;
  deliveryRegistry.bindChatAbort(deliveryInvocation.id, () => { abortCount += 1; });
  let deliveryCalls = 0;
  let deliveredPayload = null;
  let chatSettled = false;
  const pendingChat = new Promise(() => {}).finally(() => { chatSettled = true; });
  const delivered = await userInputTest.deliverTransientUserInputResponse(
    deliveryRegistry,
    "pipeon-delivery",
    "delivery-ui",
    "text",
    { text: "  café  " },
    async (toolName, payload) => {
      deliveryCalls += 1;
      deliveredPayload = JSON.parse(JSON.stringify(payload));
      if (toolName !== "dorkpipe.provider_pool_user_input_respond") {
        throw new Error(`Wrong user-input MCP tool: ${toolName}`);
      }
      return { delivered: true };
    }
  );
  void pendingChat;
  const deliveredProjection = deliveryRegistry.project("pipeon-delivery");
  const retainedAfterDelivery = JSON.stringify(deliveredProjection);
  if (
    delivered !== "delivered"
    || deliveryCalls !== 1
    || chatSettled
    || abortCount !== 0
    || deliveredPayload.text !== "  café  "
    || deliveredPayload.prompt_ref !== "opaque-prompt-text"
    || retainedAfterDelivery.includes("café")
    || JSON.stringify(Object.keys(deliveredProjection).sort()) !== JSON.stringify(["state", "uiReference"])
    || deliveryRegistry.project("pipeon-delivery")?.state !== "delivered"
  ) {
    throw new Error("Exact user-input delivery did not remain one-shot, transient, and independent from chat completion.");
  }
  const duplicateDelivery = await userInputTest.deliverTransientUserInputResponse(
    deliveryRegistry,
    "pipeon-delivery",
    "delivery-ui",
    "text",
    { text: "other" },
    async () => { deliveryCalls += 1; return { delivered: true }; }
  );
  if (duplicateDelivery !== "rejected" || deliveryCalls !== 1) {
    throw new Error("Duplicate delivered response reached MCP.");
  }

  const ambiguousRegistry = new userInputTest.TransientUserInputRegistry(() => "ambiguous-ui", () => "unused-option-ui");
  const ambiguousInvocation = ambiguousRegistry.begin("pipeon-ambiguous");
  ambiguousRegistry.retain(ambiguousInvocation.id, promptFor("text"));
  let ambiguousAbortCount = 0;
  let ambiguousCalls = 0;
  ambiguousRegistry.bindChatAbort(ambiguousInvocation.id, () => { ambiguousAbortCount += 1; });
  const ambiguous = await userInputTest.deliverTransientUserInputResponse(
    ambiguousRegistry,
    "pipeon-ambiguous",
    "ambiguous-ui",
    "text",
    { text: "safe" },
    async () => { ambiguousCalls += 1; throw new Error("ambiguous transport"); }
  );
  const ambiguousRetry = await userInputTest.deliverTransientUserInputResponse(
    ambiguousRegistry,
    "pipeon-ambiguous",
    "ambiguous-ui",
    "text",
    { text: "fallback" },
    async () => { ambiguousCalls += 1; return { delivered: true }; }
  );
  if (
    ambiguous !== "transport_error"
    || ambiguousRetry !== "rejected"
    || ambiguousCalls !== 1
    || ambiguousAbortCount !== 1
    || ambiguousRegistry.project("pipeon-ambiguous")?.state !== "transport_error"
  ) {
    throw new Error("Ambiguous user-input delivery retried, failed to abort, or re-enabled controls.");
  }

  let readCount = 0;
  let foundCount = 0;
  const monitorStatus = await userInputTest.monitorProviderPoolUserInput({
    isActive: () => true,
    cadenceMs: 0,
    lifetimeMs: 1000,
    readPrompt: async () => {
      readCount += 1;
      if (readCount === 1) {
        throw new Error("MCP RPC -32000: no exact provider-pool user-input prompt is pending");
      }
      if (readCount === 2) {
        throw new Error("MCP RPC -32000: no provider-pool chat is active");
      }
      return promptFor("select_one");
    },
    onPrompt: () => { foundCount += 1; },
    onTransportFailure: () => { throw new Error("Expected user-input monitor miss aborted the chat."); },
  });
  if (monitorStatus !== "found" || readCount !== 3 || foundCount !== 1) {
    throw new Error("User-input monitor did not perform expected non-consuming misses followed by one exact prompt.");
  }
  let failureReads = 0;
  let failureAborts = 0;
  const failureStatus = await userInputTest.monitorProviderPoolUserInput({
    isActive: () => true,
    cadenceMs: 0,
    lifetimeMs: 1000,
    readPrompt: async () => { failureReads += 1; throw new Error("authenticated MCP transport failed"); },
    onPrompt: () => { throw new Error("Transport failure produced a prompt."); },
    onTransportFailure: () => { failureAborts += 1; },
  });
  if (failureStatus !== "transport_error" || failureReads !== 1 || failureAborts !== 1) {
    throw new Error("User-input monitor transport failure retried or did not abort exactly once.");
  }

  const routeCases = [
    ["codex", "codex_app_server", "normal question", true],
    ["codex", "codex_app_server", "/codex explicit", false],
    ["codex", "codex_exec", "normal question", false],
    ["ollama", "codex_app_server", "normal question", false],
    ["claude", "codex_app_server", "normal question", false],
  ];
  for (const [provider, adapter, text, expected] of routeCases) {
    if (userInputTest.shouldStartTransientInteractiveControls(provider, adapter, text) !== expected) {
      throw new Error(`Transient user-input route gate was wrong for ${provider}/${adapter}/${text}.`);
    }
  }

  const cleanupRegistry = new userInputTest.TransientUserInputRegistry(
    (() => { let count = 0; return () => `cleanup-ui-${++count}`; })(),
    () => "cleanup-option-ui"
  );
  const cleanupOne = cleanupRegistry.begin("cleanup-one");
  cleanupRegistry.retain(cleanupOne.id, promptFor("text"));
  cleanupRegistry.bindChatAbort(cleanupOne.id, () => {});
  if (!cleanupRegistry.abortSession("cleanup-one") || cleanupRegistry.project("cleanup-one") !== null) {
    throw new Error("View disposal cleanup retained replayable user-input state.");
  }
  const cleanupTwo = cleanupRegistry.begin("cleanup-two");
  cleanupRegistry.retain(cleanupTwo.id, promptFor("text"));
  cleanupRegistry.bindChatAbort(cleanupTwo.id, () => {});
  cleanupRegistry.abortAll();
  if (cleanupRegistry.project("cleanup-two") !== null) {
    throw new Error("Reload/teardown cleanup retained replayable user-input state.");
  }
  const reloadedRegistry = new userInputTest.TransientUserInputRegistry(() => "unused", () => "unused-option");
  if (reloadedRegistry.project("pipeon-delivery") !== null) {
    throw new Error("Extension reload reconstructed transient user-input state.");
  }

  const compact = userInputTest.compactChatStoreForPersistence({
    activeSessionId: "pipeon-delivery",
    sessions: [{ id: "pipeon-delivery", title: "Chat", createdAt: "2026-08-03T00:00:00.000Z", updatedAt: "2026-08-03T00:00:00.000Z", codexSessionAdapter: "codex_app_server", messages: [] }],
    transientUserInput: { prompt: promptFor("text"), text: "must-not-persist", uiReference: "must-not-persist-ui" },
  });
  const serializedCompact = JSON.stringify(compact);
  if (serializedCompact.includes("transientUserInput") || serializedCompact.includes("must-not-persist") || serializedCompact.includes("input-process-secret")) {
    throw new Error(`Persisted chat state captured transient user-input data: ${serializedCompact}`);
  }
}

async function runCancellationHostFixtures() {
  const Module = require("module");
  const originalLoad = Module._load;
  Module._load = function(request, parent, isMain) {
    if (request === "vscode") {
      return {};
    }
    return originalLoad.call(this, request, parent, isMain);
  };
  let cancellationTest;
  try {
    cancellationTest = require(path.resolve(__dirname, "..", "extension.js")).__cancellationTest;
  } finally {
    Module._load = originalLoad;
  }
  if (!cancellationTest) {
    throw new Error("Extension cancellation fixture seam is unavailable.");
  }

  const scope = {
    session: { provider: "codex", session_id: "cancellation-session-secret" },
    correlation: {
      process_incarnation_id: "cancellation-process-secret",
      connection_id: "cancellation-connection-secret",
      session_id: "cancellation-session-secret",
      interaction_id: "cancellation-turn-secret",
      activity_id: "",
      request_id: "",
      decision_id: "",
    },
  };
  const clone = (value) => JSON.parse(JSON.stringify(value));
  const expectRejected = (name, fn) => {
    let rejected = false;
    try {
      fn();
    } catch {
      rejected = true;
    }
    if (!rejected) {
      throw new Error(`Invalid cancellation fixture was accepted: ${name}`);
    }
  };
  cancellationTest.normalizeNeutralCancellationScope(scope);
  for (const [name, value] of [
    ["extended scope", { ...clone(scope), reason: "user_requested" }],
    ["extended session", { ...clone(scope), session: { ...scope.session, adapter: "codex_app_server" } }],
    ["extended correlation", { ...clone(scope), correlation: { ...scope.correlation, provider_id: "forbidden" } }],
    ["wrong provider", { ...clone(scope), session: { ...scope.session, provider: "claude" } }],
    ["blank session", { ...clone(scope), session: { ...scope.session, session_id: "" } }],
    ["mismatched session", { ...clone(scope), correlation: { ...scope.correlation, session_id: "other-session" } }],
    ["blank process", { ...clone(scope), correlation: { ...scope.correlation, process_incarnation_id: "" } }],
    ["surrounding whitespace", { ...clone(scope), correlation: { ...scope.correlation, connection_id: " connection " } }],
    ["activity scope", { ...clone(scope), correlation: { ...scope.correlation, activity_id: "forbidden" } }],
    ["request scope", { ...clone(scope), correlation: { ...scope.correlation, request_id: "forbidden" } }],
    ["decision scope", { ...clone(scope), correlation: { ...scope.correlation, decision_id: "forbidden" } }],
    ["control identifier", { ...clone(scope), correlation: { ...scope.correlation, interaction_id: "bad\nturn" } }],
    ["invalid surrogate", { ...clone(scope), correlation: { ...scope.correlation, interaction_id: "bad\ud800turn" } }],
  ]) {
    expectRejected(name, () => cancellationTest.normalizeNeutralCancellationScope(value));
  }

  const randomRegistry = new cancellationTest.TransientCancellationRegistry();
  const randomOne = randomRegistry.begin("random-session-one", "random-chat-one");
  const randomTwo = randomRegistry.begin("random-session-two", "random-chat-two");
  randomRegistry.retain(randomOne.id, scope);
  randomRegistry.retain(randomTwo.id, scope);
  const randomReferences = [randomRegistry.project("random-session-one")?.uiReference, randomRegistry.project("random-session-two")?.uiReference];
  if (new Set(randomReferences).size !== 2 || randomReferences.some((reference) => !/^cancellation_ui_[A-Za-z0-9_-]{24}$/.test(String(reference || "")))) {
    throw new Error(`Cancellation UI references were not distinct cryptographic random references: ${JSON.stringify(randomReferences)}`);
  }
  randomRegistry.abortAll();

  const registry = new cancellationTest.TransientCancellationRegistry(() => "cancellation-host-ui");
  const invocation = registry.begin("pipeon-cancellation-session", "provider-pool-chat-one");
  const sourceScope = clone(scope);
  registry.retain(invocation.id, sourceScope);
  sourceScope.session.session_id = "mutated-session";
  sourceScope.correlation.connection_id = "mutated-connection";
  const projection = registry.project("pipeon-cancellation-session");
  const serializedProjection = JSON.stringify(projection);
  if (
    JSON.stringify(Object.keys(projection).sort()) !== JSON.stringify(["state", "uiReference"])
    || projection.state !== "pending"
    || serializedProjection.includes("cancellation-session-secret")
    || serializedProjection.includes("process")
    || serializedProjection.includes("user_requested")
    || serializedProjection.includes("scope")
    || serializedProjection.includes("intent")
  ) {
    throw new Error(`Cancellation projection leaked host-only data: ${serializedProjection}`);
  }
  if (registry.prepareIntent("other-session", "cancellation-host-ui") !== null) {
    throw new Error("Cross-session cancellation reached MCP.");
  }
  if (registry.prepareIntent("pipeon-cancellation-session", "substituted-ui") !== null) {
    throw new Error("Substituted cancellation UI reference reached MCP.");
  }
  registry.invocations.get(invocation.id).providerPoolChatId = "other-provider-pool-chat";
  if (registry.prepareIntent("pipeon-cancellation-session", "cancellation-host-ui") !== null) {
    throw new Error("Cross-chat cancellation reached MCP.");
  }
  registry.invocations.get(invocation.id).providerPoolChatId = "provider-pool-chat-one";

  const retainedControls = {
    approval: { uiReference: "approval-independent", state: "pending" },
    userInput: { uiReference: "input-independent", state: "pending" },
  };
  const retainedControlsJSON = JSON.stringify(retainedControls);
  let deliveryCalls = 0;
  let deliveredPayload = null;
  let chatSettled = false;
  const pendingChat = new Promise(() => {}).finally(() => { chatSettled = true; });
  const delivered = await cancellationTest.deliverTransientCancellationIntent(
    registry,
    "pipeon-cancellation-session",
    "cancellation-host-ui",
    async (toolName, payload) => {
      deliveryCalls += 1;
      deliveredPayload = clone(payload);
      if (toolName !== "dorkpipe.provider_pool_cancellation_deliver") {
        throw new Error(`Wrong cancellation MCP tool: ${toolName}`);
      }
      return { delivered: true };
    }
  );
  void pendingChat;
  if (
    delivered !== "delivered"
    || deliveryCalls !== 1
    || chatSettled
    || deliveredPayload.reason !== "user_requested"
    || JSON.stringify(deliveredPayload.session) !== JSON.stringify(scope.session)
    || JSON.stringify(deliveredPayload.correlation) !== JSON.stringify(scope.correlation)
    || registry.project("pipeon-cancellation-session")?.state !== "delivered"
    || JSON.stringify(retainedControls) !== retainedControlsJSON
  ) {
    throw new Error("Cancellation delivery was not exact, one-shot, non-terminal, and independent.");
  }
  const duplicate = await cancellationTest.deliverTransientCancellationIntent(
    registry,
    "pipeon-cancellation-session",
    "cancellation-host-ui",
    async () => { deliveryCalls += 1; return { delivered: true }; }
  );
  if (duplicate !== "rejected" || deliveryCalls !== 1) {
    throw new Error("Duplicate cancellation intent reached MCP.");
  }

  const ambiguousRegistry = new cancellationTest.TransientCancellationRegistry(() => "cancellation-ambiguous-ui");
  const ambiguousInvocation = ambiguousRegistry.begin("pipeon-cancellation-ambiguous", "provider-pool-chat-ambiguous");
  ambiguousRegistry.retain(ambiguousInvocation.id, scope);
  let ambiguousCalls = 0;
  const ambiguous = await cancellationTest.deliverTransientCancellationIntent(
    ambiguousRegistry,
    "pipeon-cancellation-ambiguous",
    "cancellation-ambiguous-ui",
    async () => { ambiguousCalls += 1; throw new Error("ambiguous cancellation delivery"); }
  );
  const ambiguousRetry = await cancellationTest.deliverTransientCancellationIntent(
    ambiguousRegistry,
    "pipeon-cancellation-ambiguous",
    "cancellation-ambiguous-ui",
    async () => { ambiguousCalls += 1; return { delivered: true }; }
  );
  if (
    ambiguous !== "transport_error"
    || ambiguousRetry !== "rejected"
    || ambiguousCalls !== 1
    || ambiguousRegistry.project("pipeon-cancellation-ambiguous")?.state !== "transport_error"
    || JSON.stringify(retainedControls) !== retainedControlsJSON
  ) {
    throw new Error("Ambiguous cancellation delivery retried, replayed, or changed another control.");
  }

  for (const [name, acknowledgement] of [
    ["missing", null],
    ["false", { delivered: false }],
    ["extended", { delivered: true, terminal: "cancelled" }],
  ]) {
    const acknowledgementRegistry = new cancellationTest.TransientCancellationRegistry(() => `cancellation-${name}-ack-ui`);
    const acknowledgementInvocation = acknowledgementRegistry.begin(`pipeon-${name}-ack`, `provider-pool-${name}-ack`);
    acknowledgementRegistry.retain(acknowledgementInvocation.id, scope);
    let acknowledgementCalls = 0;
    const status = await cancellationTest.deliverTransientCancellationIntent(
      acknowledgementRegistry,
      `pipeon-${name}-ack`,
      `cancellation-${name}-ack-ui`,
      async () => { acknowledgementCalls += 1; return acknowledgement; }
    );
    if (status !== "transport_error" || acknowledgementCalls !== 1 || acknowledgementRegistry.project(`pipeon-${name}-ack`)?.state !== "transport_error") {
      throw new Error(`Cancellation ${name} acknowledgement was not treated as permanently ambiguous.`);
    }
  }

  let readCount = 0;
  let foundCount = 0;
  const monitorStatus = await cancellationTest.monitorProviderPoolCancellation({
    isActive: () => true,
    cadenceMs: 0,
    lifetimeMs: 1000,
    readScope: async () => {
      readCount += 1;
      if (readCount === 1) {
        throw new Error("MCP RPC -32000: no exact provider-pool cancellation scope is pending");
      }
      if (readCount === 2) {
        throw new Error("MCP RPC -32000: no provider-pool chat is active");
      }
      return scope;
    },
    onScope: () => { foundCount += 1; },
    onTransportFailure: () => { throw new Error("Expected cancellation miss became a transport error."); },
  });
  if (monitorStatus !== "found" || readCount !== 3 || foundCount !== 1) {
    throw new Error("Cancellation monitor did not perform bounded non-consuming reads.");
  }

  const monitorErrorRegistry = new cancellationTest.TransientCancellationRegistry(() => "cancellation-monitor-error-ui");
  const monitorErrorInvocation = monitorErrorRegistry.begin("pipeon-monitor-error", "provider-pool-chat-monitor-error");
  let failureReads = 0;
  let failureMarks = 0;
  const failureStatus = await cancellationTest.monitorProviderPoolCancellation({
    isActive: () => monitorErrorRegistry.isActive(monitorErrorInvocation.id),
    cadenceMs: 0,
    lifetimeMs: 1000,
    readScope: async () => { failureReads += 1; throw new Error("authenticated MCP transport failed"); },
    onScope: () => { throw new Error("Cancellation transport failure produced a scope."); },
    onTransportFailure: () => {
      failureMarks += 1;
      monitorErrorRegistry.markMonitorTransportError(monitorErrorInvocation.id);
    },
  });
  if (
    failureStatus !== "transport_error"
    || failureReads !== 1
    || failureMarks !== 1
    || monitorErrorRegistry.project("pipeon-monitor-error")?.state !== "transport_error"
    || JSON.stringify(retainedControls) !== retainedControlsJSON
  ) {
    throw new Error("Cancellation monitor failure retried, inferred an intent, or changed another control.");
  }
  let nearMissFailures = 0;
  const nearMissStatus = await cancellationTest.monitorProviderPoolCancellation({
    isActive: () => true,
    cadenceMs: 0,
    lifetimeMs: 1000,
    readScope: async () => { throw new Error("prefix: no provider-pool chat is active"); },
    onScope: () => {},
    onTransportFailure: () => { nearMissFailures += 1; },
  });
  if (nearMissStatus !== "transport_error" || nearMissFailures !== 1) {
    throw new Error("Cancellation monitor accepted a non-exact expected-miss response.");
  }

  const routeCases = [
    ["codex", "codex_app_server", "normal question", "direct_chat", true],
    ["codex", "codex_app_server", "/codex explicit", "direct_chat", false],
    ["codex", "codex_app_server", "/status", "direct_chat", false],
    ["codex", "codex_exec", "normal question", "direct_chat", false],
    ["codex", "", "normal question", "direct_chat", false],
    ["codex", "unknown", "normal question", "direct_chat", false],
    ["ollama", "codex_app_server", "normal question", "direct_chat", false],
    ["claude", "codex_app_server", "normal question", "direct_chat", false],
    ["codex", "codex_app_server", "workflow", "workflow", false],
    ["codex", "codex_app_server", "prepared edit", "prepared_edit", false],
    ["codex", "codex_app_server", "bounded worker", "bounded_worker", false],
  ];
  for (const [provider, adapter, text, routeKind, expected] of routeCases) {
    if (cancellationTest.shouldStartTransientInteractiveControls(provider, adapter, text, routeKind) !== expected) {
      throw new Error(`Cancellation route gate was wrong for ${provider}/${adapter}/${routeKind}/${text}.`);
    }
  }

  for (const cleanupClass of ["completion", "cancellation", "failure", "denial", "disconnect", "transport-loss", "teardown", "reset", "session-removal", "view-disposal"]) {
    const cleanupRegistry = new cancellationTest.TransientCancellationRegistry(() => `cleanup-${cleanupClass}-ui`);
    const cleanupInvocation = cleanupRegistry.begin(`cleanup-${cleanupClass}`, `provider-pool-${cleanupClass}`);
    cleanupRegistry.retain(cleanupInvocation.id, scope);
    if (!cleanupRegistry.abortSession(`cleanup-${cleanupClass}`) || cleanupRegistry.project(`cleanup-${cleanupClass}`) !== null) {
      throw new Error(`${cleanupClass} retained a replayable cancellation scope.`);
    }
  }
  const preScopeRegistry = new cancellationTest.TransientCancellationRegistry(() => "pre-scope-ui");
  const preScopeInvocation = preScopeRegistry.begin("pre-scope-session", "pre-scope-chat");
  let monitorAbortCount = 0;
  preScopeRegistry.bindMonitorAbort(preScopeInvocation.id, () => { monitorAbortCount += 1; });
  preScopeRegistry.abortAll();
  if (monitorAbortCount !== 1 || preScopeRegistry.isActive(preScopeInvocation.id)) {
    throw new Error("Teardown did not stop the pre-scope cancellation monitor.");
  }
  const reloadedRegistry = new cancellationTest.TransientCancellationRegistry(() => "reloaded-ui");
  if (reloadedRegistry.project("pipeon-cancellation-session") !== null) {
    throw new Error("Extension reload reconstructed transient cancellation state.");
  }

  const compact = cancellationTest.compactChatStoreForPersistence({
    activeSessionId: "pipeon-cancellation-session",
    sessions: [{ id: "pipeon-cancellation-session", title: "Chat", createdAt: "2026-08-03T00:00:00.000Z", updatedAt: "2026-08-03T00:00:00.000Z", codexSessionAdapter: "codex_app_server", messages: [] }],
    transientCancellation: { scope, intent: deliveredPayload, uiReference: "must-not-persist-cancellation" },
    pendingActions: [{ cancellation: scope }],
    diagnostics: { cancellation: deliveredPayload },
  });
  const serializedCompact = JSON.stringify(compact);
  if (
    serializedCompact.includes("transientCancellation")
    || serializedCompact.includes("must-not-persist-cancellation")
    || serializedCompact.includes("cancellation-process-secret")
    || serializedCompact.includes("user_requested")
  ) {
    throw new Error(`Persisted chat state captured transient cancellation data: ${serializedCompact}`);
  }
}

runCodexAdapterConfigFixtures();
runSessionAdapterHostFixtures()
  .then(() => runApprovalHostFixtures())
  .then(() => runUserInputHostFixtures())
  .then(() => runCancellationHostFixtures())
  .then(() => console.log("webview smoke passed"))
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
