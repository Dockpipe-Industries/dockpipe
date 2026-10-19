# MCP Registry discovery and configured servers

DockPipe can read the public Official MCP Registry and validate selected local or
remote server configuration. This is a bounded foundation for TASK-040, **not a
running gateway or server importer**. Discovery does not install anything. The
explicit local MCPB artifact commands below inspect or materialize a limited native
bundle subset; no process or remote MCP connection starts and no authority is granted.

## CLI

```sh
dockpipe mcp registry search --search filesystem --limit 30
dockpipe mcp registry versions --name org.example/server
dockpipe mcp registry show --name org.example/server --version 1.2.3
dockpipe mcp config validate --workdir /path/to/project
```

Commands return JSON and fail with a nonzero exit status on invalid input or
request failure. `search` requests latest versions. `show` defaults to `latest`;
its response includes the concrete version. Search returns one page and an opaque `next_cursor`; pass that value with
`--cursor` to request another page. The official version-history endpoint does not
define pagination; `versions` retrieves its bounded history in one request. The CLI does not crawl the registry, retry requests, or cache metadata.

The reader uses the official HTTPS origin and `/v0.1/servers`, named `/versions`,
and named `/versions/{version}` endpoints. It rejects redirects and reads at most
8 MiB per response, matching DockPipe's existing package catalog metadata budget.
An operation has a 15-second total deadline and honors cancellation. Search pages request at most 100 entries (30 by default); histories exceeding
100 entries fail explicitly rather than silently truncate. Search text is limited to 512 bytes and cursor
text to 4096 bytes. Exhaustion fails explicitly, without truncated success output.

Normalized output contains source, name, concrete version, title, description,
status and latest-version metadata. The complete upstream `document` is retained,
including packages, remotes, arguments, environment requirements and future
extensions. These declarations are untrusted data. A runtime hint or published
package argument is never converted into a command or authorization automatically.
Deprecated/deleted statuses remain explicit; registry presence is not an endorsement.

## One project configuration

The optional `mcp` property belongs to the existing `dockpipe.config.json`:

```json
{
  "schema": 1,
  "mcp": {
    "servers": {
      "language-service": {
        "transport": "stdio",
        "local": {
          "executable": "/opt/example/bin/language-server",
          "arguments": ["--workspace", "/work/project"],
          "working_directory": "/opt/example",
          "runtime": "host",
          "platform": "linux/amd64"
        }
      },
      "remote-service": {
        "transport": "streamable-http",
        "remote": {"url": "https://mcp.example.test/api"}
      }
    }
  }
}
```

Use an explicitly installed executable, an absolute working directory, a selected
DockPipe runtime profile and target `os/arch`. Windows uses drive-qualified paths;
UNC/device paths are not accepted in this first contract. Supported target labels
are Linux, macOS (`darwin`) and Windows with `amd64` or `arm64`. These labels describe
configuration, not tested runtime compatibility. The validator performs no file
existence, executable permission, runtime-profile resolution or package provisioning.

An optional `source` records `{ "registry": "https://registry.modelcontextprotocol.io",
"name": "org.example/server", "version": "1.2.3" }`. Record the concrete registry
version, never `latest`. This is provenance metadata, not integrity verification or
a package lock. Existing DockPipe package stores and lifecycle remain unchanged.

Select existing project `secrets.environments` by `secret_environment` on the local
or remote object. Remote `header_bindings` map header names to variables explicitly
bound in that selected environment, for example `"Authorization": "API_KEY"`.
References contain no resolved token values. Validation checks that the environment
and binding exist; it never calls a resolver, forwards credentials or prints them.
Future runtime owners must resolve only approved per-import bindings and enforce
credential/audience isolation. There is no ambient credential inheritance contract.

Remote URLs must be explicit HTTPS addresses without inline credentials, query
parameters, fragments or unresolved templates. Header names are checked for
case-insensitive duplicates, line injection and transport-owned fields. This
lexical validation is **not network isolation**: future connections must enforce
TASK-043 egress policy at DNS resolution and connect time, including redirects and
rebinding. Legacy SSE is explicitly unsupported.

The aggregate MCP configuration budget is 8 MiB before typed decoding, with 64 servers, 128 arguments per local server (8192 bytes
per argument), 4096-byte paths/URLs, and 32 remote header bindings. These initial
management limits keep the authored review surface finite; larger deployments need
an explicit capacity review. Unknown MCP fields are rejected. The
[JSON Schema](../../src/lib/infrastructure/schema/mcp-config.schema.json) supports
editor validation of the `mcp` property; domain validation additionally checks
platform-specific paths and references into the surrounding project's secrets.

`config validate` returns counts and `servers_started: false`, never configuration
values. Other project commands do not perform registry discovery; absent MCP config
adds no network, process or persistent service dependency.

## Remaining TASK-039/040 work

