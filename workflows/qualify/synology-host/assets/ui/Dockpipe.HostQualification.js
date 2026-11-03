Ext.ns("com.dockpipe");

Ext.define("com.dockpipe.HostQualification", {
    extend: "SYNO.SDS.AppInstance",
    appWindowName: "com.dockpipe.HostQualification.Window"
});

Ext.define("com.dockpipe.HostQualification.Window", {
    extend: "SYNO.SDS.AppWindow",

    constructor: function(config) {
        var content;
        if (window.location.protocol === "https:") {
            content = {
                xtype: "box",
                autoEl: {
                    tag: "iframe",
                    src: "/webman/3rdparty/dockpipe-host-qualification/host-auth.cgi",
                    title: "Native DSM identity check",
                    width: "100%",
                    height: "100%",
                    frameborder: "0"
                }
            };
        } else {
            content = {
                xtype: "box",
                autoEl: {
                    tag: "div",
                    html: "Open DSM using HTTPS to run this check.",
                    style: "padding: 24px;"
                }
            };
        }
        config = Ext.apply({
            title: "Dockpipe host qualification — read only",
            width: 850,
            height: 550,
            resizable: true,
            maximizable: true,
            layout: "fit",
            items: [content]
        }, config);
        this.callParent([config]);
    }
});
