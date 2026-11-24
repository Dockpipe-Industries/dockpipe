"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const source = fs.readFileSync(path.join(__dirname, "../assets/task-ui/Dockpipe.HostTaskQualification.v3.js"), "utf8");
const config = JSON.parse(fs.readFileSync(path.join(__dirname, "../assets/task-ui/config"), "utf8"));
const origin = "https://nas.example.test:5001";
const scriptPath = "/webman/3rdparty/dockpipe-host-task-qualification/Dockpipe.HostTaskQualification.v3.js";
const reportPath = "/webman/3rdparty/dockpipe-host-task-qualification/host-auth.cgi";
const token = "dummy-dsm-session-token";
const schema = "dockpipe.synology_host_task_qualification.v1";
const user = "fixture-user";

function challenge(nonce = "a".repeat(64)) {
    return {
        nonce,
        script: "/var/packages/dockpipe-host-task-qualification/target/bin/task-host probe"
            + " --socket /var/packages/dockpipe-host-task-qualification/var/run/task.sock --nonce " + nonce,
        task_name: "Dockpipe host test " + nonce.slice(0, 16),
        expires_at: "2099-01-01T00:00:00Z"
    };
}

function taskFor(value, identifier = 42, owner = user, realOwner = owner) {
    return {
        id: identifier, name: value.task_name, owner, real_owner: realOwner,
        type: "script", enable: false, extra: { script: value.script },
        schedule: { date_type: 1, repeat_date: 0, repeat_hour: 0, repeat_min: 0 }
    };
}

