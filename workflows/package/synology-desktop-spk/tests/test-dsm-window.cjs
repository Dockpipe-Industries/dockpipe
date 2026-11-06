"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { configuration, createBackend, createRuntime, token } = require("../../../shared/synology/tests/dsm-fixture.cjs");
const entryName = "Dockpipe.Desktop.native-v4.js";
const entry = fs.readFileSync(path.join(__dirname, "../assets/ui/" + entryName), "utf8");
const launcher = JSON.parse(fs.readFileSync(path.join(__dirname, "../assets/ui/config"), "utf8"));
const desktopPath = "/webman/3rdparty/dockpipe-desktop/desktop/";

function createWindow(options = {}) {
    const config = configuration(true);
    config.scriptPath = "/webman/3rdparty/dockpipe-desktop/" + entryName;
    config.reportPath = "/webman/3rdparty/dockpipe-desktop/native-host.cgi";
    const backend = createBackend(config, options.backend || {});
    const definitions = {};
    const events = {};
    const output = { textContent: "" };
    const stage = { textContent: "" };
    let components = new Map();
    const navigations = [];
    let activeCard = 0;
    let frameSource = "about:blank";
    let focusCount = 0;
    const pointerListeners = [];
    const frame = {
        get src() { return frameSource; },
        set src(value) {
            navigations.push(value);
            frameSource = value;
        },
        getAttribute(name) {
            assert.equal(name, "src");
            return frameSource;
        },
        contentWindow: {
            focus() { focusCount++; },
            addEventListener(type, handler, capture) {
                pointerListeners.push({ type, handler, capture });
            }
        }
    };
    Object.defineProperty(output, "innerHTML", {
        set() { throw new Error("Untrusted native status must be rendered as text"); }
    });
    const runtime = createRuntime(config, backend, {
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
            Button() { throw new Error("Use the native DSM button constructor"); }
        }
    });
    if (options.http)
        runtime.context.window.location.protocol = "http:";
    if (options.noToken)
        runtime.context.document.currentScript.src = runtime.context.window.location.origin + config.scriptPath;
    if (options.wrongInitialUID)
        backend.state.result.uid = 0;
    if (options.missingExpiry || options.rawHeartbeatError) {
        const fetch = runtime.context.window.fetch;
        runtime.context.window.fetch = async (url, settings) => {
            if (url === config.reportPath && settings.method === "POST"
                && JSON.parse(settings.body).action === "heartbeat") {
                if (options.rawHeartbeatError)
                    throw new Error(token);
                const response = await fetch(url, settings);
                const report = await response.json();
                delete report.lease_expires_at;
                return new Response(JSON.stringify(report));
            }
            return fetch(url, settings);
        };
    }
    vm.runInContext(entry, runtime.context);
    const definition = definitions["com.dockpipe.DesktopWindow"];
    const instance = Object.assign({}, definition, {
        callParent(argumentsList) {
            this.configuration = argumentsList[0];
            function component(configuration) {
                const children = new Map();
                for (const item of configuration.items || []) {
                    if (item.itemId)
                        children.set(item.itemId, component(item));
                }
                let element = { textContent: "" };
                if (configuration.itemId === "native-status")
                    element = output;
                if (configuration.itemId === "native-stage")
                    element = stage;
                if (configuration.itemId === "desktop-frame")
                    element = frame;
                if (configuration.autoEl && configuration.autoEl.html)
                    element.textContent = configuration.autoEl.html;
                return {
                    rendered: true,
                    getComponent(identifier) { return children.get(identifier); },
                    getEl() { return { dom: element }; }
                };
            }
            components = new Map(this.configuration.items.map(item => [item.itemId, component(item)]));
            frameSource = this.configuration.items[1].autoEl.src;
            this.rendered = options.synchronous === true;
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
            return components.get(identifier);
        },
        getLayout() {
            return { setActiveItem(index) { activeCard = index; } };
        }
    });
    definition.constructor.call(instance, { appInstance: {} });
    for (const button of [instance.enableButton, instance.disableButton])
        assert.equal(button.nativeDSMButton, true);
    return { instance, events, runtime, backend, frame, navigations, output, stage, pointerListeners,
        activeCard: () => activeCard, focusCount: () => focusCount };
}

function controlHeartbeat(fixture) {
    const originalFetch = fixture.runtime.context.window.fetch;
    const control = { failure: null, calls: 0, aborted: 0 };
    fixture.runtime.context.window.fetch = (url, settings) => {
        if (url.endsWith("/native-host.cgi") && settings.method === "POST"
            && JSON.parse(settings.body).action === "heartbeat") {
            control.calls++;
            if (control.failure === "network")
                return Promise.reject(new Error(token));
            if (Number.isInteger(control.failure))
                return Promise.resolve(new Response("proxy unavailable", { status: control.failure }));
            if (control.failure === "pending") {
                return new Promise((resolve, reject) => settings.signal.addEventListener("abort", () => {
                    control.aborted++;
                    reject(new Error(token));
                }, { once: true }));
            }
        }
        return originalFetch(url, settings);
    };
    return control;
}

