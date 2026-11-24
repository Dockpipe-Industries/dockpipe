(function() {
    var scriptPath = "/webman/3rdparty/dockpipe-host-qualification/Dockpipe.HostQualification.js";
    var reportPath = "/webman/3rdparty/dockpipe-host-qualification/host-auth.cgi";
    var maximumResponseBytes = 16 * 1024;
    var requestTimeoutMilliseconds = 5000;

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
        if (document.currentScript)
            return tokenFromScript(document.currentScript);

        // DSM may evaluate an already downloaded script. Only its exact,
        // same-origin source is eligible; ambiguous copies fail closed.
        var token = null;
        var matches = 0;
        var scripts = document.scripts;
        for (var index = 0; index < scripts.length; index++) {
            var candidate = tokenFromScript(scripts[index]);
            if (candidate !== null) {
                token = candidate;
                matches++;
            }
        }
        return matches === 1 ? token : null;
    }

    // Keep DSM's session token private to this script. It never becomes an Ext
    // component property, report URL, saved setting or log message.
    var sessionToken = captureToken();

    async function readReport(response, request) {
        if (!response.body)
            throw new Error("missing response body");
        var reader = response.body.getReader();
        request.reader = reader;
        var data = new Uint8Array(maximumResponseBytes);
        var length = 0;
        try {
            while (true) {
                var chunk = await reader.read();
                if (chunk.done)
                    break;
                if (chunk.value.byteLength > maximumResponseBytes - length) {
                    request.controller.abort();
                    await reader.cancel();
                    throw new Error("response exceeds budget");
                }
                data.set(chunk.value, length);
                length += chunk.value.byteLength;
            }
        } finally {
            request.reader = null;
            reader.releaseLock();
        }
        var text = new TextDecoder("utf-8", { fatal: true }).decode(data.subarray(0, length));
        var report = JSON.parse(text);
        if (!report || report.schema !== "dockpipe.synology_host_qualification.v1")
            throw new Error("invalid report schema");
        return text;
    }

    function stopRequest(request) {
        request.controller.abort();
        if (request.reader)
            request.reader.cancel().catch(function() {});
    }

    Ext.ns("com.dockpipe");

    Ext.define("com.dockpipe.HostQualification", {
        extend: "SYNO.SDS.AppInstance",
        appWindowName: "com.dockpipe.HostQualificationWindow"
    });

    Ext.define("com.dockpipe.HostQualificationWindow", {
        extend: "SYNO.SDS.AppWindow",

        constructor: function(config) {
            this.appInstance = config.appInstance;
            config = Ext.apply({
                title: "Dockpipe host qualification — read only",
                width: 850,
                height: 550,
                resizable: true,
                maximizable: true,
                layout: "fit",
                items: [{
                    xtype: "box",
                    itemId: "qualification-report",
                    autoEl: {
                        tag: "pre",
                        style: "margin: 0; padding: 24px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere;"
                    }
                }]
            }, config);
            this.callParent([config]);
            this.on("afterrender", this.loadReport, this);
            this.on("beforedestroy", this.cancelReport, this);
        },

        loadReport: async function() {
            var component = this.getComponent("qualification-report");
            if (!component || !component.rendered)
                return;
            var output = component.getEl().dom;
            if (window.location.protocol !== "https:") {
                output.textContent = "Open DSM using HTTPS to run this check.";
                return;
            }
            if (sessionToken === null) {
                output.textContent = "DSM session token is unavailable. Reload DSM and reopen this check.";
                return;
            }
            if (this.reportRequest)
                return;

            output.textContent = "Checking native DSM identity…";
            var request = { controller: new AbortController(), reader: null, timedOut: false };
            this.reportRequest = request;
            var owner = this;
            request.timeout = setTimeout(function() {
                request.timedOut = true;
                if (owner.reportRequest === request)
                    output.textContent = "Native DSM identity check timed out.";
                stopRequest(request);
            }, requestTimeoutMilliseconds);
            try {
                var response = await window.fetch(reportPath, {
                    method: "GET",
                    credentials: "same-origin",
                    redirect: "error",
                    cache: "no-store",
                    referrerPolicy: "no-referrer",
                    headers: { "Accept": "application/json", "X-Syno-Token": sessionToken },
                    signal: request.controller.signal
                });
                if (owner.reportRequest !== request || request.timedOut) {
                    request.controller.abort();
                    return;
                }
                if (!response.ok) {
                    request.controller.abort();
                    if (owner.reportRequest === request) {
                        if (response.status === 503)
                            output.textContent = "Native DSM authentication is unavailable (503).";
                        else
                            output.textContent = "Native DSM identity check failed. Reload DSM and try again.";
                    }
                    return;
                }
                var text = await readReport(response, request);
                if (owner.reportRequest === request && !request.timedOut)
                    output.textContent = text;
            } catch (error) {
                if (owner.reportRequest === request) {
                    if (request.timedOut)
                        output.textContent = "Native DSM identity check timed out.";
                    else
                        output.textContent = "Native DSM identity report is unavailable.";
                }
            } finally {
                clearTimeout(request.timeout);
                if (owner.reportRequest === request)
                    owner.reportRequest = null;
            }
        },

        cancelReport: function() {
            var request = this.reportRequest;
            this.reportRequest = null;
            if (request) {
                clearTimeout(request.timeout);
                stopRequest(request);
            }
        }
    });
})();
