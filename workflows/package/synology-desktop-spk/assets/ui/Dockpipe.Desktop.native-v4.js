/* The producer prepends workflows/shared/synology/assets/dsm-native-access.js. */
(function() {
    var desktopPath = "/webman/3rdparty/dockpipe-desktop/desktop/";
    var nativeAccess = window.DockpipeNativeAccess.create({
        scriptPath: "/webman/3rdparty/dockpipe-desktop/Dockpipe.Desktop.native-v4.js",
        reportPath: "/webman/3rdparty/dockpipe-desktop/native-host.cgi",
        schema: "dockpipe.synology_native_host.v1",
        scriptPrefix: "/var/packages/dockpipe-desktop/target/bin/dockpipe host worker --detach"
            + " --socket /var/packages/dockpipe-desktop/etc/native-host/native-worker.sock --nonce ",
        taskNamePrefix: "Dockpipe host access ",
        persistent: true,
        finishAction: "finish"
    });

    function accessFailure(message) {
        var error = new Error(message);
        error.safeTaskMessage = true;
        return error;
    }

    Ext.ns("com.dockpipe");
    Ext.define("com.dockpipe.Desktop", {
        extend: "SYNO.SDS.AppInstance",
        appWindowName: "com.dockpipe.DesktopWindow"
    });
    Ext.define("com.dockpipe.DesktopWindow", {
        extend: "SYNO.SDS.AppWindow",

        constructor: function(config) {
            this.appInstance = config.appInstance;
            this.nativeState = {
                closing: false, disabling: false, generation: 0, operator: null, expectedUID: null,
                heartbeatTimer: null, retryTimer: null, expiryTimer: null, verifiedDeadline: null,
                heartbeatPromise: null, enablePromise: null, loadPromise: null,
                wantLease: false, allowFrame: false, frameOpen: false, busy: false,
                failureMessage: null, action: "checking"
            };
            var owner = this;
            this.enableButton = new SYNO.ux.Button({
                text: "Enable NAS access",
                btnStyle: "blue", scale: "large", width: 260, height: 52,
                handler: function() { return owner.enableNASAccess(); }
            });
            this.disableButton = new SYNO.ux.Button({
                text: "Disable NAS access",
                handler: function() { return owner.disableNASAccess(); }
            });
            config = Ext.apply({
                title: "Dockpipe", width: 1100, height: 760, minWidth: 640, minHeight: 480,
                resizable: true, maximizable: true, minimizable: true,
                layout: "card", activeItem: 0, buttons: [this.disableButton],
                items: [{
                    xtype: "container", itemId: "native-setup", layout: "hbox",
                    layoutConfig: { pack: "center", align: "middle" },
                    style: "background: #f5f7fa;",
                    items: [{
                        xtype: "container", itemId: "native-content", width: 560, height: 360,
                        layout: "vbox", layoutConfig: { pack: "center", align: "center" },
                        items: [{
                            xtype: "box", itemId: "native-headline", width: 560, height: 54,
                            autoEl: {
                                tag: "h1", html: "Connect Dockpipe to your NAS",
                                style: "margin: 0; font-size: 28px; line-height: 36px; font-weight: 600; text-align: center; color: #182230;"
                            }
                        }, {
                            xtype: "box", width: 560, height: 64,
                            autoEl: {
                                tag: "p", html: "Use your current DSM account to enable NAS access.<br>The desktop opens after the connection is verified.",
                                style: "margin: 0; font-size: 16px; line-height: 24px; text-align: center; color: #475467;"
                            }
                        }, {
                            xtype: "box", itemId: "native-status", width: 560, height: 96,
                            autoEl: {
                                tag: "div", html: "Checking existing NAS access…",
                                style: "box-sizing: border-box; padding: 12px 8px; font-size: 15px; line-height: 22px; text-align: center; white-space: pre-wrap; color: #344054;"
                            }
                        }, {
                            xtype: "box", itemId: "native-stage", width: 560, height: 40,
                            autoEl: {
                                tag: "div", html: "Checking your DSM session",
                                style: "font-size: 13px; line-height: 20px; text-align: center; color: #667085;"
                            }
                        }, this.enableButton]
                    }]
                }, {
                    xtype: "box", itemId: "desktop-frame",
                    autoEl: {
                        tag: "iframe", src: "about:blank", title: "Dockpipe desktop",
                        width: "100%", height: "100%", frameborder: "0",
                        style: "display: block; border: 0;",
                        allow: "clipboard-read; clipboard-write; fullscreen"
                    }
                }]
            }, config);
            this.callParent([config]);
            this.accessController = nativeAccess.createController(function(report, message) {
                if (owner.nativeState.closing)
                    return;
                if (report) {
                    owner.nativeState.operator = report.authenticated_user;
                    owner.nativeState.expectedUID = report.expected_uid;
                }
                if (owner.nativeState.disabling)
                    return;
                if (!owner.nativeState.frameOpen) {
                    if (owner.nativeState.failureMessage)
                        owner.setNativeStatus(owner.nativeState.failureMessage, "Connection needs attention");
                    else if (report && nativeAccess.isReady(report))
                        owner.setNativeStatus("NAS access is ready. Connecting…", "Verifying access before opening the desktop");
                    else if (report && owner.nativeState.action === "enabling")
                        owner.setNativeStatus("NAS access setup is unfinished. Select Enable NAS access to check progress, or Disable NAS access to stop.", "Waiting for the NAS connection");
                    else if (report)
                        owner.setNativeStatus("NAS access is not enabled. Select Enable NAS access to continue.", "Ready to connect");
                    else
                        owner.setNativeStatus(message.replace(/\btest\b/g, "setup"), "Connection needs attention");
                }
            }, function(busy) {
                owner.nativeState.busy = busy;
                owner.updateNativeButtons();
            }, async function(report) {
                owner.setNativeStatus("Your DSM account is verified. Starting the NAS connection…", "Renewing the browser session");
                owner.startLease(report, false);
                if (!await owner.renewLease())
                    throw accessFailure(owner.nativeState.failureMessage || "The NAS session could not be verified. Try Enable NAS access again.");
                owner.setNativeStatus("Starting the NAS connection. Keep this window open…", "Starting native NAS access");
            });
            var loaded = false;
            function loadOnce() {
                if (loaded || owner.nativeState.closing)
                    return;
                loaded = true;
                owner.nativeState.loadPromise = owner.loadNativeAccess();
                return owner.nativeState.loadPromise;
            }
            this.on("afterrender", loadOnce, this, { single: true });
            this.on("activate", this.focusDesktop, this);
            this.on("show", this.focusDesktop, this);
            this.on("beforedestroy", this.releaseDesktop, this);
            if (this.rendered)
                loadOnce();
        },

        updateNativeButtons: function() {
            var state = this.nativeState;
            this.enableButton.setDisabled(state.busy || state.disabling);
            // Disable remains reachable while an Enable request is in progress.
            this.disableButton.setDisabled(state.disabling);
        },

        setupComponent: function(identifier) {
            var setup = this.getComponent("native-setup");
            var content = setup && setup.getComponent("native-content");
            return content && content.getComponent(identifier);
        },

        setNativeStatus: function(message, stage) {
            var component = this.setupComponent("native-status");
            if (component && component.rendered)
                component.getEl().dom.textContent = message;
            var stageComponent = this.setupComponent("native-stage");
            if (stage && stageComponent && stageComponent.rendered)
                stageComponent.getEl().dom.textContent = stage;
        },

        desktopFrame: function() {
            var component = this.getComponent("desktop-frame");
            if (!component || !component.rendered)
                return null;
            return component.getEl().dom;
        },

        closeFrame: function() {
            var frame = this.desktopFrame();
            if (frame)
                frame.src = "about:blank";
            this.nativeState.frameOpen = false;
            if (!this.nativeState.closing) {
                var layout = this.getLayout();
                if (layout)
                    layout.setActiveItem(0);
            }
        },

        showFrame: function(report) {
            var state = this.nativeState;
            if (!state.allowFrame || state.closing || !nativeAccess.isReady(report)
                || report.authenticated_user !== state.operator || report.expected_uid !== state.expectedUID)
                return;
            var frame = this.desktopFrame();
            if (!frame)
                return;
            var opening = !state.frameOpen;
            if (frame.getAttribute("src") !== desktopPath) {
                frame.onload = function() {
                    // Chromium can report outer-page coordinates for raw pointer
                    // events inside an iframe. Normal pointer events use the
                    // correct frame coordinates; retain those and touch input.
                    frame.contentWindow.addEventListener("pointerrawupdate", function(event) {
                        if (event.pointerType === "mouse" || event.pointerType === "pen")
                            event.stopImmediatePropagation();
                    }, true);
                };
                frame.src = desktopPath;
            }
            state.frameOpen = true;
            this.getLayout().setActiveItem(1);
            if (opening)
                this.focusDesktop();
        },

        stopLease: function() {
            var state = this.nativeState;
            state.generation++;
            state.wantLease = false;
            state.allowFrame = false;
            if (state.heartbeatTimer !== null)
                clearInterval(state.heartbeatTimer);
            if (state.expiryTimer !== null)
                clearTimeout(state.expiryTimer);
            if (state.retryTimer !== null)
                clearTimeout(state.retryTimer);
            state.heartbeatTimer = null;
            state.expiryTimer = null;
            state.retryTimer = null;
            state.verifiedDeadline = null;
            if (this.accessController)
                this.accessController.cancelHeartbeat();
            this.closeFrame();
        },

        failNativeAccess: function(message) {
            this.nativeState.failureMessage = message || "NAS access is unavailable. Select Enable NAS access to reconnect.";
            this.stopLease();
            this.accessController.abort();
            this.setNativeStatus(this.nativeState.failureMessage, "Connection needs attention");
        },

        startLease: function(report, allowFrame) {
            var state = this.nativeState;
            state.operator = report.authenticated_user;
            state.expectedUID = report.expected_uid;
            state.wantLease = true;
            state.allowFrame = allowFrame;
            if (state.heartbeatTimer === null) {
                var owner = this;
                state.heartbeatTimer = setInterval(function() { owner.renewLease(); }, 10000);
            }
        },

        renewLease: function() {
            var state = this.nativeState;
            if (state.closing || !state.wantLease)
                return Promise.resolve(null);
            if (state.heartbeatPromise)
                return state.heartbeatPromise;
            if (state.verifiedDeadline !== null && state.verifiedDeadline <= Date.now()) {
                this.failNativeAccess("Your NAS access session expired. Select Enable NAS access to reconnect.");
                return Promise.resolve(null);
            }
            if (state.retryTimer !== null) {
                clearTimeout(state.retryTimer);
                state.retryTimer = null;
            }
            var owner = this;
            var generation = state.generation;
            state.heartbeatPromise = (async function() {
                try {
                    var report = await owner.accessController.heartbeat(state.operator);
                    if (state.closing || generation !== state.generation || !state.wantLease)
                        return null;
                    if (state.verifiedDeadline !== null && state.verifiedDeadline <= Date.now())
                        throw accessFailure("Your NAS access session expired. Select Enable NAS access to reconnect.");
                    if (report.expected_uid !== state.expectedUID)
                        throw accessFailure("The NAS account did not match your DSM account. Reopen Dockpipe and try again.");
                    var expires = Date.parse(report.lease_expires_at);
                    var verifiedAt = Date.now();
                    var remaining = Math.min(30000, expires - verifiedAt);
                    if (!Number.isFinite(remaining))
                        throw accessFailure("NAS access could not be verified: the server did not provide a valid session expiry. Try Enable NAS access again.");
                    if (remaining <= 0)
                        throw accessFailure("Your NAS access session expired. Select Enable NAS access to reconnect.");
                    // Only a successful authenticated response can advance this deadline.
                    state.verifiedDeadline = verifiedAt + remaining;
                    if (state.expiryTimer !== null)
                        clearTimeout(state.expiryTimer);
                    state.expiryTimer = setTimeout(function() {
                        if (generation === state.generation && state.wantLease)
                            owner.failNativeAccess("Your NAS access session expired. Select Enable NAS access to reconnect.");
                    }, remaining);
                    if (state.allowFrame) {
                        if (!nativeAccess.isReady(report))
                            throw accessFailure("The NAS host connection is unavailable. Select Enable NAS access to reconnect.");
                        owner.showFrame(report);
                    }
                    return report;
                } catch (error) {
                    if (!state.closing && generation === state.generation) {
                        var remaining = state.verifiedDeadline - Date.now();
                        if (error && error.retryableNASRequest === true && state.wantLease
                            && state.verifiedDeadline !== null && remaining > 0) {
                            state.retryTimer = setTimeout(function() {
                                state.retryTimer = null;
                                owner.renewLease();
                            }, Math.min(2000, remaining));
                            owner.setNativeStatus("The NAS is taking longer to respond. Retrying while your verified session remains valid…", "Checking the NAS connection");
                            return null;
                        }
                        var message = "The NAS session check failed. Check your DSM connection and try again.";
                        if (error && error.safeTaskMessage)
                            message = error.message;
                        owner.failNativeAccess(message);
                    }
                    return null;
                } finally {
                    state.heartbeatPromise = null;
                }
            })();
            return state.heartbeatPromise;
        },

        loadNativeAccess: async function() {
            var state = this.nativeState;
            var generation = state.generation;
            var report = await this.accessController.perform("load");
            if (state.closing || generation !== state.generation || !report)
                return;
            if (nativeAccess.isReady(report)) {
                this.startLease(report, true);
                await this.renewLease();
            }
        },

        enableNASAccess: function() {
            var state = this.nativeState;
            if (state.closing || state.enablePromise || state.busy || state.disabling)
                return Promise.resolve(null);
            state.failureMessage = null;
            state.action = "enabling";
            this.stopLease();
            this.setNativeStatus("Connecting to your NAS using your DSM account…", "Checking permissions and preparing access");
            var generation = state.generation;
            var owner = this;
            state.enablePromise = (async function() {
                try {
                    var report = await owner.accessController.perform("enable");
                    if (state.closing || generation !== state.generation)
                        return null;
                    if (!report) {
                        owner.stopLease();
                        return null;
                    }
                    if (nativeAccess.isReady(report)) {
                        owner.startLease(report, true);
                        await owner.renewLease();
                    } else {
                        owner.setNativeStatus("NAS access setup is incomplete. Select Enable NAS access to check it again, or Disable NAS access to stop.", "Waiting for the NAS connection");
                    }
                    return report;
                } finally {
                    state.enablePromise = null;
                }
            })();
            return state.enablePromise;
        },

        disableNASAccess: async function() {
            var state = this.nativeState;
            if (state.closing || state.disabling)
                return;
            state.disabling = true;
            state.failureMessage = null;
            this.stopLease();
            this.setNativeStatus("Closing NAS access…", "Removing setup state and disconnecting");
            this.accessController.abort();
            this.updateNativeButtons();
            try {
                if (state.enablePromise)
                    await state.enablePromise;
                if (state.loadPromise)
                    await state.loadPromise;
                if (state.heartbeatPromise)
                    await state.heartbeatPromise;
                var cleanup = await this.accessController.perform("remove");
                await this.accessController.disconnect(state.operator);
                var message = "NAS access is disabled.";
                if (!cleanup || cleanup.challenge)
                    message += " Setup cleanup could not be confirmed. Reopen Dockpipe to check it again.";
                this.setNativeStatus(message, "NAS access is disabled");
            } catch (error) {
                this.setNativeStatus("The desktop is closed. NAS access could not be confirmed disabled; its current session will expire shortly.", "Disconnect could not be confirmed");
            } finally {
                state.disabling = false;
                if (!state.closing)
                    this.updateNativeButtons();
            }
        },

        focusDesktop: function() {
            var frame = this.desktopFrame();
            if (this.nativeState.frameOpen && frame && frame.contentWindow)
                frame.contentWindow.focus();
        },

        releaseDesktop: function() {
            this.nativeState.closing = true;
            this.stopLease();
            this.accessController.cancel();
        }
    });
})();
