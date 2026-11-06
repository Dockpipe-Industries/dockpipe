/* DSM 7.2.2+ JsonpLoader module; uses the installed Vue 2.7 runtime. */
(function() {
    "use strict";

    var maximumFolders = 16;
    var maximumJSONBytes = 64 * 1024;
    var maximumInspectPaths = 128;
    var inspectTimeoutMilliseconds = 5000;
    var folderOperationTimeoutMilliseconds = 120000;
    var maximumACLBytes = 256 * 1024;
    var maximumACLRules = 512;
    var maximumFolderRequests = maximumInspectPaths * 10;
    var permissionKeys = ["read_data", "write_data", "exe_file", "append_data", "delete", "delete_sub",
        "read_attr", "write_attr", "read_ext_attr", "write_ext_attr", "read_perm", "change_perm", "take_ownership"];
    var inheritanceKeys = ["this_folder", "child_files", "child_folders", "all_descendants"];
    var packageAccount = "dockpipe-desktop";
    var encoder = new TextEncoder();
    var framework = SYNO.SDS.PkgManApp.Custom;
    var restoreMode = "@DOCKPIPE_RESTORE_MODE@";
    var retainedVolumes = volumeList("@DOCKPIPE_RESTORE_VOLUMES@");
    var checkedVolumes = volumeList("@DOCKPIPE_CHECKED_VOLUMES@");
    var unknownVolumes = volumeList("@DOCKPIPE_UNKNOWN_VOLUMES@");

    function volumeList(value) {
        if (value === "")
            return [];
        var volumes = value.split(",");
        if (volumes.length > 64 || new Set(volumes).size !== volumes.length
            || !volumes.every(function(volume) { return /^\/volume[1-9][0-9]*$/.test(volume); }))
            return null;
        return volumes;
    }

    function restoreDecision(owner) {
        var volume = owner && owner.installVolume;
        // DSM fixes the existing volume during repair and does not expose a
        // selection on the upgrade wizard. The native installer validates its
        // authoritative destination and retained settings before preparation.
        if (restoreMode === "upgrade" && owner && volume === undefined)
            return { volume: "", intent: "upgrade" };
        if (typeof volume !== "string" || !/^\/volume[1-9][0-9]*$/.test(volume))
            return { error: "Choose a valid installation volume in DSM before continuing." };
        if (restoreMode === "upgrade")
            return { volume: volume, intent: "upgrade" };
        if (!retainedVolumes || !checkedVolumes || !unknownVolumes || restoreMode === "unknown")
            return { error: "DSM could not confirm saved Dockpipe settings for the installation volume. Existing data has not been reset. Reopen Package Center and verify the selected volume before continuing." };
        if (restoreMode === "restore" && retainedVolumes.includes(volume))
            return { volume: volume, intent: "restore" };
        if (restoreMode === "fresh" && retainedVolumes.length === 0 && unknownVolumes.length === 0 && checkedVolumes.includes(volume))
            return { volume: volume, intent: "fresh" };
        if (retainedVolumes.length > 0)
            return { error: "Saved Dockpipe data was found on " + retainedVolumes.join(", ")
                + ". Select one of those installation volumes in DSM to retain its sign-in and workspace." };
        return { error: "Saved Dockpipe settings could not be verified on " + volume
            + ". Reopen Package Center with the intended installation volume; no existing data has been reset." };
    }

    function byteLength(value) {
        return encoder.encode(value).length;
    }

    function folderName(source) {
        var trimmed = source.replace(/\/+$/, "");
        return trimmed.slice(trimmed.lastIndexOf("/") + 1);
    }

    function validSource(source) {
        if (byteLength(source) > 4096 || /[\x00-\x1f\x7f]/.test(source))
            return false;
        var parts = source.split("/");
        return parts.length >= 3 && parts[0] === "" && /^volume[1-9][0-9]*$/.test(parts[1])
            && parts.slice(2).every(function(part) {
                return part !== "" && part !== "." && part !== ".." && part[0] !== "@";
            });
    }

    function validName(name) {
        return byteLength(name) >= 1 && byteLength(name) <= 128
            && name !== "." && name !== ".." && !/[\/\\\x00-\x1f\x7f]/.test(name);
    }

    var component = Vue.defineComponent({
        props: Object.assign({}, framework.useHook.props),

        setup: function(props) {
            var hooks = framework.useHook(props);
            var action = Vue.ref("keep");
            var rows = Vue.ref([]);
            var validationError = Vue.ref("");
            var pickerError = Vue.ref("");
            var pickingRow = Vue.ref(null);
            var preparingAccess = Vue.ref(false);
            var operationMessage = Vue.ref("");
            var operationFailure = Vue.ref("");
            var activeFolderOperation = null;
            var verifiedSelection = null;
            var layoutRoot = Vue.ref(null);
            var scrollHeight = Vue.ref(0);
            var layoutObserver = null;
            var layoutResizeListener = null;
            var activePicker = null;
            var disposed = false;
            var nextIdentifier = 1;

            function fitWizardBody() {
                var element = layoutRoot.value;
                if (disposed || !element)
                    return;
                var body = element.closest(".mcontentwrapper");
                if (!body || body.clientHeight <= 0)
                    return;
                var elementBounds = element.getBoundingClientRect();
                if (elementBounds.width <= 0)
                    return;
                var bodyBounds = body.getBoundingClientRect();
                var bottomPadding = parseFloat(window.getComputedStyle(body).paddingBottom) || 0;
                // Focus can scroll DSM's overflow-hidden body. Measure from its
                // unscrolled origin, including the native headline and padding.
                var offset = elementBounds.top - bodyBounds.top - body.clientTop + body.scrollTop;
                var available = Math.max(0, Math.floor(body.clientHeight - Math.max(0, offset) - bottomPadding));
                if (scrollHeight.value !== available)
                    scrollHeight.value = available;
                Vue.nextTick(function() {
                    if (!disposed && layoutRoot.value === element)
                        body.scrollTop = 0;
                });
            }

            Vue.onMounted(function() {
                var element = layoutRoot.value;
                if (!element)
                    return;
                fitWizardBody();
                var body = element.closest(".mcontentwrapper");
                if (typeof ResizeObserver === "function" && body) {
                    layoutObserver = new ResizeObserver(fitWizardBody);
                    layoutObserver.observe(body);
                    layoutObserver.observe(element.parentElement);
                } else {
                    layoutResizeListener = fitWizardBody;
                    window.addEventListener("resize", layoutResizeListener);
                }
            });

            function projectedRows() {
                return rows.value.map(function(row) {
                    return { source: row.source, name: row.name, access: row.access };
                });
            }

            function errorMessage() {
                if (action.value !== "keep" && action.value !== "set" && action.value !== "remove")
                    return "Choose whether to keep, replace or remove the folder mappings.";
                if (action.value !== "set")
                    return "";
                if (rows.value.length === 0 || rows.value.length > maximumFolders)
                    return "Add between 1 and 16 folders, or choose Remove all folder mappings.";
                var names = new Set();
                for (var index = 0; index < rows.value.length; index++) {
                    var row = rows.value[index];
                    var label = "Folder " + (index + 1) + ": ";
                    if (!validSource(row.source))
                        return label + "enter an existing absolute NAS path under /volumeN/share, without system directories or dot segments.";
                    if (!validName(row.name))
                        return label + "enter a name of 1–128 UTF-8 bytes, without slashes, backslashes, control characters, or the names \".\" or \"..\".";
                    if (names.has(row.name))
                        return label + "choose a unique name under /host. Names are case-sensitive.";
                    names.add(row.name);
                    if (row.access !== "ro" && row.access !== "rw")
                        return label + "choose Read only or Read and write.";
                }
                // Include the persisted envelope and Go JSON's HTML/line-separator
                // escaping so a valid wizard list also fits the installer budget.
                var manifest = JSON.stringify({
                    schema: "dockpipe.synology_host_folders.v2", folders: projectedRows()
                }).replace(/[<>&\u2028\u2029]/g, function(character) {
                    return "\\u" + character.charCodeAt(0).toString(16).padStart(4, "0");
                });
                if (byteLength(manifest) > maximumJSONBytes)
                    return "The folder mappings exceed the 64 KiB limit. Shorten paths or remove folders.";
                return "";
            }

            function checkState(owner) {
                owner = owner || props.getOwner();
                hooks.checkState(owner);
                validationError.value = restoreDecision(owner).error || errorMessage();
                owner.getButton("next").setDisabled(validationError.value !== "" || preparingAccess.value);
                if (preparingAccess.value && activeFolderOperation && activeFolderOperation.backButton)
                    activeFolderOperation.backButton.setDisabled(true);
            }

            function selectionScope() {
                return JSON.stringify({
                    volume: props.getOwner().installVolume,
                    folders: rows.value.map(function(row) { return { source: row.source, access: row.access }; })
                });
            }

            function invalidateRow(row) {
                row.permissionState = "unchecked";
                row.permissionMessage = "Access will be prepared when you continue.";
            }

            function cancelledFailure() {
                return new Error("folder_operation_cancelled");
            }

            function assertCurrent(context) {
                if (disposed || activeFolderOperation !== context || context.cancelled
                    || action.value !== "set" || selectionScope() !== context.scope
                    || context.owner.getActiveStep() !== context.step || Date.now() >= context.deadline)
                    throw cancelledFailure();
            }

            function cancelFolderOperation() {
                verifiedSelection = null;
                var context = activeFolderOperation;
                if (!context)
                    return;
                context.cancelled = true;
                for (var operation of Array.from(context.operations))
                    operation.abort();
            }

            function requestFolderAPI(context, api, method, version, parameters, mutation) {
                assertCurrent(context);
                if (++context.requestCount > maximumFolderRequests)
                    throw new Error("folder_request_budget");
                return new Promise(function(resolve, reject) {
                    var request = null;
                    var timer = null;
                    var settled = false;
                    function finish(error, data) {
                        if (settled)
                            return;
                        settled = true;
                        clearTimeout(timer);
                        context.operations.delete(operation);
                        if (error)
                            reject(error);
                        else
                            resolve(data);
                    }
                    var operation = {
                        abort: function() {
                            finish(cancelledFailure());
                            if (request && typeof request.abort === "function") {
                                try {
                                    request.abort();
                                } catch (error) {
                                    // Cancellation cannot undo a write already sent.
                                }
                            }
                        }
                    };
                    context.operations.add(operation);
                    timer = setTimeout(function() {
                        finish(new Error("folder_request_timeout"));
                        operation.abort();
                    }, inspectTimeoutMilliseconds);
                    try {
                        if (mutation)
                            context.possiblyChanged = true;
                        request = SYNO.API.Request({
                            api: api, method: method, version: version, params: parameters,
                            timeout: inspectTimeoutMilliseconds, suppressError: true,
                            callback: function(success, data) {
                                if (success !== true) {
                                    finish(new Error("folder_api_failed"));
                                    return;
                                }
                                try {
                                    assertCurrent(context);
                                    if (byteLength(JSON.stringify(data)) > maximumACLBytes)
                                        throw new Error("folder_response_budget");
                                    finish(null, data);
                                } catch (error) {
                                    finish(error);
                                }
                            }
                        });
                        if (!settled && (!request || typeof request.abort !== "function"))
                            finish(new Error("folder_api_unavailable"));
                    } catch (error) {
                        finish(new Error("folder_api_unavailable"));
                    }
                });
            }

            function inspectionPlan() {
                var paths = new Map();
                for (var row of rows.value) {
                    var parts = row.source.split("/");
                    if (parts.length - 2 > maximumInspectPaths)
                        throw new Error("folder_path_budget");
                    for (var length = 3; length <= parts.length; length++) {
                        var path = parts.slice(0, length).join("/");
                        var item = paths.get(path);
                        if (!item) {
                            if (paths.size >= maximumInspectPaths)
                                throw new Error("folder_path_budget");
                            item = { path: path, target: false, access: "ro" };
                            paths.set(path, item);
                        }
                        if (path === row.source) {
                            item.target = true;
                            if (row.access === "rw")
                                item.access = "rw";
                        }
                    }
                }
                return Array.from(paths.values()).sort(function(left, right) {
                    return left.path.split("/").length - right.path.split("/").length;
                });
            }

            async function inspectFolder(context, item) {
                var data = await requestFolderAPI(context, "SYNO.Core.ACL", "inspect", 1, {
                    file_path: item.path, owner_type: "user", owner_name: packageAccount
                }, false);
                var permission = data && data.permission;
                if (!permission || !["read_data", "exe_file", "write_data", "append_data"].every(function(key) {
                    return typeof permission[key] === "boolean";
                }))
                    throw new Error("folder_inspection_invalid");
                return permission.exe_file && (!item.target || (permission.read_data
                    && (item.access !== "rw" || (permission.write_data && permission.append_data))));
            }

            function copyBooleans(value, keys) {
                if (!value || typeof value !== "object" || Array.isArray(value)
                    || Object.keys(value).length !== keys.length)
                    throw new Error("folder_acl_invalid");
                var copy = {};
                for (var key of keys) {
                    if (typeof value[key] !== "boolean")
                        throw new Error("folder_acl_invalid");
                    copy[key] = value[key];
                }
                return copy;
            }

            function readACLData(data) {
                if (!data || data.acl_editable !== true || data.change_permission !== true
                    || data.is_acl !== true || typeof data.is_inherited !== "boolean"
                    || !Array.isArray(data.acl) || data.acl.length > maximumACLRules)
                    throw new Error("folder_acl_not_editable");
                var rules = data.acl.map(function(rule) {
                    var fields = ["level", "owner_id", "owner_name", "owner_type", "permission_type", "permission", "inherit"];
                    var allowedFields = fields.concat("owner_is_internal");
                    if (!rule || !Object.keys(rule).every(function(key) { return allowedFields.includes(key); }) || !fields.every(function(key) {
                        return Object.prototype.hasOwnProperty.call(rule, key);
                    }) || !Number.isSafeInteger(rule.level) || rule.level < 0
                        || !Number.isSafeInteger(rule.owner_id) || rule.owner_id < 0
                        || typeof rule.owner_name !== "string" || byteLength(rule.owner_name) > 1024
                        || typeof rule.owner_type !== "string" || rule.owner_type.length === 0 || rule.owner_type.length > 128
                        || (Object.prototype.hasOwnProperty.call(rule, "owner_is_internal") && typeof rule.owner_is_internal !== "boolean")
                        || (rule.permission_type !== "allow" && rule.permission_type !== "deny"))
                        throw new Error("folder_acl_invalid");
                    // owner_is_internal is DSM's optional read-only identity hint.
                    // The native collector omits it from updates; owner identity,
                    // permission bits and inheritance remain explicit below.
                    return {
                        level: rule.level, owner_id: rule.owner_id, owner_name: rule.owner_name,
                        owner_type: rule.owner_type, permission_type: rule.permission_type,
                        permission: copyBooleans(rule.permission, permissionKeys),
                        inherit: copyBooleans(rule.inherit, inheritanceKeys)
                    };
                });
                return { rules: rules, inherited: data.is_inherited };
            }

            async function readFolderACL(context, path) {
                var data = await requestFolderAPI(context, "SYNO.Core.ACL", "get", 1, {
                    file_path: path, include_noname_rules: true, type: "all"
                }, false);
                return readACLData(data);
            }

            function aclSignature(snapshot) {
                return JSON.stringify({
                    inherited: snapshot.inherited,
                    rules: snapshot.rules.map(ruleSignature).sort()
                });
            }

            function semanticInheritance(inherit) {
                var normalized = Object.assign({}, inherit);
                if (!normalized.child_files && !normalized.child_folders)
                    normalized.all_descendants = false;
                return normalized;
            }

            function ruleSignature(rule) {
                return JSON.stringify(Object.assign({}, rule, { inherit: semanticInheritance(rule.inherit) }));
            }

            function sameInheritance(left, right) {
                return JSON.stringify(semanticInheritance(left)) === JSON.stringify(semanticInheritance(right));
            }

            function requestedRule(item) {
                var permission = {};
                permissionKeys.forEach(function(key) { permission[key] = false; });
                permission.exe_file = true;
                if (item.target) {
                    ["read_data", "read_attr", "read_ext_attr", "read_perm"].forEach(function(key) { permission[key] = true; });
                    if (item.access === "rw") {
                        ["write_data", "append_data", "delete", "delete_sub", "write_attr", "write_ext_attr"].forEach(function(key) {
                            permission[key] = true;
                        });
                    }
                }
                return {
                    owner_name: packageAccount, owner_type: "user", permission_type: "allow", permission: permission,
                    inherit: { this_folder: true, child_files: item.target, child_folders: item.target, all_descendants: item.target }
                };
            }

            function additiveRules(snapshot, item) {
                var desired = requestedRule(item);
                var rules = snapshot.rules.filter(function(rule) { return rule.level === 0; }).map(function(rule) {
                    return {
                        owner_id: rule.owner_id, owner_name: rule.owner_name, owner_type: rule.owner_type,
                        permission_type: rule.permission_type, permission: Object.assign({}, rule.permission),
                        inherit: Object.assign({}, rule.inherit)
                    };
                });
                var matching = rules.filter(function(rule) {
                    return rule.owner_name.toLowerCase() === packageAccount && rule.owner_type === "user" && rule.permission_type === "allow";
                });
                var blocked = rules.some(function(rule) {
                    return rule.owner_name.toLowerCase() === packageAccount && rule.owner_type === "user"
                        && rule.permission_type === "deny" && rule.inherit.this_folder
                        && permissionKeys.some(function(key) { return desired.permission[key] && rule.permission[key]; });
                });
                if (blocked)
                    throw new Error("folder_access_denied");
                var sameScope = matching.find(function(rule) { return sameInheritance(rule.inherit, desired.inherit); });
                if (sameScope) {
                    var changes = permissionKeys.some(function(key) { return desired.permission[key] && !sameScope.permission[key]; });
                    if (!changes)
                        throw new Error("folder_access_denied");
                    permissionKeys.forEach(function(key) { sameScope.permission[key] = sameScope.permission[key] || desired.permission[key]; });
                } else {
                    if (matching.length > 0)
                        throw new Error("folder_acl_scope_conflict");
                    if (snapshot.rules.length >= maximumACLRules)
                        throw new Error("folder_acl_rule_budget");
                    rules.push(desired);
                }
                return rules;
            }

            function ruleMatches(actual, expected) {
                return (expected.owner_id === undefined || actual.owner_id === expected.owner_id)
                    && actual.owner_name === expected.owner_name && actual.owner_type === expected.owner_type
                    && actual.permission_type === expected.permission_type
                    && JSON.stringify(actual.permission) === JSON.stringify(expected.permission)
                    && sameInheritance(actual.inherit, expected.inherit);
            }

            function verifyUpdatedACL(before, after, expected) {
                if (before.inherited !== after.inherited)
                    throw new Error("folder_acl_verification_failed");
                var inheritedBefore = before.rules.filter(function(rule) { return rule.level > 0; });
                var inheritedAfter = after.rules.filter(function(rule) { return rule.level > 0; });
                if (JSON.stringify(inheritedBefore.map(ruleSignature).sort())
                    !== JSON.stringify(inheritedAfter.map(ruleSignature).sort()))
                    throw new Error("folder_acl_verification_failed");
                var explicit = after.rules.filter(function(rule) { return rule.level === 0; });
                if (explicit.length !== expected.length)
                    throw new Error("folder_acl_verification_failed");
                for (var rule of expected) {
                    var index = explicit.findIndex(function(candidate) { return ruleMatches(candidate, rule); });
                    if (index < 0)
                        throw new Error("folder_acl_verification_failed");
                    explicit.splice(index, 1);
                }
            }

            async function grantFolder(context, item) {
                var virtualPath = "/" + item.path.split("/").slice(2).join("/");
                var metadata = await requestFolderAPI(context, "SYNO.FileStation.List", "getinfo", 2, {
                    path: [virtualPath], additional: ["real_path", "perm"]
                }, false);
                if (!metadata || !Array.isArray(metadata.files) || metadata.files.length !== 1
                    || !metadata.files[0] || metadata.files[0].isdir !== true || metadata.files[0].path !== virtualPath
                    || !metadata.files[0].additional || metadata.files[0].additional.real_path !== item.path)
                    throw new Error("folder_identity_unverified");
                var before = await readFolderACL(context, item.path);
                var rules = additiveRules(before, item);
                var parameters = {
                    file_path: item.path, files: item.path, dirPaths: virtualPath,
                    change_acl: true, rules: rules, inherited: before.inherited, acl_recur: false
                };
                var selfDenied = await requestFolderAPI(context, "SYNO.Core.ACL", "check_self_denied", 1, parameters, false);
                if (!selfDenied || selfDenied.is_denied !== false)
                    throw new Error("folder_acl_self_denied");
                var current = await readFolderACL(context, item.path);
                if (aclSignature(current) !== aclSignature(before))
                    throw new Error("folder_acl_conflict");
                assertCurrent(context);
                await requestFolderAPI(context, "SYNO.Core.ACL", "set", 1, parameters, true);
                var after = await readFolderACL(context, item.path);
                verifyUpdatedACL(before, after, rules);
                if (!await inspectFolder(context, item))
                    throw new Error("folder_access_denied");
            }

            function operationError(context, error) {
                var messages = {
                    folder_path_budget: "The selected folders exceed the 128 folder and parent checks that can be prepared in one operation.",
                    folder_identity_unverified: "DSM could not verify the selected folder's physical location.",
                    folder_acl_conflict: "The folder's permissions changed while access was being prepared. Existing changes were not overwritten.",
                    folder_acl_scope_conflict: "The folder has an existing Dockpipe allow rule with a different scope. It cannot be expanded safely automatically.",
                    folder_acl_self_denied: "DSM rejected the additional rule because it would deny your own access.",
                    folder_acl_not_editable: "DSM does not allow this folder's ACL permissions to be edited.",
                    folder_acl_invalid: "DSM returned an unsupported or malformed ACL rule, so folder access could not be verified safely.",
                    folder_request_timeout: "DSM timed out while preparing folder access. No automatic retry was performed.",
                    folder_acl_verification_failed: "DSM's saved permissions did not match the additive update. No rollback was attempted.",
                    folder_access_denied: "DSM still denies the requested access. Existing deny rules have been preserved.",
                    folder_operation_cancelled: "Folder access preparation stopped before verification completed."
                };
                var message = messages[error && error.message] || "DSM could not safely prepare and verify the selected folder access.";
                if (context.path)
                    message = context.path + ": " + message;
                if (context.possiblyChanged)
                    message += " Permission updates already sent may have completed. Existing updates were not rolled back; another Next attempt will inspect current access first.";
                return message;
            }

            function releaseFolderOperation(context) {
                clearTimeout(context.timer);
                if (activeFolderOperation !== context)
                    return;
                activeFolderOperation = null;
                preparingAccess.value = false;
                if (!disposed && context.owner.getActiveStep() === context.step && context.backButton)
                    context.backButton.setDisabled(context.backDisabled);
            }

            function startFolderOperation(owner, step) {
                var context = {
                    owner: owner, step: step, scope: selectionScope(), operations: new Set(),
                    cancelled: false, possiblyChanged: false, requestCount: 0, path: "",
                    timer: null, deadline: Date.now() + folderOperationTimeoutMilliseconds,
                    backButton: owner.getButton("back"), backDisabled: false
                };
                if (context.backButton) {
                    context.backDisabled = context.backButton.disabled === true;
                    context.backButton.setDisabled(true);
                }
                activeFolderOperation = context;
                verifiedSelection = null;
                preparingAccess.value = true;
                operationFailure.value = "";
                operationMessage.value = "Preparing the selected folder access…";
                rows.value.forEach(function(row) {
                    row.permissionState = "pending";
                    row.permissionMessage = "Preparing access to " + row.source + "…";
                });
                context.timer = setTimeout(function() {
                    context.cancelled = true;
                    for (var operation of Array.from(context.operations))
                        operation.abort();
                }, folderOperationTimeoutMilliseconds);
                checkState(owner);
                Promise.resolve().then(async function() {
                    var plan = inspectionPlan();
                    for (var item of plan) {
                        assertCurrent(context);
                        context.path = item.path;
                        if (!await inspectFolder(context, item))
                            await grantFolder(context, item);
                    }
                    // Recheck the complete selection, including ancestor traversal,
                    // after all additions. No cached verdict advances the wizard.
                    for (var item of plan) {
                        assertCurrent(context);
                        context.path = item.path;
                        if (!await inspectFolder(context, item))
                            throw new Error("folder_access_denied");
                    }
                    assertCurrent(context);
                    verifiedSelection = context.scope;
                    rows.value.forEach(function(row) {
                        row.permissionState = "allowed";
                        row.permissionMessage = "Access verified for " + row.source + ".";
                    });
                    operationMessage.value = "Folder access verified.";
                    releaseFolderOperation(context);
                    checkState(owner);
                    if (!disposed && owner.getActiveStep() === step)
                        owner.goNext(hooks.getNext());
                }).catch(function(error) {
                    verifiedSelection = null;
                    releaseFolderOperation(context);
                    if (disposed || owner.getActiveStep() !== step)
                        return;
                    operationMessage.value = "";
                    operationFailure.value = operationError(context, error);
                    rows.value.forEach(function(row) {
                        row.permissionState = "unavailable";
                        row.permissionMessage = "Access was not verified. Continue again after resolving the reported problem.";
                    });
                    checkState(owner);
                });
            }

            function getNext() {
                if (disposed || preparingAccess.value || restoreDecision(props.getOwner()).error || errorMessage())
                    return false;
                if (action.value !== "set")
                    return hooks.getNext();
                var owner = props.getOwner();
                if (typeof owner.getActiveStep !== "function" || typeof owner.goNext !== "function") {
                    operationFailure.value = "DSM's wizard navigation is unavailable. Reopen Package Center before continuing.";
                    return false;
                }
                var step = owner.getActiveStep();
                if (!step)
                    return false;
                startFolderOperation(owner, step);
                // DSM explicitly supports false as "stay on this step". The
                // guarded completion advances through the normal owner.goNext.
                return false;
            }

            function appendRow() {
                if (rows.value.length >= maximumFolders)
                    return;
                var row = { id: nextIdentifier++, source: "", name: "", access: "ro", nameEdited: false };
                invalidateRow(row);
                rows.value.push(row);
            }

            function setAction(value) {
                if (action.value !== value) {
                    cancelFolderOperation();
                }
                action.value = value;
                if (value === "set" && rows.value.length === 0)
                    appendRow();
                checkState();
            }

            function addRow() {
                cancelFolderOperation();
                action.value = "set";
                appendRow();
                checkState();
            }

            function removeRow(identifier) {
                cancelFolderOperation();
                rows.value = rows.value.filter(function(row) { return row.id !== identifier; });
                checkState();
            }

            function findRow(identifier) {
                return rows.value.find(function(row) { return row.id === identifier; });
            }

            function setSource(identifier, value) {
                var row = findRow(identifier);
                if (!row)
                    return;
                if (row.source !== value) {
                    cancelFolderOperation();
                    invalidateRow(row);
                }
                row.source = value;
                if (!row.nameEdited)
                    row.name = folderName(value);
                checkState();
            }

            function browseFolder(identifier) {
                if (activePicker || !findRow(identifier))
                    return;
                pickerError.value = "";
                var chooser = SYNO.SDS.Utils && SYNO.SDS.Utils.FileChooser
                    && SYNO.SDS.Utils.FileChooser.Chooser;
                if (typeof chooser !== "function") {
                    pickerError.value = "DSM's folder picker is unavailable. Reopen Package Center or enter the NAS path directly.";
                    return;
                }
                try {
                    var picker = new chooser({
                        owner: props.getOwner(),
                        usage: { type: "chooseDir" },
                        folderToolbar: false,
                        enumTieringShare: false,
                        enumTieringBucket: false,
                        listeners: {
                            choose: function(dialog, selection) {
                                // DSM's path is a share-relative display path.
                                // fullpath is the physical NAS path needed by Docker.
                                if (!selection || typeof selection.fullpath !== "string"
                                    || !validSource(selection.fullpath)
                                    || selection.is_in_c2share || selection.is_in_tiering_share) {
                                    pickerError.value = "Select a local NAS folder inside a shared folder.";
                                } else {
                                    setSource(identifier, selection.fullpath);
                                }
                                dialog.close();
                            },
                            hide: function() { releasePicker(picker); },
                            destroy: function() { releasePicker(picker); }
                        }
                    });
                    activePicker = picker;
                    pickingRow.value = identifier;
                    picker.show();
                } catch (error) {
                    var failedPicker = activePicker;
                    activePicker = null;
                    pickingRow.value = null;
                    if (failedPicker && typeof failedPicker.destroy === "function")
                        failedPicker.destroy();
                    pickerError.value = "DSM's folder picker could not open. Enter the NAS path directly or reopen Package Center.";
                }
            }

            function releasePicker(picker) {
                if (activePicker === picker) {
                    activePicker = null;
                    pickingRow.value = null;
                }
            }

            Vue.onBeforeUnmount(function() {
                disposed = true;
                if (layoutObserver)
                    layoutObserver.disconnect();
                if (layoutResizeListener)
                    window.removeEventListener("resize", layoutResizeListener);
                cancelFolderOperation();
                var picker = activePicker;
                activePicker = null;
                if (picker)
                    picker.close();
            });

            function setName(identifier, value) {
                var row = findRow(identifier);
                if (!row)
                    return;
                row.name = value;
                row.nameEdited = true;
                checkState();
            }

            function useFolderName(identifier) {
                var row = findRow(identifier);
                if (!row)
                    return;
                row.nameEdited = false;
                row.name = folderName(row.source);
                checkState();
            }

            function setAccess(identifier, value) {
                var row = findRow(identifier);
                if (!row)
                    return;
                if (row.access !== value) {
                    cancelFolderOperation();
                    invalidateRow(row);
                }
                row.access = value;
                checkState();
            }

            function getValues() {
                var restore = restoreDecision(props.getOwner());
                var message = restore.error || errorMessage();
                if (!message && action.value === "set" && (preparingAccess.value || verifiedSelection !== selectionScope()))
                    message = "Select Next on the folder step to prepare and verify this selection before continuing.";
                if (message)
                    throw new Error(message);
                // Package Center merges this map with Ext.apply. Returning an
                // array creates a numeric "0" field instead of script variables.
                return {
                    wizard_host_folder_action: action.value,
                    wizard_host_folders: action.value === "set" ? JSON.stringify(projectedRows()) : "[]",
                    wizard_restore_intent: restore.intent,
                    wizard_restore_volume: restore.volume
                };
            }

            Vue.watchEffect(function() { checkState(); });
            return {
                getNext: getNext,
                checkState: checkState,
                headline: "Add Host Folders",
                getValues: getValues,
                action: action,
                rows: rows,
                validationError: validationError,
                pickerError: pickerError,
                pickingRow: pickingRow,
                preparingAccess: preparingAccess,
                operationMessage: operationMessage,
                operationFailure: operationFailure,
                layoutRoot: layoutRoot,
                scrollHeight: scrollHeight,
                browseFolder: browseFolder,
                setAction: setAction,
                addRow: addRow,
                removeRow: removeRow,
                setSource: setSource,
                setName: setName,
                useFolderName: useFolderName,
                setAccess: setAccess
            };
        },

        render: function(createElement) {
            var owner = this;
            var h = createElement;
            var fieldStyle = { display: "block", width: "100%", boxSizing: "border-box", padding: "7px", marginTop: "4px" };

            function button(data, text) {
                data.style = Object.assign({
                    minHeight: "32px", padding: "6px 12px", border: "1px solid #cbd5e1",
                    borderRadius: "4px", background: "#f8fafc"
                }, data.style || {});
                return h("button", data, text);
            }

            function option(value, text) {
                return h("option", { domProps: { value: value } }, text);
            }

            function field(label, identifier, control) {
                return h("label", {
                    attrs: { for: identifier },
                    style: { display: "block", marginBottom: "12px" }
                }, [h("span", label), control]);
            }

            var content = [
                h("p", "Choose which NAS folders Dockpipe can access. Each appears under /host with its own name. Keep retains saved mappings; on a new installation it adds none."),
                field("Folder settings", "dockpipe-folder-action", h("select", {
                    attrs: { id: "dockpipe-folder-action", disabled: this.preparingAccess }, style: fieldStyle,
                    domProps: { value: this.action },
                    on: { change: function(event) { owner.setAction(event.target.value); } }
                }, [
                    option("keep", "Keep saved folder mappings"),
                    option("set", "Add or replace folder mappings"),
                    option("remove", "Remove all folder mappings from Dockpipe")
                ]))
            ];

            if (this.action === "set") {
                content.push(h("p", "This list replaces all saved mappings. Add every folder you want to keep; existing mappings are not prefilled."));
                this.rows.forEach(function(row, index) {
                    var prefix = "dockpipe-folder-" + row.id;
                    content.push(h("fieldset", {
                        key: row.id,
                        attrs: { disabled: owner.preparingAccess },
                        style: { border: "1px solid #cbd5e1", padding: "12px", marginBottom: "12px" }
                    }, [
                        h("legend", "Folder " + (index + 1)),
                        field("NAS folder path", prefix + "-source", h("input", {
                            attrs: { id: prefix + "-source", type: "text", placeholder: "/volume1/projects/My project", autocomplete: "off" },
                            style: fieldStyle, domProps: { value: row.source },
                            on: { input: function(event) { owner.setSource(row.id, event.target.value); } }
                        })),
                        button({
                            attrs: { type: "button", disabled: owner.preparingAccess || owner.pickingRow !== null },
                            style: { padding: "6px 16px", marginBottom: "12px" },
                            on: { click: function() { owner.browseFolder(row.id); } }
                        }, "Browse…"),
                        field("Name under /host", prefix + "-name", h("input", {
                            attrs: { id: prefix + "-name", type: "text", placeholder: "Folder name", autocomplete: "off" },
                            style: fieldStyle, domProps: { value: row.name },
                            on: { input: function(event) { owner.setName(row.id, event.target.value); } }
                        })),
                        h("p", "Available in Dockpipe at /host/" + (row.name || "<name>")),
                        button({
                            attrs: { type: "button" },
                            on: { click: function() { owner.useFolderName(row.id); } }
                        }, "Use folder name"),
                        field("Access", prefix + "-access", h("select", {
                            attrs: { id: prefix + "-access" }, style: fieldStyle,
                            domProps: { value: row.access },
                            on: { change: function(event) { owner.setAccess(row.id, event.target.value); } }
                        }, [option("ro", "Read only"), option("rw", "Read and write")])),
                        h("p", {
                            attrs: { role: "status", "data-access-state": row.permissionState },
                            style: { color: row.permissionState === "denied" || row.permissionState === "unavailable" ? "#b42318" : "#475467" }
                        }, row.permissionMessage),
                        button({
                            attrs: { type: "button" },
                            on: { click: function() { owner.removeRow(row.id); } }
                        }, "Remove folder from this list")
                    ]));
                });
                content.push(button({
                    attrs: { type: "button", disabled: this.preparingAccess || this.rows.length >= maximumFolders },
                    on: { click: function() { owner.addRow(); } }
                }, "Add another folder (" + this.rows.length + "/16)"));

            }
            if (this.operationMessage)
                content.push(h("p", { attrs: { role: "status", "aria-live": "polite" } }, this.operationMessage));
            if (this.operationFailure)
                content.push(h("p", { attrs: { role: "alert" }, style: { color: "#b42318" } }, this.operationFailure));
            if (this.pickerError)
                content.push(h("p", { attrs: { role: "alert" }, style: { color: "#b42318" } }, this.pickerError));
            if (this.validationError)
                content.push(h("p", { attrs: { role: "alert" }, style: { color: "#b42318" } }, this.validationError));
            content.push(h("p", "Dockpipe receives the access selected for each folder when you continue. Existing permissions and folder contents are preserved. Updating restarts the desktop; save your work first."));
            return h("pkg-center-step-content", [h("div", {
                ref: "layoutRoot",
                style: {
                    width: "100%", maxWidth: "680px", height: this.scrollHeight + "px",
                    minHeight: "0", overflowY: "auto", overflowX: "hidden",
                    boxSizing: "border-box", paddingRight: "8px", overflowWrap: "anywhere"
                }
            }, content)]);
        }
    });

    framework.JsonpLoader.load({ default: { name: "dockpipe_host_folders", render: component } });
})();