Not implemented here: Registry-selected installation, runtime/dependency resolution, trust approval,
managed process lifecycle, MCP initialization/tool discovery or forwarding, protocol
compatibility tests against actual servers, namespace collision with native tools,
capability profiles, authentication exchange, restart/reconnect, schema-change
approval, desktop integration, a gateway endpoint, REST or upstream MCP exposure.
Do not treat validation success as readiness to execute privileged imports.

The inspected staging baseline has gateway/import backlog contracts but no generic
upstream client implementation. DorkPipe's separately owned MCP product is not a
substitute for that missing capability. This change preserves existing engine,
model/domain/infrastructure/application ownership and adds no package registry,
package type, bucket, daemon, or configuration store.

## Local MCPB artifact adapter

```sh
dockpipe mcp bundle inspect --path server.mcpb --sha256 <reviewed-digest> --platform linux --arch amd64
dockpipe mcp bundle install --workdir /path/to/project --path server.mcpb --sha256 <reviewed-digest> --platform linux --arch amd64
```

These generic commands accept a local archive and mandatory expected digest. They
support MCPB 0.3 binary servers with an explicit compatible platform and contained
`${__dirname}/<entry_point>` command and no runtime provisioning or environment
injection. Arguments may be literal or one exact `${user_config.<name>}` directory
binding. Required, non-secret, single directory fields are supported through an
explicit operator JSON file (`--user-config bindings.json`) and `--workspace-root`.
For example, the JSON may contain `{"workspace":"/absolute/project"}`. Directory
values must exist and canonicalize within the canonical authorized root. Unknown
fields, secret fields, defaults, multiple values and partial interpolation fail
closed. The receipt binds effective arguments and the root with a configuration
digest; this is provenance, not execution authority. Configuration changes select
a separate immutable artifact destination. ELF64, PE and thin Mach-O headers must match
the selected `amd64`/`arm64` target; universal Mach-O is currently rejected. Header
validation identifies the target, not the trustworthiness or runnability of code.

Archive limits match the catalog installer: 512 MiB compressed, 2 GiB expanded,
100,000 members and 8 MiB metadata. Every member is streamed and CRC-checked;
traversal, links, special files, case aliases and Windows-reserved paths fail closed.
The installer first takes a private snapshot, validates it, extracts through a
root-bounded filesystem handle, then atomically publishes a digest-addressed
artifact beneath the existing project internal cache. It preserves the original
archive and inspection receipt. Existing destinations are never overwritten;
a different version/digest is a separate immutable artifact. Failed/cancelled staging
is removed. Ordinary existing cache cleanup owns these disposable artifacts.

This is artifact materialization, not an enabled import: project configuration,
secret bindings and launch grants remain unchanged. Do not treat the returned
executable path as proof of runtime authorization. Remote Registry artifact selection
and downloading, registration, update selection, approved sessions and dedicated
removal UX remain pending. No FreeFlight-specific package or alternate registry
has been added.

## Internal session owner and pending application integration

The missing Registry-to-session path is a platform implementation gap, not a
credential denial or an external approval failure. Local artifact installation is
tested separately; it does not establish a configured, authorized running server. The following inspected APIs explain why passing
validation cannot establish an installed, running server:

| Existing implementation | Reusable boundary and missing capability |
| --- | --- |
| [`packagecatalog.Client.Install`](../../src/lib/infrastructure/packagecatalog/install.go) | Downloads and verifies DockPipe catalog tarballs; requires the expected archive layout and matching `package.yml`. It does not accept MCPB ZIP bundles or Registry ecosystem packages. Do not relabel an upstream bundle as a DockPipe package to bypass validation. |
| [`InstallWithDependenciesUsingCodec`](../../src/lib/infrastructure/packagecatalog/install_dependencies.go) | Stages and publishes the existing catalog dependency closure. Its dependency resolver selects DockPipe catalog entries, not npm, PyPI, or MCPB runtime requirements. Preserve this store and dependency model. |
| [`ImageArtifactManifest`](../../src/lib/model/runtimeartifact/model.go) and [`maybeSkipDockerBuildForArtifact`](../../src/lib/application/run_image_artifact.go) | Represent and reuse materialized Docker images for compiled workflows. They are not a native MCPB installer or a long-lived upstream MCP session owner. |
| [`CompileSecurityPolicyForWorkflow`](../../src/lib/application/internal/runtimepolicy/security.go) and [`ApplyCompiledRuntimeManifest`](../../src/lib/application/internal/runtimepolicy/runtime.go) | Compile and apply runtime security to `RunOpts`. Several controls map to container options; a host process does not acquire equivalent isolation by naming a runtime profile. |
| [`runMCP`](../../src/lib/application/mcp_cmd.go), [`mcp` models](../../src/lib/model/mcp/model.go) and [domain validation](../../src/lib/domain/mcp/config.go) | Read Registry metadata and validate authored server contracts. Local bundle artifact inspection/installation is implemented; the internal `mcpsession` package supplies supervised protocol operations, but application launch authority, configured runtime adaptation and CLI/session exposure remain absent. |

The local MCPB binary/stdio adapter above implements bounded inspection and
installation. Registry-selected artifact downloading and project registration are
still pending; the existing Registry metadata remains inert.

