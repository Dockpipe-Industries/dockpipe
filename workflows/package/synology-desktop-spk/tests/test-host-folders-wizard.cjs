"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const source = fs.readFileSync(path.join(__dirname, "../assets/host-folders-wizard.js"), "utf8");
const permissionKeys = ["read_data", "write_data", "exe_file", "append_data", "delete", "delete_sub", "read_attr", "write_attr", "read_ext_attr", "write_ext_attr", "read_perm", "change_perm", "take_ownership"];
const inheritance = { this_folder: true, child_files: false, child_folders: false, all_descendants: true };
const clone = value => JSON.parse(JSON.stringify(value));

function permission(overrides = {}) {
    return Object.assign(Object.fromEntries(permissionKeys.map(key => [key, false])), overrides);
}

function rule(name, id, type = "allow", rights = {}, inherit = inheritance, level = 0) {
    return { level, owner_id: id, owner_name: name, owner_type: "user", permission_type: type, permission: permission(rights), inherit: clone(inherit) };
}

function createFixture(options = {}) {
    let entry;
    let model;
    let mount;
    let unmount;
    let time = 100000;
    let nextTimer = 1;
    const timers = new Map();
    const nextTicks = [];
    const observers = [];
    const requests = [];
    const writes = [];
    const advances = [];
    const pickers = [];
    const pending = new Set();
    const stores = new Map();
    const reads = new Map();
    const folderStep = {};
    const confirmationStep = {};
    let activeStep = folderStep;
    let lastPhysical = "";
    const buttons = {
        next: { disabled: false, setDisabled(value) { this.disabled = value; } },
        back: { disabled: false, setDisabled(value) { this.disabled = value; } }
    };
    const owner = {
        installVolume: options.installVolume === undefined ? "/volume1" : options.installVolume,
        getButton(name) { return buttons[name]; },
        getActiveStep() { return activeStep; },
        goNext(next) {
            if (next === false)
                return false;
            assert.equal(next, "confirmation");
            assert.equal(activeStep, folderStep);
            advances.push(clone(model.getValues()));
            activeStep = confirmationStep;
            return true;
        }
    };
    function useHook(props) {
        assert.equal(props.getOwner(), owner);
        return {
            getNext() { return "confirmation"; },
            checkState() {
                // The native hook may reset buttons before the adapter's guard.
                buttons.next.setDisabled(false);
                buttons.back.setDisabled(false);
            }
        };
    }
    useHook.props = { getOwner: { type: Function } };
    function storeFor(physical) {
        if (!stores.has(physical)) {
            stores.set(physical, {
                acl: [
                    rule("another-user", 1028, "allow", { read_data: true }),
                    rule("", 765432, "deny", { delete: true }),
                    rule("inherited-user", 2028, "allow", { read_attr: true }, inheritance, 1)
                ],
                acl_editable: true, change_permission: true, is_acl: true, is_inherited: true
            });
            if (options.initializeACL)
                options.initializeACL(stores.get(physical), physical);
        }
        return stores.get(physical);
    }
    function effective(physical) {
        if (options.allowed)
            return Object.fromEntries(permissionKeys.map(key => [key, true]));
        const rights = permission();
        const rules = storeFor(physical).acl;
        for (const entry of rules) {
            if (entry.owner_name === "dockpipe-desktop" && entry.owner_type === "user" && entry.permission_type === "allow" && entry.inherit.this_folder) {
                for (const key of permissionKeys)
                    rights[key] ||= entry.permission[key];
            }
        }
        for (const entry of rules) {
            if (entry.owner_name === "dockpipe-desktop" && entry.permission_type === "deny" && entry.inherit.this_folder) {
                for (const key of permissionKeys) {
                    if (entry.permission[key])
                        rights[key] = false;
                }
            }
        }
        return options.effective ? options.effective(physical, rights, requests) : rights;
    }
    function applyWrite(configuration) {
        const parameters = configuration.params;
        assert.equal(configuration.api, "SYNO.Core.ACL");
        assert.equal(configuration.method, "set");
        assert.equal(parameters.files, parameters.file_path);
        assert.equal(parameters.dirPaths, "/" + parameters.file_path.split("/").slice(2).join("/"));
        assert.equal(parameters.change_acl, true);
        assert.equal(parameters.acl_recur, false, "never perform a recursive ACL reset");
        const store = storeFor(parameters.file_path);
        assert.equal(parameters.inherited, store.is_inherited);
        assert(parameters.rules.every(entry => !Object.hasOwn(entry, "level")));
        assert(parameters.rules.every(entry => !Object.hasOwn(entry, "owner_is_internal")), "the read-only identity hint is not an ACL update field");
        const inherited = store.acl.filter(entry => entry.level > 0);
        store.acl = parameters.rules.map(entry => ({
            level: 0,
            owner_id: entry.owner_id === undefined ? (options.packageOwnerId || 280000) : entry.owner_id,
            owner_name: entry.owner_name,
            owner_type: entry.owner_type,
            permission_type: entry.permission_type,
            permission: clone(entry.permission), inherit: clone(entry.inherit)
        })).concat(inherited);
        if (options.internalMarkers) {
            for (const entry of store.acl) {
                if (entry.owner_name === "dockpipe-desktop")
                    entry.owner_is_internal = true;
            }
        }
        if (options.normalizeInheritance) {
            for (const entry of store.acl) {
                if (!entry.inherit.child_files && !entry.inherit.child_folders)
                    entry.inherit.all_descendants = false;
            }
        }
        writes.push(clone(configuration));
        if (options.afterSet)
            options.afterSet(store, configuration);
    }
    function Request(configuration) {
        assert.equal(configuration.timeout, 5000);
        assert.equal(configuration.suppressError, true);
        const request = {
            configuration, aborted: false,
            respond(success, data) {
                pending.delete(request);
                configuration.callback(success, data);
            },
            abort() { this.aborted = true; pending.delete(request); },
            completeDefault() {
                const parameters = configuration.params;
                let data;
                if (configuration.api === "SYNO.FileStation.List") {
                    assert.equal(configuration.method, "getinfo");
                    assert.equal(configuration.version, 2);
                    assert.deepEqual(Array.from(parameters.additional), ["real_path", "perm"]);
                    const virtual = parameters.path[0];
                    data = { files: [{ path: virtual, isdir: true, additional: { real_path: lastPhysical, perm: {} } }] };
                    if (options.metadata)
                        data = options.metadata(data);
                } else {
                    assert.equal(configuration.api, "SYNO.Core.ACL");
                    assert.equal(configuration.version, 1);
                    assert(["inspect", "get", "check_self_denied", "set"].includes(configuration.method));
                    const physical = parameters.file_path;
                    assert.match(physical, /^\/volume[1-9][0-9]*\/[^/]+/);
                    if (configuration.method === "inspect") {
                        lastPhysical = physical;
                        assert.equal(parameters.owner_name, "dockpipe-desktop");
                        assert.equal(parameters.owner_type, "user");
                        data = { permission: effective(physical) };
                    } else if (configuration.method === "get") {
                        assert.equal(parameters.include_noname_rules, true);
                        assert.equal(parameters.type, "all");
                        reads.set(physical, (reads.get(physical) || 0) + 1);
                        const store = storeFor(physical);
                        if (options.beforeRead)
                            options.beforeRead(store, physical, reads.get(physical));
                        data = clone(store);
                    } else if (configuration.method === "check_self_denied") {
                        assert.equal(parameters.acl_recur, false);
                        data = { is_denied: options.selfDenied === true };
                    } else {
                        applyWrite(configuration);
                        data = {};
                    }
                }
                request.respond(true, data);
            }
        };
        requests.push(request);
        pending.add(request);
        if (!options.intercept || options.intercept(request, { storeFor, applyWrite }) !== true)
            request.completeDefault();
        return request;
    }
    function Chooser(configuration) {
        this.configuration = configuration;
        this.closed = false;
        this.show = () => {};
        this.close = () => { this.closed = true; configuration.listeners.hide(); };
        this.destroy = () => { this.closed = true; configuration.listeners.destroy(); };
        pickers.push(this);
    }
    const discovery = {
        mode: "upgrade", retained: "", checked: "", unknown: "", ...(options.discovery || {})
    };
    const wizardSource = source.replaceAll("@DOCKPIPE_RESTORE_MODE@", discovery.mode)
        .replaceAll("@DOCKPIPE_RESTORE_VOLUMES@", discovery.retained)
        .replaceAll("@DOCKPIPE_CHECKED_VOLUMES@", discovery.checked)
        .replaceAll("@DOCKPIPE_UNKNOWN_VOLUMES@", discovery.unknown);
    vm.runInNewContext(wizardSource, {
        TextEncoder,
        Date: { now() { return time; } },
        setTimeout(callback, delay) { const id = nextTimer++; timers.set(id, { callback, delay }); return id; },
        clearTimeout(id) { timers.delete(id); },
        window: {
            getComputedStyle(element) { return { paddingBottom: element.paddingBottom || "0px" }; },
            addEventListener() {}, removeEventListener() {}
        },
        ResizeObserver: class {
            constructor(callback) { this.callback = callback; this.observed = []; this.disconnected = false; observers.push(this); }
            observe(element) { this.observed.push(element); }
            disconnect() { this.disconnected = true; }
        },
        Vue: {
            defineComponent(value) { return value; }, ref(value) { return { value }; },
            watchEffect(callback) { callback(); }, onMounted(callback) { mount = callback; },
            nextTick(callback) { nextTicks.push(callback); }, onBeforeUnmount(callback) { unmount = callback; }
        },
        SYNO: {
            API: options.noAPI ? undefined : { Request },
            SDS: {
                PkgManApp: { Custom: { useHook, JsonpLoader: { load(module) { entry = module.default; } } } },
                Utils: { FileChooser: { Chooser } }
            }
        }
    }, { timeout: 1000 });
    assert.equal(entry.name, "dockpipe_host_folders");
    model = entry.render.setup({ getOwner() { return owner; } });
    const context = new Proxy(model, {
        get(target, key) {
            const value = target[key];
            return value && typeof value === "object" && Object.hasOwn(value, "value") ? value.value : value;
        }
    });
    function h(tag, data, children) {
        if (children === undefined && (typeof data === "string" || Array.isArray(data))) {
            children = data;
            data = {};
        }
        return { tag, data: data || {}, children: children || [] };
    }
    return {
        model, owner, buttons, requests, writes, advances, timers, observers, pickers, storeFor,
        mount(element) { model.layoutRoot.value = element; mount(); },
        flushLayout() { for (const callback of nextTicks.splice(0)) callback(); },
        unmount() { unmount(); },
        leaveStep() { activeStep = {}; },
        backToFolders() { activeStep = folderStep; },
        render() { return entry.render.render.call(context, h); },
        values() { return clone(model.getValues()); },
        fireTimers(delay) {
            time += delay;
            for (const [id, timer] of [...timers]) {
                if (timer.delay === delay && timers.delete(id))
                    timer.callback();
            }
        },
        advanceTime(milliseconds) { time += milliseconds; },
        async settle() {
            for (let index = 0; index < 4096 && model.preparingAccess.value; index++)
                await Promise.resolve();
            assert.equal(model.preparingAccess.value, false, "the operation must settle within its bounded request plan");
        },
        async flush() { for (let index = 0; index < 64; index++) await Promise.resolve(); }
    };
}