async function fireTimeout(fixture, identifier) {
    const timer = fixture.runtime.timers.get(identifier);
    assert(timer && !timer.interval);
    fixture.runtime.timers.delete(identifier);
    timer.callback();
    await fixture.runtime.settle();
}

async function run() {
    assert.equal(launcher[entryName]["com.dockpipe.Desktop"].appWindow, "com.dockpipe.DesktopWindow");
    assert.doesNotMatch(entry, /window\.open|password|Authorization|localStorage|sessionStorage/);
    const fresh = createWindow();
    const setup = fresh.instance.configuration.items[0];
    const content = setup.items[0];
    assert.equal(setup.layout, "hbox");
    assert.equal(setup.layoutConfig.pack, "center");
    assert.equal(setup.layoutConfig.align, "middle");
    assert.equal(content.layout, "vbox");
    assert(content.items.includes(fresh.instance.enableButton), "The primary action must be in the centered setup card");
    assert(fresh.instance.enableButton.height >= 48 && fresh.instance.enableButton.width >= 240);
    assert.equal(fresh.instance.enableButton.btnStyle, "blue");
    assert.equal(fresh.instance.configuration.buttons.length, 1);
    assert.equal(fresh.instance.configuration.buttons[0], fresh.instance.disableButton);
    assert.equal(fresh.stage.textContent, "Checking your DSM session");
    assert.equal(fresh.frame.src, "about:blank");
    await fresh.events.afterrender();
    await fresh.events.afterrender();
    assert.deepEqual(fresh.backend.state.calls, ["read"]);
    assert.equal(fresh.frame.src, "about:blank");
    assert.match(fresh.output.textContent, /Enable NAS access/);
    assert.equal(fresh.stage.textContent, "Ready to connect");
    await fresh.instance.enableButton.handler();
    assert.equal(fresh.backend.state.tasks.size, 0);
    assert.equal(fresh.backend.state.challenge, null);
    assert.equal(fresh.frame.src, desktopPath);
    assert.equal(fresh.activeCard(), 1);
    assert(fresh.backend.state.calls.indexOf("heartbeat") < fresh.backend.state.calls.indexOf("create"));
    assert(fresh.backend.state.calls.indexOf("delete") < fresh.backend.state.calls.indexOf("finish"));
    assert.equal(fresh.backend.state.calls.at(-1), "heartbeat", "The completed worker needs a fresh authenticated heartbeat before framing");
    const opened = fresh.navigations.filter(value => value === desktopPath).length;
    await fresh.runtime.fireInterval(10000);
    assert.equal(fresh.navigations.filter(value => value === desktopPath).length, opened, "Heartbeat must not reload the GUI");
    fresh.events.activate();
    fresh.events.show();
    assert(fresh.focusCount() >= 3);
    await fresh.instance.disableButton.handler();
    assert.equal(fresh.frame.src, "about:blank");
    assert.equal(fresh.backend.state.grant, false);
    assert.equal(fresh.backend.state.calls.at(-1), "disconnect");
    assert.equal(fresh.runtime.timers.size, 0);

    const reused = createWindow({ synchronous: true, backend: { ready: true } });
    assert.deepEqual(reused.backend.state.calls, ["read"]);
    await reused.runtime.settle();
    assert.equal(reused.frame.src, desktopPath);
    assert.deepEqual(reused.backend.state.calls, ["read", "heartbeat"]);
    await reused.events.afterrender();
    assert.deepEqual(reused.backend.state.calls, ["read", "heartbeat"]);
    const beforeClose = [...reused.backend.state.calls];
    reused.events.beforedestroy();
    assert.equal(reused.frame.src, "about:blank");
    assert.deepEqual(reused.backend.state.calls, beforeClose, "Window close must not asynchronously revoke a reopened session");
    assert.equal(reused.runtime.timers.size, 0);

    const absentFrame = createWindow();
    absentFrame.instance.getComponent = () => undefined;
    absentFrame.instance.getLayout = () => { throw new Error("A closing window must not reactivate its destroyed layout"); };
    absentFrame.events.beforedestroy();
    assert.equal(absentFrame.runtime.timers.size, 0);

    for (const options of [
        { backend: { ready: true, heartbeatFailure: true } },
        { backend: { ready: true, heartbeatWrongUID: true } },
        { backend: { ready: true, expiredLease: true } },
        { backend: { ready: true }, wrongInitialUID: true },
        { backend: { workerUID: 0 } },
        { backend: { finishDropsResult: true } }
    ]) {
        const blocked = createWindow(options);
        await blocked.events.afterrender();
        if (!options.backend.ready)
            await blocked.instance.enableButton.handler();
        assert.equal(blocked.frame.src, "about:blank");
        assert(!blocked.navigations.includes(desktopPath));
        assert(!blocked.output.textContent.includes(token));
        blocked.events.beforedestroy();
        assert.equal(blocked.runtime.timers.size, 0);
    }

    const lostHeartbeat = createWindow({ backend: { ready: true } });
    await lostHeartbeat.events.afterrender();
    assert.equal(lostHeartbeat.frame.src, desktopPath);
    lostHeartbeat.backend.options.heartbeatFailure = true;
    await lostHeartbeat.runtime.fireInterval(10000);
    assert.equal(lostHeartbeat.frame.src, "about:blank");
    assert.match(lostHeartbeat.output.textContent, /HTTP 403.*lease_expired/);
    assert.equal(lostHeartbeat.stage.textContent, "Connection needs attention");
    assert.equal(lostHeartbeat.runtime.timers.size, 0);

    for (const failure of ["network", 504, "pending"]) {
        const resilient = createWindow({ backend: { ready: true } });
        await resilient.events.afterrender();
        const control = controlHeartbeat(resilient);
        control.failure = failure;
        const deadline = resilient.instance.nativeState.verifiedDeadline;
        const expiryTimer = resilient.instance.nativeState.expiryTimer;
        await resilient.runtime.fireInterval(10000);
        if (failure === "pending") {
            await resilient.runtime.fireInterval(10000);
            assert.equal(control.calls, 1, "Only one renewal request may be in flight");
            const requestTimer = [...resilient.runtime.timers].find(([, timer]) => timer.milliseconds === 8000);
            await fireTimeout(resilient, requestTimer[0]);
            assert.equal(control.aborted, 1);
        }
        assert.equal(resilient.frame.src, desktopPath);
        assert.equal(resilient.instance.nativeState.verifiedDeadline, deadline, "Failure cannot extend a verified lease");
        assert.equal(resilient.instance.nativeState.expiryTimer, expiryTimer);
        assert(resilient.instance.nativeState.retryTimer !== null);
        control.failure = null;
        await fireTimeout(resilient, resilient.instance.nativeState.retryTimer);
        assert.equal(resilient.frame.src, desktopPath);
        assert.equal(resilient.instance.nativeState.retryTimer, null);
        assert.notEqual(resilient.instance.nativeState.expiryTimer, expiryTimer, "Successful authentication replaces the deadline timer");
        resilient.events.beforedestroy();
        assert.equal(resilient.runtime.timers.size, 0);
    }

    const firstTransient = createWindow({ backend: { ready: true } });
    controlHeartbeat(firstTransient).failure = 504;
    await firstTransient.events.afterrender();
    assert.equal(firstTransient.frame.src, "about:blank");
    assert.equal(firstTransient.runtime.timers.size, 0, "An unverified first heartbeat cannot fall back to a GET or prepare response");

    for (const finish of ["expiry", "disable", "close"]) {
        const retrying = createWindow({ backend: { ready: true } });
        await retrying.events.afterrender();
        controlHeartbeat(retrying).failure = 504;
        await retrying.runtime.fireInterval(10000);
        assert(retrying.instance.nativeState.retryTimer !== null);
        if (finish === "expiry")
            await fireTimeout(retrying, retrying.instance.nativeState.expiryTimer);
        else if (finish === "disable")
            await retrying.instance.disableButton.handler();
        else
            retrying.events.beforedestroy();
        assert.equal(retrying.frame.src, "about:blank");
        assert.equal(retrying.runtime.timers.size, 0, finish + " must cancel all renewal timers");
    }

    for (const finish of ["expiry", "disable", "close"]) {
        const pendingHeartbeat = createWindow({ backend: { ready: true } });
        await pendingHeartbeat.events.afterrender();
        const control = controlHeartbeat(pendingHeartbeat);
        control.failure = "pending";
        await pendingHeartbeat.runtime.fireInterval(10000);
        if (finish === "expiry")
            await fireTimeout(pendingHeartbeat, pendingHeartbeat.instance.nativeState.expiryTimer);
        else if (finish === "disable")
            await pendingHeartbeat.instance.disableButton.handler();
        else {
            pendingHeartbeat.events.beforedestroy();
            await pendingHeartbeat.runtime.settle();
        }
        assert.equal(control.aborted, 1, finish + " must abort the in-flight renewal");
        assert.equal(pendingHeartbeat.frame.src, "about:blank");
        assert.equal(pendingHeartbeat.runtime.timers.size, 0);
    }

    const unauthorized = createWindow({ backend: { ready: true } });
    await unauthorized.events.afterrender();
    controlHeartbeat(unauthorized).failure = 401;
    await unauthorized.runtime.fireInterval(10000);
    assert.equal(unauthorized.frame.src, "about:blank");
    assert.equal(unauthorized.runtime.timers.size, 0);

    const expired = createWindow({ backend: { ready: true } });
    await expired.events.afterrender();
    const expiry = [...expired.runtime.timers.values()].find(timer => !timer.interval && timer.milliseconds > 20000);
    assert(expiry);
    expiry.callback();
    assert.equal(expired.frame.src, "about:blank");
    assert.equal(expired.runtime.timers.size, 0);

    const missingExpiry = createWindow({ missingExpiry: true });
    await missingExpiry.events.afterrender();
    await missingExpiry.instance.enableButton.handler();
    assert.match(missingExpiry.output.textContent, /server did not provide a valid session expiry/);
    assert.equal(missingExpiry.stage.textContent, "Connection needs attention");
    assert(!missingExpiry.backend.state.calls.includes("create"));
    assert.equal(missingExpiry.frame.src, "about:blank");
    assert.equal(missingExpiry.runtime.timers.size, 0);

    const networkFailure = createWindow({ rawHeartbeatError: true, backend: { ready: true } });
    await networkFailure.events.afterrender();
    assert.equal(networkFailure.frame.src, "about:blank");
    assert(!networkFailure.output.textContent.includes(token));
    assert.equal(networkFailure.stage.textContent, "Connection needs attention");

    for (const options of [{ http: true }, { noToken: true }]) {
        const blocked = createWindow(options);
        await blocked.events.afterrender();
        await blocked.instance.enableButton.handler();
        assert.equal(blocked.backend.state.calls.length, 0);
        assert.equal(blocked.frame.src, "about:blank");
    }

    const cancelled = createWindow({ backend: { pending: true } });
    const loading = cancelled.events.afterrender();
    cancelled.events.beforedestroy();
    await loading;
    assert.equal(cancelled.frame.src, "about:blank");
    assert.equal(cancelled.runtime.timers.size, 0);

    const interrupted = createWindow();
    await interrupted.events.afterrender();
    const originalFetch = interrupted.runtime.context.window.fetch;
    let awaitingCreate = false;
    interrupted.runtime.context.window.fetch = (url, settings) => {
        const fields = Object.fromEntries(new URLSearchParams(settings.body || ""));
        if (url === "/webapi/entry.cgi" && fields.method === "create") {
            awaitingCreate = true;
            return new Promise((resolve, reject) => settings.signal.addEventListener("abort", () => reject(new Error("fixture abort")), { once: true }));
        }
        return originalFetch(url, settings);
    };
    const enabling = interrupted.instance.enableButton.handler();
    await interrupted.runtime.settle();
    assert(awaitingCreate);
    assert.equal(interrupted.instance.disableButton.disabled, false);
    await interrupted.instance.disableButton.handler();
    await enabling;
    assert.equal(interrupted.frame.src, "about:blank");
    assert.equal(interrupted.backend.state.grant, false);
    assert.equal(interrupted.runtime.timers.size, 0);

    const embeddedPointer = createWindow({ backend: { ready: true } });
    await embeddedPointer.events.afterrender();
    assert.equal(embeddedPointer.frame.src, desktopPath);
    embeddedPointer.frame.onload();
    assert.equal(embeddedPointer.pointerListeners.length, 1);
    const pointerGuard = embeddedPointer.pointerListeners[0];
    assert.equal(pointerGuard.type, "pointerrawupdate");
    assert.equal(pointerGuard.capture, true);
    for (const pointerType of ["mouse", "pen", "touch", ""]) {
        let stopped = false;
        pointerGuard.handler({ pointerType, stopImmediatePropagation() { stopped = true; } });
        assert.equal(stopped, pointerType === "mouse" || pointerType === "pen");
    }
    const frameLoadHandler = embeddedPointer.frame.onload;
    await embeddedPointer.instance.renewLease();
    assert.equal(embeddedPointer.frame.onload, frameLoadHandler);
    assert.equal(embeddedPointer.pointerListeners.length, 1);
    embeddedPointer.events.beforedestroy();
    assert.equal(embeddedPointer.frame.src, "about:blank");

    console.log("DSM desktop launcher passed: shared setup, synchronous/delayed render, verified UID/heartbeat gate, lease renewal/expiry, native buttons, explicit disable and teardown.");
}

const watchdog = setTimeout(() => { console.error("DSM desktop fixture timed out."); process.exitCode = 1; }, 8000);
run().catch(error => { console.error(error); process.exitCode = 1; }).finally(() => clearTimeout(watchdog));
