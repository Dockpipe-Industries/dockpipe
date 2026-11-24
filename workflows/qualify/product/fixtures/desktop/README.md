# Desktop fixture artifact

This manual workflow exports the existing Linux X11 desktop and accessibility
helpers for package-owned qualification. It starts no display, product GUI or guest and has
no CI registration. Invoke it with an absolute pinned Dockpipe executable:

```bash
"$DOCKPIPE_BIN" --workflow-file "$PWD/workflows/qualify/product/fixtures/desktop/config.yml" -- \
  --output /absolute/private/fresh-desktop-fixture
```

The deterministic archive contains exactly `manifest.json`, `desktop.py` and
`ui.py`. The manifest has schema `dockpipe.product-desktop-fixture/v1`, version
`0.8.0`, platform `linux-x11`, and the byte length and SHA-256 of each module.
The private build receipt binds the complete archive SHA-256. No source paths or
operator selections are embedded in the archive.

Consumers must pin the complete artifact, admit only these regular members,
check their exact lengths and hashes, and extract into a new private directory
before loading either module. They must select and verify their own tools, GUI
closure and private profile. `OwnedDesktop` requires an explicit callable
`prepare_environment` selection; native runtime, package and VM settings belong
to the caller's profile. The artifact supplies desktop lifecycle and
bounded owned-process AT-SPI helpers; it grants no authority to launch a product,
send input, contact a remote target or change an existing profile.

Packages own their product-specific actions, guest probes and receipts. Missing
accessibility or display prerequisites remain unpassed. Normal application exits
and exact process/socket cleanup require independent evidence; exporting or
importing this artifact does not qualify them.

Version 0.3.0 adds `ui.capture_owned_window`; existing 0.2.0
artifacts remain immutable. This API is additive: existing guided screen capture
and accessibility helpers retain their behavior.

The caller supplies GUI and private X-server identities, each with exact host
`pid`, kernel `starttime` and `exe_sha256`; observed window `{id, x, y, width,
height}`; exactly `DISPLAY` and `XAUTHORITY`; and absolute, SHA-pinned `xwd`,
`xprop` and `xwininfo` tools. The local server command must bind that display and
private authority file. Its directory and a fresh PNG output directory must be
operator-private. No ambient environment, shell, root-screen capture, input or
window discovery is used. Xauthority contents are never read or recorded.

Before and after capture the helper verifies current process identities,
`_NET_WM_PID`, viewable window geometry and private server binding. The accepted
X11 PID is derived from the exact host process's bounded `/proc/PID/status`
`NSpid` mapping, including a Flatpak inner PID; callers cannot supply a substitute
PID. These X11 properties are cooperative fixture checks, not a security boundary
against a malicious same-user X client. Inability to observe the host mapping
fails closed.

XWD v7 TrueColor ZPixmap at 24/32 bits per pixel, the admitted dimensions,
standard RGB masks and a checked padded stride are decoded. Version 0.8 also
admits the exact identity DirectColor layout described below. A 4096-by-4096
ceiling bounds raw image retention to 64 MiB plus header/color records. Tool
output, executable hashing, archive-independent decoding and PNG conversion
share a caller deadline of at most 30 seconds. Cancellation stops the exact
invocation's dedicated tool process group; each cleanup has up to one extra
second. Expected tool failures, malformed/truncated images, byte exhaustion,
identity drift and cancellation before the publication commit publish no image. Complete PNG bytes are written to a new private temporary file and atomically
published through a no-replace hard link; raced output preimages are preserved.
Successful PNGs are new private files with a receipt binding the owner, display, window, tools and PNG digest.
The capture proves only the selected drawable's pixels; it does not qualify an
input action, product case or guest lifecycle.

## Optional owned screen geometry

