# Native Display SDK candidates

`DockPipeDisplaySDK` is the small core-owned QWidget display interface SDK. It
exports `LauncherDisplayPackage.h`, `LauncherNativeLibraryScope.h`, relocated
CMake imports, license and candidate metadata. It does not distribute the
Launcher UI library, Qt, GIO, a display backend or a VM application. Packages
implement the display interface and own their native runtime closures.

## Qualification and publication

[The candidate workflow](../.github/workflows/display-sdk-candidate.yml) builds
Qt **6.10.3** consumers on Linux amd64, Windows amd64, macOS amd64 and macOS arm64.
It uses aqtinstall **3.3.0** and the same native Qt kit families as the desktop
release, without WebEngine. Linux arm64 has no qualified acquisition/CI lane in
this workflow; do not infer its support from the portable headers.

Each build installs the seven-file SDK component, relocates it, compiles and
executes an external consumer using the exact supplied Qt kit, and verifies the
consumer's native machine architecture. The producer rejects runtime payloads,
links, wrong targets and payloads beyond its 64-file/2-MiB contract. The source
identity binds the headers, leaf CMake/consumer sources and archive tooling;
`source_sha` identifies the caller-supplied committed checkout. The workflow
provides that checkout's exact `GITHUB_SHA`.

Pull requests qualify artifacts only. An accepted `dev` push qualifies all four
native matrix jobs before publication jobs can attest and publish their matching
platform artifacts. The publisher separately rejects manual dispatch, other
repositories/branches/events and malformed source identities. This workflow has
no manual publication dispatch or production-channel promotion.

Each platform gets its own immutable prerelease tag:

```text
display-sdk-1.0.0-PLATFORM-staging.FULL_40_CHARACTER_SOURCE_SHA
```

The tag is bound to that exact source commit. Its three assets are:

```text
DockPipeDisplaySDK-1.0.0-PLATFORM.tar.gz
sdk-files.json
sdk-release.json
```

Separate platform tags prevent collisions between metadata asset names. The
publisher checks product/version/interface/Qt/platform/source/compiler identity,
archive size and SHA-256, manifest SHA-256 and required native qualification
receipts. Archive delivery is capped at 4 MiB and each JSON asset at 128 KiB.

The existing Launcher SDK publication helpers supply bounded release inventory
lookup, exact commit/tag verification, sanitized command failures and remote
asset hash/size comparison. A new release is created as a draft, checked by its
numeric release ID, then made a prerelease with `--latest=false`. Complete
matching drafts resume; matching published releases are read back without
replacement. Any remote tag or asset mismatch fails instead of uploading,
clobbering, deleting or moving it. Final release and tag readback are required.
Attestations cover the archive and both metadata files.

Consumers must select a particular platform/source tag and verify its archive
and manifest hashes; discovery of a newer candidate does not update an existing
lock. A Display SDK candidate does not publish catalog packages, update core
package-store pins or release the Dockpipe application.

## Local checks and remaining CI proof

Run the producer and publication contract tests:

```bash
python3 -B -m unittest discover -s release/packaging/tests -p test_display_sdk_producer.py -v
python3 -B -m unittest discover -s release/packaging/tests -p test_publish_display_sdk.py -v
```

The publisher can run only in its accepted CI event context; there is no local
force or bypass flag. Adding these workflows is not evidence that their hosted
jobs ran. All four hosted acquisition/build/relocation/native-run lanes,
attestation permissions and immutable release/readback still require an actual
accepted CI run. Prior task-local native SDK/VM proofs remain separate evidence.
