# Remote connectivity resolvers

Provider adapters for `dockpipe remote`. Core owns delivery, pairing, execution, and results.
Resolvers own provider login, endpoint setup, and tunnel configuration. DorkPipe retains scheduling.

`dockpipe.cloudflare.remote-edge` is the Cloudflare adapter (version 0.3.0). Browser login, named tunnel, and DNS setup are implemented;
setup prints progress and pairing guidance and preserves the desktop browser session. Core can
deliver selected workflows, assets and explicit package dependencies to workers that opt in with
`--allow-delivery`. Local loopback delivery is tested; live account and macOS service qualification
remain pending. Other providers can implement the same
contract without changing the broker protocol.

See [Remote nodes](../../docs/runtime/remote-nodes.md) for setup, pairing, limits, recovery,
credential handling, build/distribution, and verification.
