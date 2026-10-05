# Remote connectivity resolvers

Provider adapters for `dockpipe remote`. Core owns delivery, pairing, execution, and results.
Resolvers own provider login, endpoint setup, and tunnel configuration. DorkPipe retains scheduling.

Cloudflare is the first adapter. Browser login, named tunnel, and DNS setup are implemented;
live account and macOS service qualification remain pending. Other providers can implement the same
contract without changing the broker protocol.

See [Remote nodes](../../docs/runtime/remote-nodes.md) for setup, pairing, limits, recovery,
credential handling, build/distribution, and verification.
