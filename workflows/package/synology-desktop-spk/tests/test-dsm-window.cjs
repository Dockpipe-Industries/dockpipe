"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const script = fs.readFileSync(path.join(__dirname, "../assets/ui/Dockpipe.Desktop.js"), "utf8");

function createWindow(protocol) {
    const definitions = {};
    const context = {
        window: { location: { protocol } },
        Ext: {
            ns() {},
            define(name, definition) { definitions[name] = definition; },
            apply(defaults, config) { return Object.assign(defaults, config); }
        }
    };
    vm.runInNewContext(script, context);
    const definition = definitions["com.dockpipe.Desktop.Window"];
    const events = {};
    const instance = Object.assign({}, definition, {
        callParent(argumentsList) { this.configuration = argumentsList[0]; },
        on(event, callback, scope) { events[event] = () => callback.call(scope); },
        getComponent() { return this.component; }
    });
    definition.constructor.call(instance, { appInstance: {} });
    return { instance, events, definitions };
}

const secure = createWindow("https:");
assert.equal(secure.definitions["com.dockpipe.Desktop"].extend, "SYNO.SDS.AppInstance");
assert.equal(secure.instance.extend, "SYNO.SDS.AppWindow");
assert.equal(secure.instance.configuration.layout, "fit");
assert.equal(secure.instance.configuration.items[0].autoEl.src,
    "/webman/3rdparty/dockpipe-desktop/desktop/");
assert.equal(secure.instance.configuration.items[0].autoEl.width, "100%");
assert.equal(secure.instance.configuration.items[0].autoEl.height, "100%");

let focusCount = 0;
const frame = {
    src: "desktop/",
    contentWindow: { focus() { focusCount += 1; } }
};
secure.instance.component = { rendered: true, getEl() { return { dom: frame }; } };
secure.events.activate();
secure.events.show();
assert.equal(focusCount, 2);
secure.events.beforedestroy();
assert.equal(frame.src, "about:blank");
secure.instance.component = undefined;
secure.events.activate();
secure.events.beforedestroy();

const insecure = createWindow("http:");
assert.equal(insecure.instance.configuration.items[0].autoEl.tag, "div");
assert.match(insecure.instance.configuration.items[0].autoEl.html, /HTTPS/);
assert.equal(insecure.instance.configuration.items[0].autoEl.src, undefined);
assert.doesNotMatch(script, /window\.open|password|Authorization/);
console.log("DSM shell draft: identity, HTTPS gate, fit sizing, focus, teardown checks passed.");
