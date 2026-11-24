"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const script = fs.readFileSync(path.join(__dirname, "../assets/ui/Dockpipe.HostQualification.js"), "utf8");
const launcherConfig = JSON.parse(fs.readFileSync(path.join(__dirname, "../assets/ui/config"), "utf8"));
const origin = "https://nas.example.test:5001";
const scriptPath = "/webman/3rdparty/dockpipe-host-qualification/Dockpipe.HostQualification.js";
const reportPath = "/webman/3rdparty/dockpipe-host-qualification/host-auth.cgi";
const token = "fixture-session-token";
const report = {
    schema: "dockpipe.synology_host_qualification.v1",
    hostname: "fixture-nas",
    authenticated_user: "<img src=x onerror=alert(1)>",
    identity: { uid: 1234, euid: 1234 },
    service: { available: true }
};

function createWindow(options = {}) {
    const definitions = {};
    const events = {};
    const requests = [];
    const timers = new Map();
    const output = { textContent: "" };
    Object.defineProperty(output, "innerHTML", {
        set() { throw new Error("Report data must never become HTML"); }
    });
    const source = options.source === undefined
        ? { src: origin + scriptPath + "?v=fixture&SynoToken=" + token }
        : options.source;
    const context = {
        URL, Uint8Array, TextDecoder, AbortController,
        window: {
            location: { origin, href: origin + "/webman/index.cgi", protocol: options.protocol || "https:" },
            fetch(url, configuration) {
                requests.push({ url, configuration });
                if (options.fetch)
                    return options.fetch(url, configuration);
                return Promise.resolve(new Response(JSON.stringify(report)));
            }
        },
        document: { currentScript: source, scripts: options.scripts || [] },
        setTimeout(callback, milliseconds) {
            const identifier = timers.size + 1;
            timers.set(identifier, { callback, milliseconds });
            return identifier;
        },
        clearTimeout(identifier) { timers.delete(identifier); },
        Ext: {
            ns() {},
            define(name, definition) { definitions[name] = definition; },
            apply(defaults, configuration) { return Object.assign(defaults, configuration); }
        }
    };
    vm.runInNewContext(script, context);
    const definition = definitions["com.dockpipe.HostQualificationWindow"];
    const instance = Object.assign({}, definition, {
        callParent(argumentsList) { this.configuration = argumentsList[0]; },
        on(event, callback, scope) { events[event] = () => callback.call(scope); },
        getComponent(identifier) {
            assert.equal(identifier, "qualification-report");
            return { rendered: true, getEl() { return { dom: output }; } };
        }
    });
    definition.constructor.call(instance, { appInstance: {} });
    assert.equal(instance.configuration.items[0].autoEl.tag, "pre");
    assert.equal(definitions["com.dockpipe.HostQualification"].appWindowName,
        "com.dockpipe.HostQualificationWindow");
    return { instance, events, output, requests, timers, definitions };
}

// Exercise ordinary JavaScript namespace assignment, including replacing an
// existing constructor. This models the registration hazard without claiming
// to reproduce DSM's internal asynchronous loader implementation.
function classRegistry(definitions) {
    const namespace = {};
    return {
        install(name) {
            const parts = name.split(".");
            let parent = namespace;
            for (const part of parts.slice(0, -1)) {
                if (!parent[part])
                    parent[part] = {};
                parent = parent[part];
            }
            const constructor = function() {};
            Object.assign(constructor.prototype, definitions[name]);
            parent[parts.at(-1)] = constructor;
        },
        getClass(name) {
            let candidate = namespace;
            for (const part of name.split(".")) {
                if (!candidate)
                    return null;
                candidate = candidate[part];
            }
            return typeof candidate === "function" ? candidate : null;
        }
    };
}

function verifyIndependentClassRegistration(definitions) {
    const applicationName = "com.dockpipe.HostQualification";
    const windowName = "com.dockpipe.HostQualificationWindow";
    const classes = launcherConfig["Dockpipe.HostQualification.js"];
    assert.equal(classes[applicationName].type, "app");
    assert.equal(classes[applicationName].appWindow, windowName);
    assert.deepEqual(classes[applicationName].depend, [windowName]);
    assert.equal(classes[windowName].type, "lib");
    assert.equal(definitions[applicationName].appWindowName, windowName);

    const legacyWindowName = applicationName + ".Window";
    const legacyDefinitions = {
        [applicationName]: definitions[applicationName],
        [legacyWindowName]: definitions[windowName]
    };
    for (const windowFirst of [false, true]) {
        const registry = classRegistry(definitions);
        const order = windowFirst ? [windowName, applicationName] : [applicationName, windowName];
        for (const name of order)
            registry.install(name);
        assert(registry.getClass(applicationName));
        assert(registry.getClass(windowName));
        registry.install(applicationName);
        assert(registry.getClass(windowName), "Replacing the app constructor must retain its sibling window");
        Object.assign(registry.getClass(windowName).prototype, classes[windowName]);

        const legacyRegistry = classRegistry(legacyDefinitions);
        const legacyOrder = windowFirst
            ? [legacyWindowName, applicationName] : [applicationName, legacyWindowName];
        for (const name of legacyOrder)
            legacyRegistry.install(name);
        if (windowFirst)
            assert.equal(legacyRegistry.getClass(legacyWindowName), null);
        legacyRegistry.install(applicationName);
        assert.equal(legacyRegistry.getClass(legacyWindowName), null);
        assert.throws(() => legacyRegistry.getClass(legacyWindowName).prototype, TypeError);
    }
}