function server(options = {}) {
    const state = { challenge: options.challenge || null, result: null, tasks: new Map(), calls: [], getCount: 0 };
    for (const task of options.tasks || [])
        state.tasks.set(task.id, task);
    function report(created = false) {
        return { schema, authenticated_user: user, challenge: state.challenge, result: state.result, created };
    }
    function json(data) {
        return new Response(JSON.stringify(data));
    }
    return {
        state,
        async fetch(url, settings) {
            assert([reportPath, "/webapi/query.cgi", "/webapi/entry.cgi"].includes(url));
            assert(!url.includes(token));
            assert.equal(settings.headers["X-Syno-Token"], token);
            assert.equal(settings.credentials, "same-origin");
            assert.equal(settings.redirect, "error");
            assert.equal(settings.cache, "no-store");
            assert.equal(settings.referrerPolicy, "no-referrer");
            if (options.pending)
                return new Promise((resolve, reject) => settings.signal.addEventListener("abort", () => reject(new Error(token)), { once: true }));
            if (options.oversized)
                return new Response("x".repeat(128 * 1024 + 1), { status: options.httpStatus || 200 });
            if (options.httpFailure)
                return new Response(JSON.stringify({ error: options.httpFailure, detail: token }), { status: 403 });
            if (options.invalidErrorBody)
                return new Response("<html>" + token + "</html>", { status: 503 });
            if (url === reportPath) {
                if (settings.method === "GET") {
                    state.calls.push("report");
                    return json(report());
                }
                const body = JSON.parse(settings.body);
                state.calls.push(body.action);
                if (body.action === "prepare") {
                    const created = state.challenge === null && !options.prepareExisting;
                    state.challenge = state.challenge || challenge();
                    return json(report(created));
                }
                assert.equal(body.action, "clear");
                assert.equal(body.nonce, state.challenge.nonce);
                assert(![...state.tasks.values()].some(task => task.name === state.challenge.task_name));
                state.challenge = null;
                state.result = null;
                return json(report());
            }
            assert.equal(settings.method, "POST");
            const fields = Object.fromEntries(new URLSearchParams(settings.body));
            if (url === "/webapi/query.cgi") {
                state.calls.push("discovery");
                assert.equal(fields.api, "SYNO.API.Info");
                assert.equal(fields.query, "SYNO.Core.TaskScheduler");
                const api = { path: "entry.cgi", minVersion: 1, maxVersion: 4 };
                if (options.unsupported)
                    api.maxVersion = 3;
                if (options.foreignDiscovery)
                    api.path = "https://foreign.example.test/steal";
                return json({ success: true, data: { "SYNO.Core.TaskScheduler": api } });
            }
            assert.equal(fields.api, "SYNO.Core.TaskScheduler");
            state.calls.push(fields.method);
            if (options.schedulerError)
                return json({ success: false, error: { code: 4800, detail: token } });
            switch (fields.method) {
                case "list": {
                    assert.equal(fields.version, "3");
                    const tasks = [...state.tasks.values()];
                    const offset = Number(fields.offset);
                    const limit = Number(fields.limit);
                    let total = tasks.length;
                    if (options.incompleteList)
                        total = 101;
                    return json({ success: true, data: { tasks: tasks.slice(offset, offset + limit), total } });
                }
                case "get": {
                    assert.equal(fields.version, "4");
                    const stored = state.tasks.get(Number(fields.id));
                    assert.equal(fields.real_owner, stored.real_owner);
                    const task = structuredClone(stored);
                    state.getCount++;
                    if (options.changeScript)
                        task.extra.script = "/bin/sh -c unrelated";
                    if (options.changeBookkeeping && state.getCount === 2)
                        task.real_owner = user;
                    if (options.changeExecutionOwner && state.getCount === 2)
                        task.owner = "root";
                    return json({ success: true, data: task });
                }
                case "create": {
                    assert.equal(fields.version, "4");
                    assert.equal(fields.owner, user);
                    assert.equal(fields.real_owner, user);
                    assert.equal(fields.enable, "false");
                    assert.equal(fields.type, "script");
                    const schedule = JSON.parse(fields.schedule);
                    assert.equal(schedule.date_type, 1);
                    assert.equal(schedule.repeat_date, 0);
                    assert.equal(schedule.repeat_hour, 0);
                    assert.equal(schedule.repeat_min, 0);
                    const extra = JSON.parse(fields.extra);
                    assert.equal(extra.script, state.challenge.script);
                    assert.equal(extra.notify_enable, false);
                    assert.equal(extra.notify_mail, "");
                    if (!options.createAbsent) {
                        const realOwner = options.rootBookkeeping ? "root" : user;
                        state.tasks.set(42, taskFor(state.challenge, 42, user, realOwner));
                    }
                    if (options.uncertainCreate)
                        throw new Error(token);
                    return json({ success: true, data: { id: 42 } });
                }
                case "run": {
                    assert.equal(fields.version, "2");
                    const task = state.tasks.get(42);
                    assert.equal(task.owner, user, "The task must execute as the authenticated normal user");
                    assert.deepEqual(JSON.parse(fields.tasks), [{ id: 42, real_owner: task.real_owner }]);
                    state.result = { hostname: "fixture-nas", identity: { uid: 1026 }, message: "<img src=x onerror=alert(1)>" };
                    if (options.uncertainRun)
                        throw new Error(token);
                    return json({ success: true });
                }
                case "delete": {
                    assert.equal(fields.version, "2");
                    const tasks = JSON.parse(fields.tasks);
                    assert.equal(tasks.length, 1);
                    const task = state.tasks.get(tasks[0].id);
                    assert.equal(task.owner, user);
                    assert.equal(tasks[0].real_owner, task.real_owner);
                    if (!options.deleteNoop)
                        state.tasks.delete(tasks[0].id);
                    if (options.uncertainDelete)
                        throw new Error(token);
                    return json({ success: true });
                }
                default:
                    throw new Error("Unexpected scheduler operation");
            }
        }
    };
}

