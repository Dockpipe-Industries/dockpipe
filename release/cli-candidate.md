# CLI-only bootstrap candidates

The package catalog needs the generic Python host-step and package-source-hook
contracts before it can build new native stores. A full Dockpipe release already
consumes published stores. This separate candidate lane breaks that dependency
cycle by publishing only the pure-Go CLI, with no store build or desktop tooling.
It does not change a repository pin automatically.

## Inputs and output

[The producer](packaging/build-cli-candidate.py) runs on Linux amd64 with the
existing pinned Go **1.26.10** toolchain. It uses a new module cache, the public
`https://proxy.golang.org` proxy and `sum.golang.org`, disables workspaces and
private/local proxy overrides, rejects module replacements, downloads/verifies
all dependencies and preserves the exact `go.mod`/`go.sum` bytes. Before any Go
command, the existing `embedded-inputs.py --check` must confirm the authored
embed manifest is current; the producer never silently regenerates it. The independently
published PipeLang module must therefore already match the committed checksum.
An unpublished module or checksum mismatch fails; there is no sibling checkout
or private-proxy fallback.

For `VERSION` equal to `0.6.0` and an exact source commit, the binary reports:

```text
0.6.0-staging.cli.shaFIRST_12_SOURCE_HEX_CHARACTERS
```

The `sha` prefix makes this a complete SemVer even if the hash happens to contain
only digits beginning with zero. The publisher binds the full source SHA to:

```text
cli-candidate-0.6.0-staging.FULL_40_CHARACTER_SOURCE_SHA
```

Five `CGO_ENABLED=0` builds produce the same flat executable archive format used
by the existing catalog `tooling/bootstrap_dockpipe.py` installer:

| Target | Archive | Member |
| --- | --- | --- |
| linux/amd64 | `dockpipe_VERSION_linux_amd64.tar.gz` | `dockpipe` |
| linux/arm64 | `dockpipe_VERSION_linux_arm64.tar.gz` | `dockpipe` |
| darwin/amd64 | `dockpipe_VERSION_darwin_amd64.tar.gz` | `dockpipe` |
| darwin/arm64 | `dockpipe_VERSION_darwin_arm64.tar.gz` | `dockpipe` |
| windows/amd64 | `dockpipe_VERSION_windows_amd64.zip` | `dockpipe.exe` |

Archives contain one regular executable, without directory prefixes, links or
optional runtime payloads. `cli-candidate.json` records version, source commit,
Go version, module-pin hashes and each archive's size/SHA-256 plus executable
size/SHA-256. Archive metadata is deterministic. Only Linux amd64 is executed
in this cross-build lane: exact `--version`, a public Python package source hook
and a public Python host workflow must all pass in an isolated fixture. The
other four outputs are cross-builds, not native Windows/macOS/arm64 acceptance.

## Publication and consumption

[The workflow](../.github/workflows/cli-candidate.yml) builds pull-request
artifacts without publication. An accepted `dev` push can attest all six files
and invoke the publisher. Manual events, other branches/repositories and malformed
source identities fail before network mutation. Publication uses the existing
bounded immutable candidate helpers: exact tag target, draft inventory lookup
by numeric ID, archive/metadata hash and size readback, resumable matching drafts,
prerelease and `--latest=false`. Conflicting bytes or tags are never replaced,
deleted or force-updated. No runtime/app release, Qt, stores, installers or
production-channel update is performed.

After actual publication, fetch the exact public archive and metadata and verify
all hashes before changing catalog inputs. Update **both** `dockpipe.config.json`
(`dockpipe_version`) and `dockpipe.lock.json` (`version`, `release`, per-platform
URL, archive size/hash, executable name/size) to these exact immutable values.
The official GitHub release URL prefix is unchanged; no installer change or
alternate executable wrapper is needed. Native catalog bootstrap verifies each
selected executable's reported version before use.

A catalog pin update is a separate reviewed source change. Run fresh catalog
Linux/Windows builds and the actual native package qualification after that
update. Publish qualified stores, then bind their exact hashes in the full core
staging release. Do not point existing production users or latest aliases at a
CLI bootstrap candidate.

## Validation status

```bash
python3 -B -m unittest discover -s release/packaging/tests -p test_cli_candidate.py -v
```

Local tests cover deterministic installer formats, version identity, public
module isolation, replacement rejection, five-platform metadata/hash binding and
immutable publication recovery. They mock compilation/network boundaries; they
are not a public fresh-fetch or native cross-platform execution claim. An actual
accepted CI run, attestations and independent public release readback remain
required before a catalog pin can consume this candidate.
