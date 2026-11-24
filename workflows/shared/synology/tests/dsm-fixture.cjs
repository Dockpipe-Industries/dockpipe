"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const sharedSource = fs.readFileSync(path.join(__dirname, "../assets/dsm-native-access.js"), "utf8");
const origin = "https://nas.example.test:5001";
const token = "dummy-private-dsm-token";
const user = "fixture-user";
const uid = 1026;

function configuration(persistent) {
    const packageName = persistent ? "dockpipe-desktop" : "dockpipe-host-task-qualification";
    return {
        scriptPath: "/webman/3rdparty/" + packageName + "/fixture.js",
        reportPath: "/webman/3rdparty/" + packageName + "/native.cgi",
        schema: persistent ? "dockpipe.synology_native_host.v1" : "dockpipe.synology_host_task_qualification.v1",
        scriptPrefix: persistent
            ? "/var/packages/dockpipe-desktop/target/bin/dockpipe host worker --detach --socket /var/packages/dockpipe-desktop/etc/native-host/native-worker.sock --nonce "
            : "/var/packages/dockpipe-host-task-qualification/target/bin/task-host probe --socket /var/packages/dockpipe-host-task-qualification/var/run/task.sock --nonce ",
        taskNamePrefix: persistent ? "Dockpipe host access " : "Dockpipe host test ",
        persistent,
        finishAction: persistent ? "finish" : "clear"
    };
}

function challenge(config) {
    const nonce = "a".repeat(64);
    return {
        nonce, task_name: config.taskNamePrefix + nonce.slice(0, 16), script: config.scriptPrefix + nonce,
        expires_at: "2099-01-01T00:00:00Z"
    };
}

function createBackend(config, options = {}) {
    const state = {
        challenge: options.challenge || null, result: options.ready ? { available: true, uid, node: "fixture-nas" } : null,
        tasks: new Map(), calls: [], grant: false, disabled: false, getCount: 0
    };
    function report(created = false) {
        return {
            schema: config.schema, authenticated_user: user, expected_uid: uid, challenge: state.challenge,
            result: state.result, created, lease_expires_at: state.grant ? new Date(Date.now() + 30000).toISOString() : null
        };
    }
    function json(value, status = 200) {
        return new Response(JSON.stringify(value), { status });
    }
    const backend = {
        state, options,
        async fetch(url, settings) {
            assert([config.reportPath, "/webapi/query.cgi", "/webapi/entry.cgi"].includes(url));
            assert(!url.includes(token));
            assert.equal(settings.headers["X-Syno-Token"], token);
            assert.equal(settings.credentials, "same-origin");
            assert.equal(settings.redirect, "error");
            assert.equal(settings.cache, "no-store");
            if (options.pending)
                return new Promise((resolve, reject) => settings.signal.addEventListener("abort", () => reject(new Error(token)), { once: true }));
            if (options.oversized)
                return new Response("x".repeat(128 * 1024 + 1));
            if (url === config.reportPath) {
                if (settings.method === "GET") {
                    state.calls.push("read");
                    return json(report());
                }
                const body = JSON.parse(settings.body);
                state.calls.push(body.action);
                if (body.action === "prepare") {
                    const created = state.challenge === null && !options.prepareExisting;
                    state.challenge = state.challenge || challenge(config);
                    state.grant = true;
                    state.disabled = false;
                    return json(report(created));
                }
                if (body.action === "heartbeat") {
                    if (options.heartbeatFailure || state.disabled)
                        return json({ error: "lease_expired", private: token }, 403);
                    state.grant = true;
                    const value = report();
                    if (options.heartbeatWrongUID)
                        value.result = { available: true, uid: 0 };
                    if (options.expiredLease)
                        value.lease_expires_at = "2000-01-01T00:00:00Z";
                    return json(value);
                }
                if (body.action === "disconnect") {
                    state.grant = false;
                    state.disabled = true;
                    state.result = null;
                    return json(report());
                }
                assert(["clear", "finish"].includes(body.action));
                assert.equal(body.nonce, state.challenge.nonce);
                assert(![...state.tasks.values()].some(task => task.name === state.challenge.task_name));
                state.challenge = null;
                if (body.action === "clear" || options.finishDropsResult)
                    state.result = null;
                return json(report());
            }
            const fields = Object.fromEntries(new URLSearchParams(settings.body));
            if (url === "/webapi/query.cgi") {
                state.calls.push("discovery");
                return json({ success: true, data: { "SYNO.Core.TaskScheduler": { path: "entry.cgi", minVersion: 1, maxVersion: 4 } } });
            }
            assert.equal(fields.api, "SYNO.Core.TaskScheduler");
            state.calls.push(fields.method);
            switch (fields.method) {
                case "list": {
                    const rows = [...state.tasks.values()];
                    const offset = Number(fields.offset);
                    return json({ success: true, data: { tasks: rows.slice(offset, offset + Number(fields.limit)), total: rows.length } });
                }
                case "get": {
                    const task = state.tasks.get(Number(fields.id));
                    assert.equal(fields.real_owner, task.real_owner);
                    const copy = structuredClone(task);
                    if (options.changedMetadata && ++state.getCount === 2)
                        copy.real_owner = user;
                    return json({ success: true, data: copy });
                }
                case "create": {
                    assert.equal(fields.owner, user);
                    assert.equal(fields.real_owner, user);
                    assert.equal(fields.enable, "false");
                    const schedule = JSON.parse(fields.schedule);
                    assert.equal(schedule.date_type, 1);
                    assert.equal(schedule.repeat_date, 0);
                    assert.equal(JSON.parse(fields.extra).script, state.challenge.script);
                    state.tasks.set(5, {
                        id: 5, owner: user, real_owner: "root", enable: false, type: "script",
                        name: state.challenge.task_name, extra: { script: state.challenge.script }, schedule
                    });
                    if (options.uncertainCreate)
                        throw new Error(token);
                    return json({ success: true, data: { id: 5 } });
                }
                case "run": {
                    assert.deepEqual(JSON.parse(fields.tasks), [{ id: 5, real_owner: "root" }]);
                    assert.equal(state.tasks.get(5).owner, user);
                    state.result = { available: true, uid: options.workerUID === undefined ? uid : options.workerUID, node: "fixture-nas" };
                    if (options.uncertainRun)
                        throw new Error(token);
                    return json({ success: true });
                }
                case "delete": {
                    assert.deepEqual(JSON.parse(fields.tasks), [{ id: 5, real_owner: "root" }]);
                    state.tasks.delete(5);
                    if (options.uncertainDelete)
                        throw new Error(token);
                    return json({ success: true });
                }
                default:
                    throw new Error("Unexpected scheduler method");
            }
        }
    };
    return backend;
}