function createWindow(backend, options = {}) {
    const definitions = {};
    const events = {};
    const timers = new Map();
    const output = { textContent: "" };
    Object.defineProperty(output, "innerHTML", {
        set() { throw new Error("Untrusted reports must never become HTML"); }
    });
    let nextTimer = 1;
    const context = {
        URL, URLSearchParams, Uint8Array, TextDecoder, AbortController,
        window: {
            location: { origin, href: origin + "/webman/index.cgi", protocol: options.protocol || "https:" },
            fetch: backend.fetch
        },
        document: {
            currentScript: { src: options.currentScriptSource || origin + scriptPath + (options.noToken ? "" : "?v=test&SynoToken=" + token) },
            scripts: options.scripts || []
        },
        setTimeout(callback, milliseconds) {
            const identifier = nextTimer++;
            timers.set(identifier, { callback, milliseconds });
            if (milliseconds === 1000) {
                queueMicrotask(() => {
                    if (timers.delete(identifier))
                        callback();
                });
            }
            return identifier;
        },
        clearTimeout(identifier) { timers.delete(identifier); },
        SYNO: {
            ux: {
                Button: function(settings) {
                    Object.assign(this, settings);
                    this.nativeDSMButton = true;
                    this.setDisabled = function(disabled) { this.disabled = disabled; };
                }
            }
        },
        Ext: {
            ns() {},
            define(name, definition) { definitions[name] = definition; },
            apply(defaults, settings) { return Object.assign(defaults, settings); },
            Button: function() { throw new Error("Task controls must use the native DSM button constructor"); }
        }
    };
    vm.runInNewContext(source, context);
    const definition = definitions["com.dockpipe.HostTaskQualificationWindow"];
    const instance = Object.assign({}, definition, {
        callParent(argumentsList) {
            this.configuration = argumentsList[0];
            output.textContent = this.configuration.items[0].autoEl.html;
            this.rendered = options.synchronousParentRender === true;
            if (this.rendered && events.afterrender)
                events.afterrender();
        },
        on(event, callback, scope) {
            events[event] = () => {
                if (event === "afterrender")
                    this.rendered = true;
                return callback.call(scope || this);
            };
        },
        getComponent(identifier) {
            assert.equal(identifier, "task-report");
            return { rendered: true, getEl() { return { dom: output }; } };
        }
    });
    definition.constructor.call(instance, { appInstance: {} });
    const buttons = instance.configuration.buttons;
    assert.equal(buttons.length, 3);
    for (const button of buttons)
        assert.equal(button.nativeDSMButton, true);
    return { instance, output, events, timers, enable: buttons[0].handler, inspect: buttons[1].handler, remove: buttons[2].handler };
}

function count(backend, method) {
    return backend.state.calls.filter(value => value === method).length;
}

