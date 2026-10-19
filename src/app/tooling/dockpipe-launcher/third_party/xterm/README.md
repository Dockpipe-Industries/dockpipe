# Bundled terminal renderer

Official MIT-licensed `@xterm/xterm` 6.0.0 and `@xterm/addon-fit` 0.11.0
release assets, downloaded from the npm registry. `provenance.json` records
immutable release URLs and SHA-512 package archive integrity values checked
before extraction. No install scripts or dependency builds run during vendoring.

Only the runtime JavaScript, CSS and license files are included. The renderer
loads from Qt resources without a CDN or network dependency. Both MIT notices
are also compiled into `terminal.qrc` so binary distributions retain them.

The upstream xterm 6 JavaScript uses class static initialization blocks that
Qt 6.2 WebEngine cannot parse. `xterm.js` is transformed to the Chrome 90 syntax
target using the pinned, architecture-independent `esbuild-wasm` 0.25.12 tool.
`compatibility.json` binds the upstream JavaScript SHA-256, tool archive integrity,
exact arguments, and transformed output SHA-256. This changes syntax without
adding runtime polyfills. The fit addon, CSS, and MIT notices remain upstream bytes.

Reproduce and verify all five runtime/license assets with Python 3 and Node.js:

```sh
python3 rebuild.py --cache /tmp/dockpipe-xterm-archives
```

The first run downloads the pinned archives and checks their SHA-512 integrity;
cached reruns work offline. No npm installation scripts run. For a reviewed
dependency update, change both provenance records and output hash, then use
`--write` to regenerate the assets. Normal launcher builds use the checked-in
assets and require neither Node.js nor a network connection.

Upstream: https://github.com/xtermjs/xterm.js
Documentation: https://xtermjs.org/docs/

Keep versions, provenance, runtime files, and license notices together when
upgrading. Validate terminal input/output, navigation isolation, queue bounds,
and dashboard retention after an upgrade.