function find(node, predicate) {
    if (!node || typeof node !== "object")
        return null;
    if (predicate(node))
        return node;
    for (const child of Array.isArray(node.children) ? node.children : [node.children]) {
        const result = find(child, predicate);
        if (result)
            return result;
    }
    return null;
}

function addFolder(fixture, sourcePath, name, access = "ro") {
    fixture.model.addRow();
    const row = fixture.model.rows.value.at(-1);
    fixture.model.setSource(row.id, sourcePath);
    if (name !== undefined)
        fixture.model.setName(row.id, name);
    fixture.model.setAccess(row.id, access);
    return row;
}

async function main() {
    {
        const fixture = createFixture({ allowed: true });
        delete fixture.owner.installVolume;
        assert.equal(fixture.values().wizard_restore_intent, "upgrade");
        assert.equal(fixture.values().wizard_restore_volume, "");
        addFolder(fixture, "/volume2/team/project");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1, "repair without a browser volume still verifies folder access");
        fixture.owner.installVolume = "/volume2";
        assert.throws(() => fixture.values(), /Select Next/, "a later volume hint invalidates the access proof");
    }
    for (const mode of ["restore", "fresh"]) {
        const fixture = createFixture({ discovery: { mode, retained: "/volume1", checked: "/volume1" } });
        delete fixture.owner.installVolume;
        assert.throws(() => fixture.values(), /valid installation volume/);
        assert.equal(fixture.requests.length, 0);
    }
    {
        const fixture = createFixture({
            discovery: { mode: "restore", retained: "/volume1,/volume2", checked: "/volume1,/volume2" },
            installVolume: "/volume2", allowed: true
        });
        assert.equal(fixture.values().wizard_restore_intent, "restore");
        assert.equal(fixture.values().wizard_restore_volume, "/volume2", "use DSM's actual volume choice, never the first discovered dataset");
        addFolder(fixture, "/volume3/team/project");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1);
        fixture.owner.installVolume = "/volume1";
        assert.throws(() => fixture.values(), /Select Next/, "changing the installation dataset invalidates the completed folder proof");
    }

    for (const options of [
        { discovery: { mode: "restore", retained: "/volume1", checked: "/volume1,/volume2" }, installVolume: "/volume2" },
        { discovery: { mode: "unknown", checked: "/volume1", unknown: "/volume1" } },
        { discovery: { mode: "fresh", checked: "/volume2" }, installVolume: "/volume1" },
        { discovery: { mode: "restore", retained: "/volume1,malformed", checked: "/volume1" } },
        { installVolume: "" },
        { installVolume: "/volume1\n" }
    ]) {
        const fixture = createFixture(options);
        addFolder(fixture, "/volume1/team/project");
        fixture.model.checkState();
        assert.equal(fixture.buttons.next.disabled, true);
        assert.equal(fixture.model.getNext(), false);
        assert.equal(fixture.requests.length, 0, "restore selection must be validated before any folder ACL operation");
        assert.throws(() => fixture.values(), /volume|settings/);
    }

    {
        const fixture = createFixture({ discovery: { mode: "fresh", checked: "/volume1,/volume2" }, installVolume: "/volume2" });
        assert.equal(fixture.values().wizard_restore_intent, "fresh");
        assert.equal(fixture.values().wizard_restore_volume, "/volume2");
        assert.equal(fixture.model.getNext(), "confirmation");
        assert.equal(fixture.requests.length, 0);
    }

    {
        const fixture = createFixture();
        assert.equal(fixture.model.getNext(), "confirmation");
        assert.deepEqual(fixture.values(), {
            wizard_host_folder_action: "keep", wizard_host_folders: "[]",
            wizard_restore_intent: "upgrade", wizard_restore_volume: "/volume1"
        });
        const row = addFolder(fixture, "/volume1/projects/My project");
        assert.equal(row.name, "My project");
        fixture.model.setSource(row.id, "/volume1/projects/Latest");
        assert.equal(row.name, "Latest");
        fixture.model.setName(row.id, "Override");
        fixture.model.setSource(row.id, "/volume1/projects/Final");
        assert.equal(row.name, "Override");
        fixture.model.useFolderName(row.id);
        assert.equal(row.name, "Final");
        fixture.render();
        fixture.model.checkState();
        assert.equal(fixture.requests.length, 0, "typing, render and validation must never request or change permissions");
        assert.equal(fixture.buttons.next.disabled, false, "normal Next starts the operation");
        assert.throws(() => fixture.values(), /Select Next/);
        fixture.model.browseFolder(row.id);
        const picker = fixture.pickers[0];
        assert.equal(picker.configuration.folderToolbar, false);
        picker.configuration.listeners.choose(picker, { fullpath: "/volume1/source", path: "/source" });
        assert.equal(row.source, "/volume1/source");
        assert.equal(row.name, "source");
        fixture.model.setAction("remove");
        assert.equal(fixture.model.getNext(), "confirmation");
        assert.equal(fixture.requests.length, 0);
        assert.equal(fixture.values().wizard_host_folders, "[]");
    }

    {
        const fixture = createFixture();
        addFolder(fixture, "/volume1/team/project", "project");
        addFolder(fixture, "/volume1/team/media", "media", "rw");
        assert.equal(fixture.model.getNext(), false, "false keeps DSM on this step while work runs");
        assert.equal(fixture.buttons.next.disabled, true);
        assert.equal(fixture.buttons.back.disabled, true, "guard Back after the native hook resets buttons");
        assert.equal(find(fixture.render(), node => node.tag === "fieldset").data.attrs.disabled, true);
        await fixture.settle();
        assert.equal(fixture.advances.length, 1);
        assert.equal(fixture.writes.length, 3);
        assert.deepEqual(fixture.writes.map(write => write.params.file_path), ["/volume1/team", "/volume1/team/project", "/volume1/team/media"]);
        const parent = fixture.storeFor("/volume1/team").acl.find(entry => entry.owner_name === "dockpipe-desktop");
        assert.deepEqual(parent.permission, permission({ exe_file: true }));
        assert.deepEqual(parent.inherit, { ...inheritance, all_descendants: false });
        const readOnly = fixture.storeFor("/volume1/team/project").acl.find(entry => entry.owner_name === "dockpipe-desktop");
        assert.equal(readOnly.permission.read_data, true);
        assert.equal(readOnly.permission.write_data, false);
        assert.equal(readOnly.inherit.child_files, true);
        const writable = fixture.storeFor("/volume1/team/media").acl.find(entry => entry.owner_name === "dockpipe-desktop");
        for (const key of ["read_data", "exe_file", "write_data", "append_data", "delete", "delete_sub", "write_attr", "write_ext_attr"])
            assert.equal(writable.permission[key], true, key);
        assert.equal(writable.permission.change_perm, false);
        assert.equal(writable.permission.take_ownership, false);
        for (const write of fixture.writes) {
            assert.equal(write.params.rules[0].owner_name, "another-user");
            assert.equal(write.params.rules[0].owner_id, 1028);
            assert.equal(write.params.rules[1].owner_name, "");
            assert.equal(write.params.rules[1].owner_id, 765432);
            assert.equal(write.params.rules[1].permission_type, "deny");
            assert.deepEqual(write.params.rules[1].permission, permission({ delete: true }));
            assert.equal(write.params.rules.some(entry => entry.owner_name === "inherited-user"), false);
        }
        fixture.advanceTime(300000);
        assert.equal(fixture.values().wizard_host_folder_action, "set", "proof remains valid on Confirmation after more than sixty seconds");
        fixture.model.setName(fixture.model.rows.value[0].id, "Renamed mapping");
        assert.equal(JSON.parse(fixture.values().wizard_host_folders)[0].name, "Renamed mapping");
        assert.equal(fixture.timers.size, 0);
        const tree = JSON.stringify(fixture.render());
        for (const label of ["Check access", "Review permissions", "Save applies", "permission editor"])
            assert(!tree.includes(label), label);
    }

    {
        const fixture = createFixture({
            packageOwnerId: 101884,
            internalMarkers: true,
            initializeACL(store, physical) {
                const traverse = rule("dockpipe-desktop", 101884, "allow", { exe_file: true }, { ...inheritance, all_descendants: false }, physical === "/volume1/team" ? 0 : 1);
                traverse.owner_is_internal = true;
                if (physical === "/volume1/team") {
                    store.acl.push(traverse);
                } else {
                    const administrator = rule("administrators", 101, "allow", { read_data: true }, inheritance, 1);
                    administrator.owner_type = "group";
                    store.acl = [rule("deny-user", 1026, "deny", { delete: true }, inheritance, 1), administrator, traverse];
                }
            }
        });
        const inheritedBefore = clone(fixture.storeFor("/volume1/team/project").acl);
        assert.equal(inheritedBefore.length, 3);
        assert(inheritedBefore.every(entry => entry.level === 1));
        assert.equal(inheritedBefore[2].owner_is_internal, true);
        addFolder(fixture, "/volume1/team/project", undefined, "rw");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1, "accept the live eight-field inherited system-user rule");
        assert.equal(fixture.writes.length, 1, "an existing parent traversal grant needs no update");
        const update = fixture.writes[0].params;
        assert.equal(update.file_path, "/volume1/team/project");
        assert.equal(update.inherited, true);
        assert.equal(update.rules.length, 1, "inherited entries are derived by DSM, not copied into explicit rules");
        assert.equal(update.rules[0].owner_id, undefined);
        assert.equal(update.rules[0].owner_is_internal, undefined);
        const after = fixture.storeFor("/volume1/team/project").acl;
        assert.deepEqual(after.filter(entry => entry.level === 1), inheritedBefore);
        const explicit = after.find(entry => entry.level === 0);
        assert.equal(explicit.owner_id, 101884);
        assert.equal(explicit.owner_is_internal, true, "accept the marker returned after set as well");
        assert.equal(explicit.permission.write_data, true);
    }

    {
        const fixture = createFixture({
            normalizeInheritance: true,
            internalMarkers: true,
            initializeACL(store) { store.acl[0].owner_is_internal = false; },
            beforeRead(store, physical, count) {
                if (count === 2) {
                    for (const entry of store.acl) {
                        if (!entry.inherit.child_files && !entry.inherit.child_folders)
                            entry.inherit.all_descendants = false;
                    }
                }
            }
        });
        addFolder(fixture, "/volume1/team/project");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1, "DSM may canonicalize the unused descendant flag on any childless ACE");
        assert.equal(fixture.writes.length, 2);
        assert.equal(fixture.writes[0].params.rules[0].inherit.all_descendants, true, "retain the original raw flags in existing update payloads");
        assert.equal(fixture.writes[0].params.rules.at(-1).inherit.all_descendants, false, "new ancestor grants use DSM's canonical this-folder-only form");
        assert.equal(fixture.writes[1].params.rules.at(-1).inherit.all_descendants, true, "selected target grants retain descendant inheritance");
    }

    for (const marker of ["true", 1, null, {}, []]) {
        const fixture = createFixture({ initializeACL(store) { store.acl[0].owner_is_internal = marker; } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0);
        assert.equal(fixture.advances.length, 0);
        assert.match(fixture.model.operationFailure.value, /unsupported or malformed ACL rule/);
    }

    {
        const fixture = createFixture({ initializeACL(store) {
            store.acl[0].owner_is_internal = true;
            store.acl[0].unrecognized_field = true;
        } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0);
        assert.match(fixture.model.operationFailure.value, /unsupported or malformed ACL rule/);
    }

    {
        const fixture = createFixture({ beforeRead(store, physical, count) {
            if (count === 2)
                store.acl[0].inherit.child_files = true;
        } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0, "semantic comparison must still detect meaningful inheritance changes");
        assert.match(fixture.model.operationFailure.value, /permissions changed/);
    }

    {
        const fixture = createFixture({ allowed: true });
        addFolder(fixture, "/volume1/team/project");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0);
        assert.equal(fixture.requests.length, 4, "inspect both paths initially and again before advancing");
        assert.equal(fixture.advances.length, 1);
    }

    for (const options of [
        { selfDenied: true },
        { metadata(data) { data.files[0].additional.real_path = "/volume2/different"; return data; } },
        { beforeRead(store, physical, count) { if (count === 2) store.acl.push(rule("concurrent-user", 9999)); } },
        { initializeACL(store) { store.acl[0].unexpected = true; } },
        { initializeACL(store) { store.acl_editable = false; } },
        { initializeACL(store) { store.acl.push(rule("dockpipe-desktop", 280000, "deny", { read_data: true })); } },
        { initializeACL(store) { store.acl.push(rule("dockpipe-desktop", 280000, "allow", { write_data: true })); } },
        { initializeACL(store) { store.acl.push(rule("DOCKPIPE-DESKTOP", 280000, "allow", { write_data: true })); } },
        { initializeACL(store) { store.acl.push(...Array.from({length: 513}, (_, index) => rule(`u${index}`, index))); } }
    ]) {
        const fixture = createFixture(options);
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0, "conflicts, denies and unsupported metadata must stop before set");
        assert.equal(fixture.advances.length, 0);
        assert(fixture.model.operationFailure.value);
        assert.equal(fixture.buttons.next.disabled, false, "failure must leave normal Next available");
    }

    {
        let failOnce = true;
        const fixture = createFixture({ intercept(request, backend) {
            if (request.configuration.method === "set" && failOnce) {
                failOnce = false;
                backend.applyWrite(request.configuration);
                request.respond(false, { private: "diagnostic must not be shown" });
                return true;
            }
            return false;
        } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 1);
        assert.equal(fixture.advances.length, 0);
        assert.match(fixture.model.operationFailure.value, /already sent may have completed/);
        assert(!fixture.model.operationFailure.value.includes("diagnostic"));
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 1, "a later explicit Next must inspect an uncertain prior write, not replay it");
        assert.equal(fixture.advances.length, 1);
    }

    {
        const fixture = createFixture({ afterSet(store) { store.acl = store.acl.filter(entry => entry.owner_name !== "another-user"); } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 1);
        assert.equal(fixture.advances.length, 0);
        assert.match(fixture.model.operationFailure.value, /did not match.*No rollback/);
        assert.throws(() => fixture.values(), /Select Next/);
    }

    {
        const fixture = createFixture({
            initializeACL(store) {
                store.acl.push(rule("dockpipe-desktop", 280000, "allow", { read_data: true, exe_file: true }, {
                    this_folder: true, child_files: true, child_folders: true, all_descendants: true
                }));
            }
        });
        addFolder(fixture, "/volume1/projects", undefined, "rw");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1);
        const allows = fixture.storeFor("/volume1/projects").acl.filter(entry => entry.owner_name === "dockpipe-desktop");
        assert.equal(allows.length, 1, "merge only a matching explicit allow with the same inheritance scope");
        assert.equal(allows[0].owner_id, 280000);
        assert.equal(allows[0].permission.write_data, true);
    }

    for (const cancellation of ["edit", "unmount", "navigate"]) {
        const fixture = createFixture({ intercept(request) { return request.configuration.method === "get"; } });
        const row = addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.flush();
        const pendingRead = fixture.requests.at(-1);
        assert.equal(pendingRead.configuration.method, "get");
        if (cancellation === "edit")
            fixture.model.setSource(row.id, "/volume1/other");
        else if (cancellation === "unmount")
            fixture.unmount();
        else
            fixture.leaveStep();
        pendingRead.completeDefault();
        await fixture.settle();
        assert.equal(fixture.writes.length, 0);
        assert.equal(fixture.advances.length, 0);
        assert.equal(fixture.timers.size, 0);
    }

    {
        const fixture = createFixture({ intercept(request) { return request.configuration.method === "set"; } });
        const row = addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.flush();
        const sent = fixture.requests.at(-1);
        assert.equal(sent.configuration.method, "set");
        const count = fixture.requests.length;
        fixture.model.setAccess(row.id, "rw");
        sent.completeDefault();
        await fixture.settle();
        assert.equal(fixture.requests.length, count, "cancellation must prevent every future write or read in the old plan");
        assert.equal(fixture.advances.length, 0);
        assert.match(fixture.model.operationFailure.value, /already sent may have completed/);
    }

    for (const deadline of [5000, 120000]) {
        const fixture = createFixture({ intercept() { return true; } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.flush();
        fixture.fireTimers(deadline);
        await fixture.settle();
        assert.equal(fixture.requests.length, 1, "do not retry after timeout");
        assert.equal(fixture.requests[0].aborted, true);
        assert.equal(fixture.writes.length, 0);
        assert.equal(fixture.advances.length, 0);
        assert.equal(fixture.timers.size, 0);
    }

    {
        const fixture = createFixture({ intercept(request) {
            if (request.configuration.method === "set") {
                request.completeDefault();
                return true;
            }
            return false;
        }, effective(physical, rights, requests) {
            const count = requests.filter(request => request.configuration.method === "inspect" && request.configuration.params.file_path === physical).length;
            if (count >= 3)
                rights.exe_file = false;
            return rights;
        } });
        addFolder(fixture, "/volume1/projects");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.writes.length, 1);
        assert.equal(fixture.advances.length, 0, "the final whole-plan inspection must not reuse a pre-write verdict");
    }

    {
        const fixture = createFixture();
        addFolder(fixture, "/volume1/team" + "/a".repeat(128));
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.requests.length, 0);
        assert.match(fixture.model.operationFailure.value, /128/);
    }

    {
        const fixture = createFixture();
        const body = { clientHeight: 401, clientTop: 2, scrollTop: 158, paddingBottom: "12px", getBoundingClientRect() { return { top: 100 }; } };
        const scaffold = {};
        const element = {
            parentElement: scaffold, offset: 80,
            closest(selector) { assert.equal(selector, ".mcontentwrapper"); return body; },
            getBoundingClientRect() { return { top: 102 + this.offset - body.scrollTop, width: 600 }; }
        };
        fixture.mount(element);
        assert.equal(fixture.model.scrollHeight.value, 309);
        fixture.flushLayout();
        assert.equal(body.scrollTop, 0);
        assert.equal(fixture.render().children[0].data.style.overflowY, "auto");
        body.clientHeight = 250;
        element.offset = 92;
        fixture.observers[0].callback();
        assert.equal(fixture.model.scrollHeight.value, 146);
        fixture.unmount();
        assert.equal(fixture.observers[0].disconnected, true);
    }

    {
        const fixture = createFixture({ allowed: true });
        const row = addFolder(fixture, "/volume1/projects", "日本語");
        for (const name of ["", ".", "..", "a/b", "a\\b", "x".repeat(129), "é".repeat(65)]) {
            fixture.model.setName(row.id, name);
            assert.equal(fixture.model.getNext(), false);
            assert.equal(fixture.requests.length, 0);
        }
        fixture.model.setName(row.id, "@valid name");
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.advances.length, 1);
    }

    {
        const fixture = createFixture({ allowed: true });
        for (let index = 1; index <= 16; index++)
            addFolder(fixture, `/volume${index}/share${index}/a/b/c/d/e/f/g`, `folder-${index}`);
        fixture.model.addRow();
        assert.equal(fixture.model.rows.value.length, 16);
        fixture.model.getNext();
        await fixture.settle();
        assert.equal(fixture.requests.length, 256, "128 distinct prefixes are inspected initially and finally");
        assert.equal(fixture.writes.length, 0);
        assert.equal(fixture.advances.length, 1);
    }

    {
        const fixture = createFixture();
        for (let index = 1; index <= 16; index++) {
            const row = addFolder(fixture, `/volume${index}/share${index}/leaf`, `folder-${index}`);
            fixture.model.setSource(row.id, (`/volume${index}/share${index}/` + ("x".repeat(200) + "/").repeat(19) + "leaf").padEnd(4096, "x"));
        }
        assert.equal(fixture.model.getNext(), false);
        assert.equal(fixture.model.preparingAccess.value, false);
        assert.equal(fixture.requests.length, 0);
        assert.throws(() => fixture.values(), /64 KiB/);
    }

    console.log("Host-folder automatic Next grants, ACL preservation, conflicts, cancellation, confirmation proof and layout passed.");
}

main().catch(error => {
    console.error(error);
    process.exitCode = 1;
});