function createRuntime(config, backend, additions = {}) {
    const timers = new Map();
    let nextTimer = 1;
    function timer(callback, milliseconds, interval) {
        const identifier = nextTimer++;
        timers.set(identifier, { callback, milliseconds, interval });
        if (!interval && milliseconds === 1000) {
            queueMicrotask(() => {
                if (timers.delete(identifier))
                    callback();
            });
        }
        return identifier;
    }
    const runtime = {
        URL, URLSearchParams, Uint8Array, TextDecoder, AbortController,
        window: {
            location: { origin, href: origin + "/webman/index.cgi", protocol: "https:" },
            fetch: backend.fetch
        },
        document: { currentScript: { src: origin + config.scriptPath + "?v=fixture&SynoToken=" + token }, scripts: [] },
        setTimeout: (callback, milliseconds) => timer(callback, milliseconds, false),
        setInterval: (callback, milliseconds) => timer(callback, milliseconds, true),
        clearTimeout: identifier => timers.delete(identifier),
        clearInterval: identifier => timers.delete(identifier),
        ...additions
    };
    const context = vm.createContext(runtime);
    vm.runInContext(sharedSource, context);
    return {
        context, runtime, timers,
        session: context.window.DockpipeNativeAccess.create(config),
        async settle() { await new Promise(resolve => setImmediate(resolve)); },
        async fireInterval(milliseconds) {
            const selected = [...timers.values()].filter(value => value.interval && value.milliseconds === milliseconds);
            for (const current of selected)
                current.callback();
            await new Promise(resolve => setImmediate(resolve));
        }
    };
}

module.exports = { configuration, challenge, createBackend, createRuntime, sharedSource, origin, token, user, uid };
