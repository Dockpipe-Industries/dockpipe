use tauri::ipc::CapabilityBuilder;
use url::{Host, Url};

/// Grant the native clipboard bridge only to the configured loopback editor.
/// Navigating to another server or another local port does not carry this grant.
pub fn clipboard_capability(editor_url: &Url) -> Option<CapabilityBuilder> {
    if !matches!(editor_url.scheme(), "http" | "https") {
        return None;
    }

    let loopback = match editor_url.host()? {
        Host::Domain(host) => host == "localhost",
        Host::Ipv4(address) => address.is_loopback(),
        Host::Ipv6(address) => address.is_loopback(),
    };
    if !loopback {
        return None;
    }

    // URLPattern treats colons inside an IPv6 host as pattern syntax unless escaped.
    let host = editor_url.host()?.to_string().replace(':', "\\:");
    let port = editor_url
        .port()
        .map(|port| format!(":{port}"))
        .unwrap_or_default();
    let origin = format!("{}://{host}{port}", editor_url.scheme());
    Some(
        CapabilityBuilder::new("pipeon-editor-clipboard")
            .local(false)
            .webview("main")
            .remote(format!("{origin}/*"))
            .permission("clipboard-bridge"),
    )
}
