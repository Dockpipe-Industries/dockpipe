#[path = "../src/clipboard_access.rs"]
mod clipboard_access;

use tauri::ipc::{CallbackFn, InvokeBody};
use tauri::test::{get_ipc_response, mock_builder, MockRuntime, INVOKE_KEY};
use tauri::webview::InvokeRequest;
use tauri::{Manager, WebviewWindow, WebviewWindowBuilder};

// These handlers deliberately never touch the host clipboard. The test exercises
// Tauri's real IPC authorization before a clipboard handler can be invoked.
#[tauri::command]
fn read_clipboard_text() -> &'static str {
    "allowed"
}

#[tauri::command]
fn write_clipboard_text() -> &'static str {
    "allowed"
}

fn invoke(window: &WebviewWindow<MockRuntime>, origin: &str, command: &str) -> bool {
    get_ipc_response(
        window,
        InvokeRequest {
            cmd: command.into(),
            callback: CallbackFn(0),
            error: CallbackFn(1),
            url: origin.parse().unwrap(),
            body: InvokeBody::default(),
            headers: Default::default(),
            invoke_key: INVOKE_KEY.to_string(),
        },
    )
    .is_ok()
}

#[test]
fn clipboard_grant_is_limited_to_the_configured_editor_origin_and_webview() {
    let app = mock_builder()
        .invoke_handler(tauri::generate_handler![
            read_clipboard_text,
            write_clipboard_text
        ])
        .build(tauri::generate_context!())
        .unwrap();
    let editor_url = "http://127.0.0.1:38421/editor?folder=project".parse().unwrap();
    let main = WebviewWindowBuilder::new(&app, "main", Default::default())
        .build()
        .unwrap();
    let other = WebviewWindowBuilder::new(&app, "other", Default::default())
        .build()
        .unwrap();

    // Reproduces the regression before an explicit remote grant is installed.
    assert!(!invoke(&main, "http://127.0.0.1:38421/", "read_clipboard_text"));
    app.add_capability(clipboard_access::clipboard_capability(&editor_url).unwrap())
        .unwrap();

    for command in ["read_clipboard_text", "write_clipboard_text"] {
        assert!(invoke(&main, "http://127.0.0.1:38421/another/path", command));
        for origin in [
            "http://127.0.0.1:38422/",
            "https://127.0.0.1:38421/",
            "http://localhost:38421/",
            "https://example.com/",
            "http://127.0.0.1.example.com:38421/",
        ] {
            assert!(!invoke(&main, origin, command), "unexpected grant for {origin}");
        }
        assert!(!invoke(&other, "http://127.0.0.1:38421/", command));
    }
    assert!(!invoke(&main, "http://127.0.0.1:38421/", "plugin:window|close"));
}

#[test]
fn remote_and_non_http_editor_urls_get_no_native_clipboard_grant() {
    for url in [
        "https://example.com/",
        "http://192.0.2.1:38421/",
        "file:///tmp/editor.html",
        "http://localhost.example.com/",
    ] {
        assert!(clipboard_access::clipboard_capability(&url.parse().unwrap()).is_none());
    }
}

#[test]
fn configured_localhost_and_ipv6_origins_keep_their_explicit_ports() {
    for origin in ["http://localhost:49152/", "http://[::1]:49152/"] {
        let app = mock_builder()
            .invoke_handler(tauri::generate_handler![read_clipboard_text])
            .build(tauri::generate_context!())
            .unwrap();
        app.add_capability(clipboard_access::clipboard_capability(&origin.parse().unwrap()).unwrap())
            .unwrap();
        let main = WebviewWindowBuilder::new(&app, "main", Default::default())
            .build()
            .unwrap();
        assert!(invoke(&main, origin, "read_clipboard_text"));
        let different_port = origin.replace("49152", "49153");
        assert!(!invoke(&main, &different_port, "read_clipboard_text"));
    }
}
