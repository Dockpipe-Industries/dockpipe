/* Dockpipe's DSM shell hosts the existing streamed Qt desktop. */
Ext.ns("com.dockpipe");

Ext.define("com.dockpipe.Desktop", {
    extend: "SYNO.SDS.AppInstance",
    appWindowName: "com.dockpipe.Desktop.Window"
});

Ext.define("com.dockpipe.Desktop.Window", {
    extend: "SYNO.SDS.AppWindow",

    constructor: function(config) {
        this.appInstance = config.appInstance;

        var desktopContent;
        if (window.location.protocol === "https:") {
            desktopContent = {
                xtype: "box",
                itemId: "desktop-frame",
                autoEl: {
                    tag: "iframe",
                    src: "/webman/3rdparty/dockpipe-desktop/desktop/",
                    title: "Dockpipe desktop",
                    width: "100%",
                    height: "100%",
                    frameborder: "0",
                    style: "display: block; border: 0;",
                    allow: "clipboard-read; clipboard-write; fullscreen"
                }
            };
        } else {
            desktopContent = {
                xtype: "box",
                autoEl: {
                    tag: "div",
                    style: "padding: 24px;",
                    html: "Open DSM using HTTPS to use the Dockpipe desktop."
                }
            };
        }

        config = Ext.apply({
            title: "Dockpipe",
            resizable: true,
            maximizable: true,
            minimizable: true,
            width: 1100,
            height: 760,
            minWidth: 640,
            minHeight: 480,
            layout: "fit",
            items: [desktopContent]
        }, config);

        this.callParent([config]);
        this.on("activate", this.focusDesktop, this);
        this.on("show", this.focusDesktop, this);
        this.on("beforedestroy", this.releaseDesktop, this);
    },

    desktopFrame: function() {
        var component = this.getComponent("desktop-frame");
        if (!component || !component.rendered)
            return null;

        return component.getEl().dom;
    },

    focusDesktop: function() {
        var frame = this.desktopFrame();
        if (frame && frame.contentWindow)
            frame.contentWindow.focus();
    },

    releaseDesktop: function() {
        var frame = this.desktopFrame();
        if (frame)
            frame.src = "about:blank";
    }
});
