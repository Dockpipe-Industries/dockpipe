# Remote connectivity resolvers

Provider adapters for `dockpipe remote`. Core owns delivery, pairing, execution, and results.
Resolvers own provider login, endpoint setup, and tunnel configuration. DorkPipe retains scheduling.

`dockpipe.cloudflare.remote-edge` is the Cloudflare adapter (version 0.3.1). Browser login, named tunnel, and DNS setup are implemented;
setup prints progress and pairing guidance and preserves the desktop browser session. Core can
deliver selected workflows, assets and explicit package dependencies to workers that opt in with
`--allow-delivery`. Local loopback delivery is tested; live account and macOS service qualification
remain pending. Other providers can implement the same
contract without changing the broker protocol.

If `cloudflared` is missing, native setup offers the package-declared installer: Homebrew on macOS,
or Cloudflare's signed APT repository on Debian/Ubuntu/Pop!_OS. Installation requires approval and
APT uses sudo. The Flatpak Marketplace artifact includes `cloudflared`; it does not run host APT.

Cloudflare-owned credentials may be owner-readable (`0400`) or owner-readable/writable (`0600`).
Setup preserves those permissions, rejects shared or linked credentials, and reuses an existing
tunnel credential when resuming after a setup failure.

See [Remote nodes](../../docs/runtime/remote-nodes.md) for setup, pairing, limits, recovery,
credential handling, build/distribution, and verification.