[`mcpsession`](../../src/lib/infrastructure/mcpsession/session.go) now supplies an
**internal protocol owner**, not a CLI launch command or gateway:

- The official MIT-licensed [Go SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0)
  negotiates 2026-07-28 discovery with legacy initialization fallback. Dependency
  versions and module checksums are pinned in `go.mod`/`go.sum`.
- An application-supplied command factory must prepare an already authorized,
  context-bound command with explicit environment, absolute executable/cwd and
  unassigned stdio. Existing `process.RunObserved` owns Unix process groups and
  Windows kill-on-close jobs. This API does not approve a host process or apply a
  configured runtime by itself.
- Startup and requests have 30-second deadlines. Frames are bounded at 16 MiB;
  aggregate discovery and retained SDK tool metadata at 8 MiB, with 1,024 tools,
  pages and retained cursor slots. One operation runs at a time; concurrent
  submissions fail instead of queueing. Cache exhaustion requires an explicit
  reviewed reconnect. Stderr is discarded rather than retained or forwarded.
- Discovery returns sorted tool metadata and its SHA256. Calls require the
  caller's reviewed digest and recheck fresh metadata; upstream TTL hints cannot
  silently bypass that check. No tool annotations, roots, sampling, elicitation,
  secrets or Registry description grants execution authority.
- Stop closes protocol pipes, allows up to three seconds per shutdown phase,
  and forces process-tree cancellation when needed (nine seconds maximum).
  Crashes fail outstanding calls. Recovery requires a fresh explicit Start;
  no mutation or prior request is replayed. Request cancellation is not proof
  that a remote side effect was rolled back.

The exact next integration point is `runMCP` plus the existing runtime policy
compiler/application path: validate the selected project server and install
receipt, reverify launch bytes, bind an import-specific authority record, prepare
an enforceable runtime command, then call `mcpsession.Start`. The shipped runtime
profiles currently provide container/VM/package execution; there is no approved
native-host adapter for an imported bundle. Do not silently ignore the configured
runtime or manufacture a host grant. CLI start/invoke/stop/restart and gateway
forwarding remain unavailable until that adapter and approval linkage exist.

Authorization must bind the selected artifact digest/version, effective launch
arguments, workspace access, runtime policy fingerprint and per-import secret
references. Reuse DockPipe's runtime policy compilation/application and existing
approval mechanisms where applicable; add the missing import-specific approval
record and enforcement linkage before launch. Review of metadata, installation,
startup and individual tool authority are distinct decisions. An update that
changes those inputs or discovered tool schemas must not silently reuse a broader
grant. Refuse requested privileged execution when the selected runtime cannot
enforce its filesystem, network, process or credential restrictions. In
particular, `runtime: host` validation is not proof of a sandbox or authorization.
This follows [TASK-040's security boundary](../agents/tasks/third-party-mcp-import/overview.md)
and leaves gateway caller-profile enforcement outstanding.

The continuation's end-to-end gate must install a neutral controlled bundle through
the real artifact lifecycle, start it through the authorized runtime, initialize
MCP, verify exact discovered tool/result semantics, cancel a blocked call, recover
from a crash without replaying a mutation, shut down all descendants, and remove
only its owned artifacts. Negative cases must reject digest/platform mismatches,
unsafe ZIP entries, unsupported configuration, stale approval and secret leakage.
Run the supported OS matrix. Contract fixtures alone cannot satisfy this gate;
**no Registry installation/session end-to-end evidence exists in this PR**.
The internal owner is tested with neutral child-process fixtures for exact tool
results, explicit environment isolation, stale tool metadata, bounded caching,
request cancellation, crash/no replay, explicit replacement and idempotent stop.
These are component tests, not configured-runtime or gateway acceptance.

## Development verification

With the repository's pinned Go toolchain on PATH:

```sh
dockpipe --workflow mcp-registry.verify --no-compile-deps
```

The workflow runs model/domain/registry tests, focused CLI tests, static checks and
a CLI build in DockPipe artifact scope.
Fixtures test pagination encoding, version identity, complete metadata retention,
malformed/oversized responses, status errors without retries, cancellation, redirect
rejection, response cleanup, local/remote configuration, secret-reference boundaries
and JSON Schema shape. Live registry checks are separate from deterministic tests.

## Protocol references

- [Official Registry API](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/official-registry-api.md)
- [Generic Registry API](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/generic-registry-api.md)
- [Server metadata](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/server-json/generic-server-json.md)

These sources establish a discovery interface, not an installation or trust policy.

### Recorded verification

On macOS ARM64 with the repository-pinned Go 1.26.9, `mcp-registry.verify`
passed its contract tests, focused CLI tests, `go vet` and CLI build. A separate
read-only smoke check exercised official discovery, an exact-version lookup and
history for a returned entry. No downstream server was connected. Linux/Windows
execution and the repository-wide CI suite remain pending; this is not evidence
that TASK-039 or TASK-040 acceptance is complete.