async function run() {
    const classes = config["Dockpipe.HostTaskQualification.v3.js"];
    assert.equal(classes["com.dockpipe.HostTaskQualification"].appWindow, "com.dockpipe.HostTaskQualificationWindow");

    const synchronous = server();
    const synchronousUI = createWindow(synchronous, { synchronousParentRender: true });
    assert.deepEqual(synchronous.state.calls, ["report"], "A synchronous parent render must start the initial GET");
    assert.match(synchronousUI.output.textContent, /Checking your DSM session/);
    await new Promise(resolve => setImmediate(resolve));
    assert.match(synchronousUI.output.textContent, /Select Enable NAS access/);
    await synchronousUI.events.afterrender();
    assert.deepEqual(synchronous.state.calls, ["report"], "A late render event must not repeat the initial request");
    assert.match(synchronousUI.instance.configuration.items[0].autoEl.style, /box-sizing: border-box/);

    const asynchronous = server();
    const asynchronousUI = createWindow(asynchronous);
    assert.deepEqual(asynchronous.state.calls, []);
    assert.match(asynchronousUI.output.textContent, /Checking your DSM session/);
    await asynchronousUI.events.afterrender();
    await asynchronousUI.events.afterrender();
    assert.deepEqual(asynchronous.state.calls, ["report"], "An asynchronous render must start exactly one read-only request");
    const normal = server();
    const ui = createWindow(normal);
    await ui.events.afterrender();
    assert.deepEqual(normal.state.calls, ["report"], "Opening the app must not write or invoke scheduler APIs");
    await ui.enable();
    assert.equal(count(normal, "create"), 1);
    assert.equal(count(normal, "run"), 1);
    assert.match(ui.output.textContent, /result is ready/);
    assert(ui.output.textContent.includes("<img src=x onerror=alert(1)>"));
    assert(!ui.output.textContent.includes(token));
    await ui.enable();
    assert.equal(count(normal, "create"), 1);
    assert.equal(count(normal, "run"), 1);
    await ui.remove();
    assert.equal(count(normal, "delete"), 1);
    assert.equal(count(normal, "clear"), 1);
    assert.equal(normal.state.tasks.size, 0);
    assert.equal(ui.timers.size, 0);

    const bookkeeping = server({ rootBookkeeping: true });
    const bookkeepingUI = createWindow(bookkeeping);
    await bookkeepingUI.enable();
    assert.equal(count(bookkeeping, "run"), 1);
    assert.equal(bookkeeping.state.tasks.get(42).owner, user);
    assert.equal(bookkeeping.state.tasks.get(42).real_owner, "root");
    await bookkeepingUI.remove();
    assert.equal(count(bookkeeping, "delete"), 1);
    assert.equal(count(bookkeeping, "clear"), 1);

    for (const setting of ["changeBookkeeping", "changeExecutionOwner"]) {
        const changed = server({ rootBookkeeping: true, [setting]: true });
        const changedUI = createWindow(changed);
        await changedUI.enable();
        assert.equal(count(changed, "create"), 1);
        assert.equal(count(changed, "run"), 0, "Ownership changes between readbacks must prevent execution");
    }

    const uncertain = server({ uncertainCreate: true });
    const uncertainUI = createWindow(uncertain);
    await uncertainUI.enable();
    assert.match(uncertainUI.output.textContent, /could not be confirmed/);
    await uncertainUI.inspect();
    await uncertainUI.enable();
    assert.equal(count(uncertain, "create"), 1);
    assert.equal(count(uncertain, "run"), 0);
    const reloaded = createWindow(uncertain);
    await reloaded.enable();
    assert.equal(count(uncertain, "create"), 1);
    assert.equal(count(uncertain, "run"), 0);
    await reloaded.remove();
    assert.equal(count(uncertain, "delete"), 1);
    assert.equal(count(uncertain, "clear"), 1);

    const absent = server({ uncertainCreate: true, createAbsent: true });
    const absentUI = createWindow(absent);
    await absentUI.enable();
    await absentUI.remove();
    assert.equal(count(absent, "create"), 1);
    assert.equal(count(absent, "run"), 0);
    assert.equal(count(absent, "clear"), 0);
    assert.match(absentUI.output.textContent, /uncertain/);

    const shared = server({ prepareExisting: true });
    const sharedUI = createWindow(shared);
    await sharedUI.enable();
    assert.equal(count(shared, "create"), 0, "Only the atomic prepare creator may create a scheduler task");

    const fullChallenge = challenge();
    const fullTasks = [taskFor(fullChallenge, 2048)];
    for (let index = 0; index < 1023; index++) {
        const task = taskFor(challenge("b".repeat(64)), index + 1, "root");
        task.name = "Unrelated task " + index;
        fullTasks.push(task);
    }
    const fullList = server({ challenge: fullChallenge, tasks: fullTasks });
    const fullUI = createWindow(fullList);
    await fullUI.enable();
    assert.equal(count(fullList, "list"), 11, "The last partial page must cover all 1024 tasks");
    assert.equal(count(fullList, "create"), 0);
    assert.equal(count(fullList, "run"), 0);

    const rejectedScheduler = server({ schedulerError: true });
    const rejectedUI = createWindow(rejectedScheduler);
    await rejectedUI.enable();
    assert.match(rejectedUI.output.textContent, /error 4800/);
    assert(!rejectedUI.output.textContent.includes(token));

    for (const setting of ["unsupported", "foreignDiscovery", "incompleteList", "changeScript"]) {
        const backend = server({ [setting]: true });
        const window = createWindow(backend);
        await window.enable();
        assert.equal(count(backend, "run"), 0, setting);
        if (setting !== "changeScript")
            assert.equal(count(backend, "create"), 0, setting);
        assert(!window.output.textContent.includes(token));
    }

    for (const mutation of [task => { task.owner = "root"; }, task => { task.real_owner = "unrelated"; }, task => { task.enable = true; }, task => { task.extra.script += " --different"; }]) {
        const value = challenge();
        const task = taskFor(value);
        mutation(task);
        const backend = server({ challenge: value, tasks: [task] });
        const window = createWindow(backend);
        await window.enable();
        await window.remove();
        assert.equal(count(backend, "create"), 0);
        assert.equal(count(backend, "run"), 0);
        assert.equal(count(backend, "delete"), 0);
        assert.equal(count(backend, "clear"), 0);
    }

    const expired = challenge();
    expired.expires_at = "2000-01-01T00:00:00Z";
    const expiredBackend = server({ challenge: expired, tasks: [taskFor(expired)] });
    const expiredUI = createWindow(expiredBackend);
    await expiredUI.enable();
    assert.equal(count(expiredBackend, "run"), 0);
    await expiredUI.remove();
    assert.equal(count(expiredBackend, "delete"), 1);

    const lostRun = server({ uncertainRun: true, uncertainDelete: true });
    const lostRunUI = createWindow(lostRun);
    await lostRunUI.enable();
    await lostRunUI.inspect();
    assert.equal(count(lostRun, "run"), 1);
    await lostRunUI.remove();
    assert.equal(count(lostRun, "delete"), 1);
    assert.equal(count(lostRun, "clear"), 1);

    const leftover = taskFor(challenge("b".repeat(64)), 21, user, "root");
    const another = taskFor(challenge("c".repeat(64)), 22);
    const unrelated = taskFor(challenge("d".repeat(64)), 23, "root");
    const orphaned = server({ tasks: [leftover, another, unrelated] });
    const orphanUI = createWindow(orphaned);
    await orphanUI.remove();
    assert.equal(count(orphaned, "delete"), 2);
    assert.equal(count(orphaned, "run"), 0);
    assert.equal(count(orphaned, "clear"), 0);
    assert.deepEqual([...orphaned.state.tasks.keys()], [23]);

    for (const options of [{ noToken: true }, { protocol: "http:" }]) {
        const backend = server();
        await createWindow(backend, options).enable();
        assert.equal(backend.state.calls.length, 0);
    }
    const loaderBackend = server();
    const loaderUI = createWindow(loaderBackend, {
        currentScriptSource: origin + "/webman/loader.js?SynoToken=unrelated",
        scripts: [{ src: origin + scriptPath + "?SynoToken=" + token }]
    });
    await loaderUI.events.afterrender();
    assert.deepEqual(loaderBackend.state.calls, ["report"]);
    const foreignBackend = server();
    const foreignUI = createWindow(foreignBackend, {
        currentScriptSource: "https://foreign.example.test" + scriptPath + "?SynoToken=" + token,
        scripts: [{ src: "https://foreign.example.test" + scriptPath + "?SynoToken=" + token }]
    });
    await foreignUI.events.afterrender();
    assert.equal(foreignBackend.state.calls.length, 0);
    const ambiguousBackend = server();
    const ambiguousUI = createWindow(ambiguousBackend, {
        currentScriptSource: origin + "/webman/loader.js",
        scripts: [
            { src: origin + scriptPath + "?SynoToken=" + token },
            { src: origin + scriptPath + "?SynoToken=another" }
        ]
    });
    await ambiguousUI.events.afterrender();
    assert.equal(ambiguousBackend.state.calls.length, 0);
    const huge = createWindow(server({ oversized: true }));
    await huge.enable();
    assert.match(huge.output.textContent, /128 KiB/);

    const forbidden = createWindow(server({ httpFailure: "local_administrator_required" }));
    await forbidden.events.afterrender();
    assert.match(forbidden.output.textContent, /HTTP 403.*local_administrator_required/);
    assert(!forbidden.output.textContent.includes(token));
    const unknownFailure = createWindow(server({ httpFailure: token }));
    await unknownFailure.events.afterrender();
    assert.match(unknownFailure.output.textContent, /HTTP 403/);
    assert(!unknownFailure.output.textContent.includes(token));
    const invalidFailure = createWindow(server({ invalidErrorBody: true }));
    await invalidFailure.events.afterrender();
    assert.match(invalidFailure.output.textContent, /HTTP 503/);
    assert(!invalidFailure.output.textContent.includes(token));
    const hugeFailure = createWindow(server({ oversized: true, httpStatus: 503 }));
    await hugeFailure.events.afterrender();
    assert.match(hugeFailure.output.textContent, /128 KiB/);

    const pending = createWindow(server({ pending: true }));
    const completion = pending.events.afterrender();
    pending.events.beforedestroy();
    await completion;
    assert.equal(pending.timers.size, 0);
    const timeout = createWindow(server({ pending: true }));
    const timedCompletion = timeout.enable();
    [...timeout.timers.values()].find(timer => timer.milliseconds === 8000).callback();
    await timedCompletion;
    assert.match(timeout.output.textContent, /failed or timed out/);
    assert.equal(timeout.timers.size, 0);

    console.log("Task setup fixture passed: explicit writes, one-shot user task, uncertainty/reload/concurrency, ownership guards, bounded reads, cancellation and exact stale-task removal.");
}

const watchdog = setTimeout(() => {
    console.error("Task setup fixture did not complete within its deadline.");
    process.exitCode = 1;
}, 8000);
run().catch(error => {
    console.error(error);
    process.exitCode = 1;
}).finally(() => clearTimeout(watchdog));