Version 0.4.0 adds `selections["screen_geometry"] = {"width": 1600,
"height": 1200}` to `OwnedDesktop`. Missing geometry retains the 1600-by-1000
default exactly. Depth remains 24; there is no raw Xvfb-argument escape. The object
requires exactly integer width/height (booleans rejected), each 1 through 4096,
and at most 8,388,608 pixels. This supports UHD 3840-by-2160 while bounding a
padded 32-bit framebuffer to 32 MiB. Invalid input fails before output creation,
application preparation or subprocesses. No existing 0.3 artifact is overwritten.

Choose a screen large enough for the admitted full window: `xwd -id` clips pixels
outside the screen even when the native window geometry is larger. Such clipped
data is not a full-window capture and remains rejected by `capture_owned_window`.
Geometry selection does not change that capture contract or grant input authority.

Version 0.5.0 removes recursive runtime-directory cleanup. Only the two generated bus sockets may be removed; unknown entries, nested directories, symlinks and mount directories are retained with a cleanup error. Callers must report retained paths rather than claiming path absence. Existing 0.4 artifacts remain immutable.

Version 0.6.0 tolerates a known bus socket disappearing between directory listing
and its first metadata check, or after the identity checks before unlink, as a
stopped bus may unlink its own socket.
Unknown names remain refused even if they have disappeared; present entries must
still be sockets, and replacement identity checks remain unchanged. Cleanup does
not traverse runtime trees or remove unknown directories. Existing 0.5 artifacts
remain immutable; callers must explicitly adopt and pin the new artifact.

Version 0.7.0 adds `ui.capture_failure_diagnostic(error)` for a rejected XWD
layout. `XwdLayoutError` remains a `ValueError` and preserves the previous constant
message. The decoder's acceptance/rejection predicates and all capture ownership,
geometry, byte, cancellation and publication guards remain unchanged.

The accessor returns only a fixed schema, finite reason/check names, requested
dimensions, signed-64-bit bounded raw length, 25 named unsigned-32-bit header
values, derived size values bounded to 2^45, and the name-terminator boolean.
Its fixed document shape is below 8 KiB. For an incomplete header it retains only
requested dimensions, raw length and the required header length; partial bytes
are not interpreted or retained. It never reads exception text or arbitrary
attributes, and never retains the window name, color records or pixel bytes.
Unrelated or malformed exception metadata returns `None`. Captures still publish
no image on failure. No header from a historical failure is reconstructed or
inferred. Consumers must explicitly adopt version 0.7 and use the new artifact's
accessor when recording sanitized failure context; old artifacts remain immutable.

Version 0.8.0 adds a narrowly admitted synthetic identity DirectColor layout
that `xwd` can produce when a 32-bit visual overlaps a 24-bit drawable. This is
not a general DirectColor or colormap decoder. The additional format requires
v7 ZPixmap, depth 24, 24 bits per pixel, bitmap unit 32, bitmap pad 8, standard
RGB masks, 8 bits per RGB, 256 colormap entries and exactly 256 color records.
The declared row stride must be between width times three and width times four;
the decoder uses that declared stride and requires the exact payload length.

Before reading pixels, it verifies a complete unique diagonal identity table:
each index from 0 through 255 occurs once as pixel `index * 0x010101`, all three
16-bit channels equal `index * 257`, and every color flag is zero. The unused
structural padding byte is ignored. Missing, duplicate, nonidentity, nondiagonal
or differently flagged records remain rejected. Cancellation is checked during
table validation and on each decoded row. Existing TrueColor acceptance and
PNG encoding remain unchanged, as do ownership, geometry, byte, deadline and
publication bounds.

A rejected identity table reports the finite diagnostic reason
`xwd-identity-palette-rejected` and check `identity-directcolor-palette`. Only
this reason uses integer `encoded_row_minimum` and `maximum_stride` instead of
`expected_stride`; all other safe header fields and redaction rules remain the
same. Color records and raw pixels are never included in diagnostics. Consumers
must explicitly admit and pin version 0.8 and its diagnostic shape. The new
format was verified against generated mixed-visual X11 colors and independent
PNG checks; it does not establish the cause of any historical capture failure.
Existing 0.7 and earlier artifacts remain immutable.
