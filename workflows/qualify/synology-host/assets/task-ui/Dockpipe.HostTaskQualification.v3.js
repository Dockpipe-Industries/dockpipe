(function() {
    var packagePath = "/webman/3rdparty/dockpipe-host-task-qualification/";
    var reportPath = packagePath + "host-auth.cgi";
    var scriptPath = packagePath + "Dockpipe.HostTaskQualification.v3.js";
    var schedulerPath = "/webapi/entry.cgi";
    var discoveryPath = "/webapi/query.cgi";
    var schema = "dockpipe.synology_host_task_qualification.v1";
    var schedulerAPI = "SYNO.Core.TaskScheduler";
    var maximumResponseBytes = 128 * 1024;
    var maximumTasks = 1024;
    var pageSize = 100;
    var requestTimeoutMilliseconds = 8000;
    var operationTimeoutMilliseconds = 60000;
    var backendErrors = [
        "not_found", "method_not_allowed", "invalid_authentication_metadata",
        "origin_required", "authentication_unavailable", "local_administrator_required",
        "json_required", "invalid_operation", "qualification_unavailable_or_owned",
        "response_exceeds_budget"
    ];

    function tokenFromScript(script) {
        if (!script || !script.src)
            return null;
        try {
            var source = new URL(script.src, window.location.href);
            if (source.origin !== window.location.origin || source.pathname !== scriptPath)
                return null;
            var tokens = source.searchParams.getAll("SynoToken");
            if (tokens.length !== 1 || !tokens[0] || tokens[0].length > 2048
                || /[\x00-\x1f\x7f]/.test(tokens[0]))
                return null;
            return tokens[0];
        } catch (error) {
            return null;
        }
    }

    function captureToken() {
        var currentToken = tokenFromScript(document.currentScript);
        if (currentToken !== null)
            return currentToken;
        var token = null;
        var matches = 0;
        for (var index = 0; index < document.scripts.length; index++) {
            var candidate = tokenFromScript(document.scripts[index]);
            if (candidate !== null) {
                token = candidate;
                matches++;
            }
        }
        return matches === 1 ? token : null;
    }

    // Keep the existing DSM session token private, with no URL or storage copy.
    var sessionToken = captureToken();

    function failure(message) {
        var error = new Error(message);
        error.safeTaskMessage = true;
        return error;
    }

    function canonicalScript(nonce) {
        return "/var/packages/dockpipe-host-task-qualification/target/bin/task-host probe"
            + " --socket /var/packages/dockpipe-host-task-qualification/var/run/task.sock --nonce " + nonce;
    }

    function canonicalTaskName(nonce) {
        return "Dockpipe host test " + nonce.slice(0, 16);
    }

    function checkedReport(report, owner) {
        var user = report && report.authenticated_user;
        if (!report || report.schema !== schema || typeof user !== "string" || !user
            || user.length > 256 || user === "root" || /[\s\x00-\x1f\x7f/:]/.test(user)
            || (owner && user !== owner))
            throw failure("DSM identity could not be verified. Reload DSM and reopen this test.");
        if (report.challenge !== null) {
            var challenge = report.challenge;
            if (!challenge || !/^[a-f0-9]{64}$/.test(challenge.nonce)
                || challenge.script !== canonicalScript(challenge.nonce)
                || challenge.task_name !== canonicalTaskName(challenge.nonce)
                || !Number.isFinite(Date.parse(challenge.expires_at)))
                throw failure("The test challenge is unavailable or invalid.");
        }
        if (report.result !== null && (!report.result || typeof report.result !== "object"))
            throw failure("The native test result is invalid.");
        return report;
    }

    function abortRequest(request) {
        request.controller.abort();
        if (request.reader)
            request.reader.cancel().catch(function() {});
    }

    function httpFailure(response, reason) {
        var message = "The NAS request failed";
        if (Number.isInteger(response.status) && response.status >= 100 && response.status <= 599)
            message += " (HTTP " + response.status + ")";
        if (typeof reason === "string" && backendErrors.includes(reason))
            message += ": " + reason;
        return failure(message + ".");
    }

    async function boundedJSON(response, request) {
        if (!response.body)
            throw httpFailure(response);
        var reader = response.body.getReader();
        request.reader = reader;
        var bytes = new Uint8Array(maximumResponseBytes);
        var length = 0;
        try {
            while (true) {
                var chunk = await reader.read();
                if (chunk.done)
                    break;
                if (chunk.value.byteLength > maximumResponseBytes - length)
                    throw failure("The NAS response exceeds the 128 KiB limit.");
                bytes.set(chunk.value, length);
                length += chunk.value.byteLength;
            }
            var result;
            try {
                var text = new TextDecoder("utf-8", { fatal: true }).decode(bytes.subarray(0, length));
                result = JSON.parse(text);
            } catch (error) {
                if (!response.ok)
                    throw httpFailure(response);
                throw failure("The NAS returned an invalid JSON response.");
            }
            if (!response.ok)
                throw httpFailure(response, result && result.error);
            return result;
        } finally {
            request.reader = null;
            reader.releaseLock();
        }
    }

    function TaskSetup(publish, busy) {
        this.publish = publish;
        this.busy = busy;
        this.operation = null;
        this.closed = false;
        this.seenTaskNonce = null;
        this.unusedChallengeNonce = null;
    }

    TaskSetup.prototype.request = async function(context, path, fields, jsonBody) {
        if (context.cancelled)
            throw failure("The test operation was cancelled.");
        var request = { controller: new AbortController(), reader: null };
        context.requests.add(request);
        var timeout = setTimeout(function() { abortRequest(request); }, requestTimeoutMilliseconds);
        var options = {
            method: "GET", credentials: "same-origin", redirect: "error", cache: "no-store",
            referrerPolicy: "no-referrer", signal: request.controller.signal,
            headers: { "Accept": "application/json", "X-Syno-Token": sessionToken }
        };
        if (fields) {
            options.method = "POST";
            if (jsonBody) {
                options.headers["Content-Type"] = "application/json";
                options.body = JSON.stringify(fields);
            } else {
                options.headers["Content-Type"] = "application/x-www-form-urlencoded;charset=UTF-8";
                options.body = new URLSearchParams(fields).toString();
            }
        }
        try {
            var response = await window.fetch(path, options);
            if (context.cancelled)
                throw failure("The test operation was cancelled.");
            return await boundedJSON(response, request);
        } finally {
            clearTimeout(timeout);
            abortRequest(request);
            context.requests.delete(request);
        }
    };

    TaskSetup.prototype.report = async function(context, action, owner, nonce) {
        var body = null;
        if (action) {
            body = { action: action };
            if (action === "clear")
                body.nonce = nonce;
        }
        return checkedReport(await this.request(context, reportPath, body, true), owner);
    };

    TaskSetup.prototype.scheduler = async function(context, method, version, fields) {
        var parameters = Object.assign({ api: schedulerAPI, method: method, version: version }, fields);
        var response = await this.request(context, schedulerPath, parameters, false);
        if (!response || response.success !== true) {
            var message = "DSM Task Scheduler rejected the request";
            var code = response && response.error && response.error.code;
            if (Number.isSafeInteger(code) && code >= 0 && code <= 2147483647)
                message += " (error " + code + ")";
            throw failure(message + ". Inspect the test before trying again.");
        }
        return response.data;
    };

    TaskSetup.prototype.discover = async function(context) {
        var response = await this.request(context, discoveryPath, {
            api: "SYNO.API.Info", version: 1, method: "query", query: schedulerAPI
        }, false);
        var api = response && response.data && response.data[schedulerAPI];
        if (!response || response.success !== true || !api || api.path !== "entry.cgi"
            || !Number.isInteger(api.minVersion) || !Number.isInteger(api.maxVersion)
            || api.minVersion > 2 || api.maxVersion < 4)
            throw failure("This DSM Task Scheduler API cannot safely run this test.");
    };

    TaskSetup.prototype.listTasks = async function(context) {
        var tasks = [];
        var identifiers = new Set();
        var expectedTotal = null;
        for (var offset = 0; offset < maximumTasks; offset += pageSize) {
            var page = await this.scheduler(context, "list", 3, {
                offset: offset, limit: pageSize, sort_by: "name", sort_direction: "ASC"
            });
            if (!page || !Array.isArray(page.tasks) || page.tasks.length > pageSize
                || !Number.isSafeInteger(page.total) || page.total < 0 || page.total > maximumTasks
                || (expectedTotal !== null && page.total !== expectedTotal))
                throw failure("The task list could not be verified completely. No task was changed.");
            expectedTotal = page.total;
            for (var row of page.tasks) {
                if (!row || !Number.isSafeInteger(row.id) || row.id <= 0 || identifiers.has(row.id))
                    throw failure("The task list changed or contains invalid task identities.");
                identifiers.add(row.id);
                tasks.push(row);
            }
            if (tasks.length === expectedTotal)
                return tasks;
            if (page.tasks.length !== pageSize || tasks.length > expectedTotal)
                break;
        }
        throw failure("The task list could not be verified completely. No task was changed.");
    };

    // DSM's real_owner routes scheduler operations. It may be root even when
    // the task's separate execution owner is the authenticated normal user.
    function supportedRealOwner(realOwner, owner) {
        return realOwner === owner || realOwner === "root";
    }

    function checkedTask(task, challenge, owner, identifier, realOwner) {
        if (!task || task.id !== identifier || task.name !== challenge.task_name
            || task.owner !== owner || !supportedRealOwner(realOwner, owner)
            || task.real_owner !== realOwner || task.enable !== false
            || task.type !== "script" || !task.extra || task.extra.script !== challenge.script
            || !task.schedule || task.schedule.date_type !== 1 || task.schedule.repeat_date !== 0
            || task.schedule.repeat_hour !== 0 || task.schedule.repeat_min !== 0)
            throw failure("The task does not match this disabled one-shot test. It was left unchanged.");
        return task;
    }

    TaskSetup.prototype.getTask = async function(context, identifier, challenge, owner, realOwner) {
        if (!supportedRealOwner(realOwner, owner))
            throw failure("The task has unsupported scheduler ownership metadata. It was left unchanged.");
        var task = await this.scheduler(context, "get", 4, { id: identifier, real_owner: realOwner });
        return checkedTask(task, challenge, owner, identifier, realOwner);
    };

    TaskSetup.prototype.reconcile = async function(context, report) {
        var challenge = report.challenge;
        var tasks = await this.listTasks(context);
        var candidates = tasks.filter(function(task) { return task.name === challenge.task_name; });
        if (candidates.length > 1)
            throw failure("More than one task matches this test name. No task was changed.");
        if (candidates.length === 0)
            return null;
        var candidate = candidates[0];
        if (candidate.owner !== report.authenticated_user || !supportedRealOwner(candidate.real_owner, report.authenticated_user)
            || candidate.enable !== false || candidate.type !== "script")
            throw failure("A task with this name has different ownership or settings. It was left unchanged.");
        var task = await this.getTask(context, candidate.id, challenge, report.authenticated_user, candidate.real_owner);
        this.seenTaskNonce = challenge.nonce;
        return task;
    };

    TaskSetup.prototype.pause = function(context, milliseconds) {
        if (context.cancelled)
            return Promise.reject(failure("The test operation was cancelled."));
        return new Promise(function(resolve, reject) {
            var wait = {
                reject: reject,
                timer: setTimeout(function() {
                    context.waits.delete(wait);
                    resolve();
                }, milliseconds)
            };
            context.waits.add(wait);
        });
    };

    TaskSetup.prototype.pollResult = async function(context, report) {
        for (var attempt = 0; attempt < 20; attempt++) {
            var current = await this.report(context, null, report.authenticated_user);
            if (!current.challenge || current.challenge.nonce !== report.challenge.nonce)
                throw failure("The test challenge expired or changed. No task will be run again.");
            if (current.result !== null)
                return current;
            await this.pause(context, 1000);
        }
        return report;
    };

    function oneShotSchedule() {
        var today = new Date();
        return {
            date_type: 1, date: today.getFullYear() + "/" + (today.getMonth() + 1) + "/" + today.getDate(),
            repeat_date: 0, monthly_week: "[]", hour: 0, minute: 0,
            repeat_hour: 0, repeat_min: 0, last_work_hour: 0
        };
    }

    function requireLiveChallenge(report) {
        if (!report.challenge || Date.parse(report.challenge.expires_at) <= Date.now())
            throw failure("The test challenge expired. Remove its disabled test task before starting again.");
    }

    TaskSetup.prototype.enable = async function(context) {
        var report = await this.report(context);
        await this.discover(context);
        if (report.challenge !== null) {
            await this.reconcile(context, report);
            this.publish(report, "An unfinished test already exists. Inspect results or remove its test task before starting another.");
            return;
        }
        report = await this.report(context, "prepare", report.authenticated_user);
        if (!report.challenge || report.created !== true) {
            this.publish(report, "Another setup attempt owns this challenge. Inspect results or remove its test task.");
            return;
        }
        requireLiveChallenge(report);
        var challenge = report.challenge;
        this.unusedChallengeNonce = challenge.nonce;
        if (await this.reconcile(context, report)) {
            this.publish(report, "A matching task already exists. Inspect results or remove it; it was not run again.");
            return;
        }
        this.unusedChallengeNonce = null;
        var created;
        try {
            created = await this.scheduler(context, "create", 4, {
                name: challenge.task_name, owner: report.authenticated_user, real_owner: report.authenticated_user,
                enable: false, type: "script", schedule: JSON.stringify(oneShotSchedule()),
                extra: JSON.stringify({ script: challenge.script, notify_enable: false, notify_mail: "", notify_if_error: false })
            });
        } catch (error) {
            this.publish(report, "Task creation could not be confirmed. Inspect results to reconcile it. Creation will not be retried.");
            return;
        }
        if (!created || !Number.isSafeInteger(created.id) || created.id <= 0)
            throw failure("Task creation returned no usable identity. Inspect results; creation will not be retried.");
        var task = await this.reconcile(context, report);
        if (!task || task.id !== created.id)
            throw failure("The created task could not be verified. Inspect results; no task was run.");
        var current = await this.report(context, null, report.authenticated_user);
        if (!current.challenge || current.challenge.nonce !== challenge.nonce)
            throw failure("The challenge changed. No task was run.");
        requireLiveChallenge(current);
        await this.getTask(context, task.id, challenge, report.authenticated_user, task.real_owner);
        var runConfirmed = true;
        try {
            await this.scheduler(context, "run", 2, {
                tasks: JSON.stringify([{ id: task.id, real_owner: task.real_owner }])
            });
        } catch (error) {
            runConfirmed = false;
        }
        var result = await this.pollResult(context, report);
        var message = "The task was run once. Its result is still pending; use Inspect results.";
        if (!runConfirmed)
            message = "The run could not be confirmed. Use Inspect results; the task will not be run again.";
        if (result.result !== null)
            message = "The NAS test result is ready. Remove the test task when finished.";
        this.publish(result, message);
    };

    TaskSetup.prototype.inspect = async function(context) {
        var report = await this.report(context);
        if (report.challenge !== null) {
            await this.discover(context);
            var task = await this.reconcile(context, report);
            if (!task) {
                this.publish(report, "No matching task is visible. An earlier create may still be unresolved; the challenge was retained.");
                return;
            }
            requireLiveChallenge(report);
            report = await this.pollResult(context, report);
        }
        var message = "No test is active. Select Enable NAS access to start.";
        if (report.challenge !== null)
            message = "The test task is verified. Its result is still pending; no task was run again.";
        if (report.result !== null)
            message = "The NAS test result is ready. Remove the test task when finished.";
        this.publish(report, message);
    };

    TaskSetup.prototype.verifyAbsent = async function(context, report) {
        for (var attempt = 0; attempt < 3; attempt++) {
            if (await this.reconcile(context, report))
                return false;
            if (attempt < 2)
                await this.pause(context, 1000);
        }
        return true;
    };

    TaskSetup.prototype.deleteTask = async function(context, task, challenge, owner) {
        await this.getTask(context, task.id, challenge, owner, task.real_owner);
        try {
            await this.scheduler(context, "delete", 2, {
                tasks: JSON.stringify([{ id: task.id, real_owner: task.real_owner }])
            });
        } catch (error) {
            // A lost response is resolved by listing, never by blind deletion.
        }
    };

    TaskSetup.prototype.removeOrphanedTasks = async function(context, report) {
        var tasks = await this.listTasks(context);
        var owner = report.authenticated_user;
        var candidates = tasks.filter(function(task) {
            return /^Dockpipe host test [a-f0-9]{16}$/.test(task.name)
                && task.owner === owner && supportedRealOwner(task.real_owner, owner)
                && task.enable === false && task.type === "script";
        });
        if (candidates.length > 16)
            throw failure("More than 16 leftover test tasks were found. No task was changed.");
        var verified = [];
        for (var candidate of candidates) {
            var task = await this.scheduler(context, "get", 4, { id: candidate.id, real_owner: candidate.real_owner });
            var script = task && task.extra && task.extra.script;
            var prefix = canonicalScript("");
            if (typeof script !== "string" || !script.startsWith(prefix))
                throw failure("A similarly named task has a different script. No leftover tasks were removed.");
            var nonce = script.slice(prefix.length);
            if (!/^[a-f0-9]{64}$/.test(nonce))
                throw failure("A similarly named task has a different script. No leftover tasks were removed.");
            var challenge = { nonce: nonce, script: canonicalScript(nonce), task_name: canonicalTaskName(nonce) };
            checkedTask(task, challenge, owner, candidate.id, candidate.real_owner);
            verified.push({ task: task, challenge: challenge });
        }
        for (var entry of verified) {
            await this.deleteTask(context, entry.task, entry.challenge, owner);
            var oldReport = { authenticated_user: owner, challenge: entry.challenge };
            if (!await this.verifyAbsent(context, oldReport))
                throw failure("A leftover test task is still present. Inspect before trying Remove again.");
        }
        this.publish(report, "Removed " + verified.length + " verified leftover test tasks. Other tasks were left unchanged.");
    };

    TaskSetup.prototype.remove = async function(context) {
        var report = await this.report(context);
        await this.discover(context);
        if (report.challenge === null) {
            await this.removeOrphanedTasks(context, report);
            return;
        }
        var challenge = report.challenge;
        var task = await this.reconcile(context, report);
        if (task)
            await this.deleteTask(context, task, challenge, report.authenticated_user);
        if (!await this.verifyAbsent(context, report)) {
            this.publish(report, "The test task is still present. Inspect it before trying Remove again.");
            return;
        }
        if (this.seenTaskNonce !== challenge.nonce && this.unusedChallengeNonce !== challenge.nonce) {
            this.publish(report, "No task is visible, but an earlier create remains uncertain. The challenge was retained; inspect again later.");
            return;
        }
        var current = await this.report(context, null, report.authenticated_user);
        if (!current.challenge || current.challenge.nonce !== challenge.nonce)
            throw failure("The challenge changed during removal. No challenge was cleared.");
        var cleared = await this.report(context, "clear", report.authenticated_user, challenge.nonce);
        if (cleared.challenge !== null || cleared.result !== null)
            throw failure("Task removal succeeded, but clearing the test could not be confirmed.");
        this.seenTaskNonce = null;
        this.unusedChallengeNonce = null;
        this.publish(cleared, "The test task and its result were removed.");
    };

    function cancelOperation(context) {
        context.cancelled = true;
        for (var request of context.requests)
            abortRequest(request);
        for (var wait of context.waits) {
            clearTimeout(wait.timer);
            wait.reject(failure("The test operation was cancelled or timed out."));
        }
        context.waits.clear();
    }

    TaskSetup.prototype.cancel = function() {
        this.closed = true;
        if (this.operation)
            cancelOperation(this.operation);
    };

    TaskSetup.prototype.perform = async function(action) {
        if (this.closed || this.operation)
            return;
        if (window.location.protocol !== "https:" || sessionToken === null) {
            this.publish(null, "Open DSM using HTTPS, reload DSM, and reopen this test to use its authenticated session.");
            return;
        }
        var context = { requests: new Set(), waits: new Set(), cancelled: false };
        this.operation = context;
        this.busy(true);
        var deadline = setTimeout(function() { cancelOperation(context); }, operationTimeoutMilliseconds);
        try {
            if (action === "load") {
                var report = await this.report(context);
                this.publish(report, "Select Enable NAS access to start, Inspect results to check an existing test, or Remove test task when finished.");
            } else {
                await this[action](context);
            }
        } catch (error) {
            if (!this.closed) {
                var message = "The NAS test request failed or timed out. Inspect results before trying setup again.";
                if (error && error.safeTaskMessage && !context.cancelled)
                    message = error.message;
                this.publish(null, message);
            }
        } finally {
            clearTimeout(deadline);
            this.operation = null;
            if (!this.closed)
                this.busy(false);
        }
    };

    Ext.ns("com.dockpipe");
    Ext.define("com.dockpipe.HostTaskQualification", {
        extend: "SYNO.SDS.AppInstance",
        appWindowName: "com.dockpipe.HostTaskQualificationWindow"
    });
    Ext.define("com.dockpipe.HostTaskQualificationWindow", {
        extend: "SYNO.SDS.AppWindow",

        constructor: function(config) {
            this.appInstance = config.appInstance;
            var owner = this;
            var buttons = [
                new SYNO.ux.Button({ text: "Enable NAS access", handler: function() { return owner.setup.perform("enable"); } }),
                new SYNO.ux.Button({ text: "Inspect results", handler: function() { return owner.setup.perform("inspect"); } }),
                new SYNO.ux.Button({ text: "Remove test task", handler: function() { return owner.setup.perform("remove"); } })
            ];
            config = Ext.apply({
                title: "Dockpipe NAS access test", width: 900, height: 620,
                resizable: true, maximizable: true, layout: "fit", buttons: buttons,
                items: [{
                    xtype: "box", itemId: "task-report",
                    autoEl: {
                        tag: "pre",
                        html: "Checking your DSM session…",
                        style: "box-sizing: border-box; width: 100%; height: 100%; margin: 0; padding: 24px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere;"
                    }
                }]
            }, config);
            this.callParent([config]);
            this.setup = new TaskSetup(function(report, message) {
                if (owner.setup.closed)
                    return;
                var component = owner.getComponent("task-report");
                if (!component || !component.rendered)
                    return;
                var text = "This creates a disabled one-shot test task and runs it once as your DSM account.\n\n" + message;
                if (report) {
                    text += "\n\nDSM account: " + report.authenticated_user;
                    if (report.challenge)
                        text += "\nChallenge expires: " + report.challenge.expires_at;
                    if (report.result !== null)
                        text += "\n\n" + JSON.stringify(report.result, null, 2);
                }
                component.getEl().dom.textContent = text;
            }, function(busy) {
                for (var button of buttons)
                    button.setDisabled(busy);
            });
            var initialLoadStarted = false;
            function loadOnce() {
                if (initialLoadStarted || owner.setup.closed)
                    return;
                initialLoadStarted = true;
                return owner.setup.perform("load");
            }
            this.on("afterrender", loadOnce, this, { single: true });
            this.on("beforedestroy", function() { owner.setup.cancel(); });
            if (this.rendered)
                loadOnce();
        }
    });
})();
