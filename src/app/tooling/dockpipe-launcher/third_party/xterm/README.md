# Bundled terminal renderer

Official MIT-licensed `@xterm/xterm` 6.0.0 and `@xterm/addon-fit` 0.11.0
release assets, downloaded from the npm registry. `provenance.json` records
immutable release URLs and SHA-512 package archive integrity values checked
before extraction. No install scripts or dependency builds run during vendoring.

Only the runtime JavaScript, CSS and license files are included. The renderer
loads from Qt resources without a CDN or network dependency. Both MIT notices
are also compiled into `terminal.qrc` so binary distributions retain them.

Upstream: https://github.com/xtermjs/xterm.js
Documentation: https://xtermjs.org/docs/

Keep versions, provenance, runtime files, and license notices together when
upgrading. Validate terminal input/output, navigation isolation, queue bounds,
and dashboard retention after an upgrade.
