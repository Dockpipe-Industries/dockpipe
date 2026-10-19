# MCP Registry discovery and configured servers

DockPipe can read the public Official MCP Registry and validate selected local or
remote server configuration. This is a bounded foundation for TASK-040, **not a
running gateway or server importer**. No package is installed, no process or remote
MCP connection starts, and no tools or authority are granted by these commands.

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

Not implemented here: installation, runtime/dependency resolution, trust approval,
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
