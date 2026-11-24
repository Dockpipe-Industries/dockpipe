"use strict";

const assert = require("node:assert/strict");
const { configuration, createBackend, createRuntime, uid } = require("./dsm-fixture.cjs");

async function fixture(persistent, options = {}) {
    const config = configuration(persistent);
    const backend = createBackend(config, options);
    const runtime = createRuntime(config, backend);
    const messages = [];
    let controller;
    controller = runtime.session.createController((report, message) => messages.push({ report, message }), () => {}, async report => {
        if (persistent)
            await controller.heartbeat(report.authenticated_user);
    });
    return { backend, runtime, controller, messages };
}

async function run() {
    const qualification = await fixture(false);
    const tested = await qualification.controller.perform("enable");
    assert(tested.result);
    assert.equal(qualification.backend.state.tasks.size, 1);
    assert(!qualification.backend.state.calls.includes("finish"));
    await qualification.controller.perform("remove");
    assert.equal(qualification.backend.state.result, null);
    assert(qualification.backend.state.calls.includes("clear"));

    const product = await fixture(true);
    const ready = await product.controller.perform("enable");
    assert.equal(ready.result.uid, uid);
    assert.equal(ready.result.available, true);
    assert.equal(ready.challenge, null);
    assert.equal(product.backend.state.tasks.size, 0);
    assert(product.backend.state.calls.indexOf("heartbeat") < product.backend.state.calls.indexOf("create"));
    assert(product.backend.state.calls.indexOf("delete") < product.backend.state.calls.indexOf("finish"));
    assert(!product.backend.state.calls.includes("clear"));
    assert.equal((await product.controller.heartbeat(ready.authenticated_user)).result.uid, uid);
    await product.controller.disconnect(ready.authenticated_user);
    assert.equal(product.backend.state.grant, false);

    for (const options of [{ workerUID: 0 }, { finishDropsResult: true }, { changedMetadata: true }]) {
        const rejected = await fixture(true, options);
        assert.equal(await rejected.controller.perform("enable"), null);
        assert.equal(rejected.messages.at(-1).report, null);
        if (options.changedMetadata)
            assert(!rejected.backend.state.calls.includes("run"));
    }

    const uncertain = await fixture(true, { uncertainCreate: true });
    await uncertain.controller.perform("enable");
    await uncertain.controller.perform("enable");
    assert.equal(uncertain.backend.state.calls.filter(value => value === "create").length, 1);
    assert(!uncertain.backend.state.calls.includes("run"));
    await uncertain.controller.perform("remove");
    assert.equal(uncertain.backend.state.tasks.size, 0);
    assert.equal(uncertain.backend.state.challenge, null);
    assert(uncertain.backend.state.calls.includes("finish"));

    const duplicate = await fixture(true, { prepareExisting: true });
    await duplicate.controller.perform("enable");
    assert(!duplicate.backend.state.calls.includes("create"));

    const huge = await fixture(false, { oversized: true });
    assert.equal(await huge.controller.perform("load"), null);
    assert.match(huge.messages.at(-1).message, /128 KiB/);
    const pending = await fixture(true, { pending: true });
    const operation = pending.controller.perform("load");
    pending.controller.cancel();
    await operation;
    assert.equal(pending.runtime.timers.size, 0);

    for (const milliseconds of [8000, 60000]) {
        const timedOut = await fixture(true, { pending: true });
        const operation = timedOut.controller.perform("load");
        const timer = [...timedOut.runtime.timers.values()].find(value => value.milliseconds === milliseconds);
        assert(timer);
        timer.callback();
        await operation;
        assert.match(timedOut.messages.at(-1).message, milliseconds === 8000 ? /request timed out after 8 seconds/ : /setup timed out after 60 seconds/);
        assert.equal(timedOut.runtime.timers.size, 0);
    }

    const disconnected = await fixture(true);
    disconnected.runtime.runtime.window.fetch = async () => { throw new Error("private raw network details"); };
    assert.equal(await disconnected.controller.perform("load"), null);
    assert.match(disconnected.messages.at(-1).message, /browser could not complete the NAS request/);
    assert(!disconnected.messages.at(-1).message.includes("private raw network details"));

    for (const status of [401, 403, 429, 500, 504]) {
        const response = await fixture(true);
        response.runtime.runtime.window.fetch = async () => new Response("proxy error", { status });
        await assert.rejects(response.controller.heartbeat("fixture-user"), error => {
            assert.equal(error.retryableNASRequest, status >= 500);
            assert.equal(error.safeTaskMessage, true);
            assert.match(error.message, new RegExp("HTTP " + status));
            return true;
        });
    }

    await assert.rejects(disconnected.controller.heartbeat("fixture-user"), error => {
        assert.equal(error.retryableNASRequest, true);
        assert(!error.message.includes("private raw network details"));
        return true;
    });

    for (const cancellation of [false, true]) {
        const pendingHeartbeat = await fixture(true, { pending: true });
        const operation = pendingHeartbeat.controller.heartbeat("fixture-user");
        const rejected = assert.rejects(operation, error => {
            assert.equal(error.retryableNASRequest, !cancellation);
            assert.match(error.message, cancellation ? /cancelled/ : /8 seconds/);
            return true;
        });
        if (cancellation)
            pendingHeartbeat.controller.cancelHeartbeat();
        else
            [...pendingHeartbeat.runtime.timers.values()].find(timer => timer.milliseconds === 8000).callback();
        await rejected;
        assert.equal(pendingHeartbeat.runtime.timers.size, 0);
    }

    const invalidIdentity = await fixture(true, { ready: true, heartbeatWrongUID: true });
    await assert.rejects(invalidIdentity.controller.heartbeat("fixture-user"), error => {
        assert.equal(error.retryableNASRequest, false);
        return true;
    });

    console.log("Shared DSM adapter passed: qualification/production lifecycle, pinned ownership, readiness identity, prepare renewal, uncertainty, bounds and cancellation.");
}

const watchdog = setTimeout(() => { console.error("Shared DSM fixture timed out."); process.exitCode = 1; }, 8000);
run().catch(error => { console.error(error); process.exitCode = 1; }).finally(() => clearTimeout(watchdog));
