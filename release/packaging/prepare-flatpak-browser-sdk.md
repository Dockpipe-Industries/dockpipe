# Exact Flatpak Browser SDK consumer

Flatpak uses the separately published `SpatialWebFlatpakBrowserSDK` 0.1.0,
platform `linux-amd64-flatpak-kde-6.10`. It does not consume or modify the five
native `SpatialWebBrowserSDK` pins. The two SDKs export the same public header
and `SpatialWeb::Browser` target, through distinct CMake package names.

The committed lock is `release/dependencies/spatialweb-flatpak-browser.lock.json`.
It binds one exact source SHA, immutable source release, three asset size/digest
pins, GNU 15.2.0/C++17, all seven Qt component versions, and the immutable KDE
SDK, Platform and QtWebEngine BaseApp refs/commits. Core, Gui, Widgets, Network
and PrintSupport are 6.10.3; WebEngineCore and WebEngineWidgets are 6.10.2.
These are the explicitly qualified official KDE distribution components.
The native package still requires its original Qt 6.10.3 contract.

Prepare a fresh prefix with:

```sh
python3 -B release/packaging/prepare-flatpak-browser-sdk.py \
  --lock release/dependencies/spatialweb-flatpak-browser.lock.json \
  --verify-installed-refs --output /absolute/fresh/browser-sdk
```

`--local-assets DIR` uses the same pinned assets and still verifies every
attestation against the public producer's `flatpak-browser-sdk.yml` workflow,
exact source digest and dev ref. The ordinary preparer's bounded HTTPS transfer,
regular-file inventory and safe extraction implementation is shared. Neither
profile bundles Qt or Projection. The Flatpak metadata must record installed
runtime DOM qualification in addition to browser and relocated consumer tests.
The receipt binds the unchanged source lock bytes and all exact identities.

CI installs Flatpak dependencies in its isolated installation, then selects the
three exact commits emitted by `--print-ref-pins`. The preparer only prints or
reads refs; it never installs or updates them. `--verify-installed-refs` fails
closed if any selected installed ref differs. Desktop staging reads the entire
installed inventory and receipt back before copying only Browser runtime files
and notices. Set `DOCKPIPE_FLATPAK_BROWSER_SDK` to this verified prefix. Native
`DOCKPIPE_BROWSER_SDK` is intentionally not a Flatpak fallback.

The launcher selects the distinct package only with
`-DDOCKPIPE_BROWSER_ABI_PROFILE=flatpak-kde-6.10`. The Flatpak preflight checks all
seven actual CMake package versions, GNU compiler identity, and shared-library
resolution in the selected SDK/BaseApp. Installed tests run against the Platform;
build-only Chromium test flags do not alter installed browser sandbox settings.