function pendingUntilAborted(url, configuration) {
    return new Promise((resolve, reject) => {
        configuration.signal.addEventListener("abort", () => reject(new Error("fixture abort")), { once: true });
    });
}

async function run() {
    const success = createWindow();
    verifyIndependentClassRegistration(success.definitions);
    await success.events.afterrender();
    assert.deepEqual(JSON.parse(success.output.textContent), report);
    assert.equal(success.requests.length, 1);
    const request = success.requests[0];
    assert.equal(request.url, reportPath);
    assert.equal(request.configuration.headers["X-Syno-Token"], token);
    assert.equal(request.configuration.credentials, "same-origin");
    assert.equal(request.configuration.redirect, "error");
    assert.equal(request.configuration.cache, "no-store");
    assert.equal(request.configuration.referrerPolicy, "no-referrer");
    assert.equal(request.configuration.method, "GET");
    assert.equal(success.timers.size, 0);
    assert.equal(success.instance.reportRequest, null);
    assert(!JSON.stringify(success.instance).includes(token));
    assert(!success.output.textContent.includes(token));

    const fallback = createWindow({ source: null, scripts: [{ src: origin + scriptPath + "?SynoToken=" + token }] });
    await fallback.events.afterrender();
    assert.equal(fallback.requests.length, 1);

    const rejectedSources = [
        null,
        { src: origin + scriptPath },
        { src: "https://foreign.example.test" + scriptPath + "?SynoToken=" + token },
        { src: origin + "/different.js?SynoToken=" + token },
        { src: origin + scriptPath + "?SynoToken=a&SynoToken=b" }
    ];
    for (const source of rejectedSources) {
        const missingToken = createWindow({ source });
        await missingToken.events.afterrender();
        assert.equal(missingToken.requests.length, 0);
        assert.match(missingToken.output.textContent, /session token is unavailable/);
    }
    const ambiguous = createWindow({ source: null, scripts: [
        { src: origin + scriptPath + "?SynoToken=a" },
        { src: origin + scriptPath + "?SynoToken=b" }
    ] });
    await ambiguous.events.afterrender();
    assert.equal(ambiguous.requests.length, 0);

    const insecure = createWindow({ protocol: "http:" });
    await insecure.events.afterrender();
    assert.equal(insecure.requests.length, 0);
    assert.match(insecure.output.textContent, /HTTPS/);

    const unavailable = createWindow({ fetch: () => Promise.resolve(new Response(token, { status: 503 })) });
    await unavailable.events.afterrender();
    assert.equal(unavailable.output.textContent, "Native DSM authentication is unavailable (503).");
    assert(unavailable.requests[0].configuration.signal.aborted);
    const rejected = createWindow({ fetch: () => Promise.reject(new Error(token)) });
    await rejected.events.afterrender();
    assert.equal(rejected.output.textContent, "Native DSM identity report is unavailable.");

    const oversized = createWindow({ fetch: () => Promise.resolve(new Response("x".repeat(16385))) });
    await oversized.events.afterrender();
    assert.equal(oversized.output.textContent, "Native DSM identity report is unavailable.");
    assert(oversized.requests[0].configuration.signal.aborted);
    const boundaryText = JSON.stringify(report).padEnd(16384, " ");
    const boundary = createWindow({ fetch: () => Promise.resolve(new Response(boundaryText)) });
    await boundary.events.afterrender();
    assert.equal(boundary.output.textContent, boundaryText);

    const cancelled = createWindow({ fetch: pendingUntilAborted });
    const cancelledLoad = cancelled.events.afterrender();
    const previousText = cancelled.output.textContent;
    cancelled.events.beforedestroy();
    await cancelledLoad;
    assert(cancelled.requests[0].configuration.signal.aborted);
    assert.equal(cancelled.output.textContent, previousText);
    assert.equal(cancelled.timers.size, 0);

    let cancelledBody = false;
    const streaming = createWindow({ fetch: () => Promise.resolve(new Response(new ReadableStream({
        pull() {},
        cancel() { cancelledBody = true; }
    }))) });
    const streamingLoad = streaming.events.afterrender();
    await new Promise(resolve => setImmediate(resolve));
    assert(streaming.instance.reportRequest.reader);
    streaming.events.beforedestroy();
    await streamingLoad;
    assert(cancelledBody);
    assert.equal(streaming.timers.size, 0);

    const timedOut = createWindow({ fetch: pendingUntilAborted });
    const timedOutLoad = timedOut.events.afterrender();
    const timer = [...timedOut.timers.values()][0];
    assert.equal(timer.milliseconds, 5000);
    timer.callback();
    await timedOutLoad;
    assert.equal(timedOut.output.textContent, "Native DSM identity check timed out.");
    assert.equal(timedOut.timers.size, 0);

    console.log("DSM report fixture passed: independent class registration, session token admission, bounded fetch, text rendering, errors, cancellation and timeout.");
}

run().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
