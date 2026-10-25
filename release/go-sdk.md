# Go compatibility SDK release

Dockpipe owns the Go SDK consumed by independently released catalog tools.
`workflows/build/go-sdk/assets/source.lock.json` identifies the exact committed
source, exported inventory, archive and manifest. Generic infrastructure, portable
SDK resources and public repository tools belong to this SDK; package and compiler
implementations remain in their owning repositories.

The Go compatibility SDK workflow checks out the literal locked source commit,
installs its declared Go toolchain and reproduces the exact archive and manifest.
A changed producer or dependency closure must reproduce those bytes, or the lock
must be updated from actual export and consumer qualification evidence.

Local reproduction runs through its owning Dockpipe workflow:

```sh
python3 workflows/shared/launch.py workflows/build/go-sdk/locked.yml -- \
  --source-root /path/to/committed-snapshot --output /fresh/output
```

The explicit source directory is a producer input, never a consumer fallback.

The registered CI workflow provides the manual entrypoint, including when the
reusable SDK workflow is absent from the default branch. A build-only run creates
CI artifacts without publication:

```sh
gh workflow run ci.yml --repo OWNER/REPOSITORY --ref dev \
  -f build_go_sdk=true -f publish_go_sdk=false
```

After reviewing the exact source, version and two asset hashes, request protected
publication with `build_go_sdk=false` and `publish_go_sdk=true`. Both inputs default
to false. The local reusable call uses the selected caller commit; normal CI runs
retain their existing checks.

Publication remains behind the `release` environment and its approval policy.
If that policy forbids self-review, the initiator and required reviewer must be
different authorized people. The workflow does not change that policy.

The publisher creates an immutable `dockpipe-go-sdk-<version>` prerelease at the
locked source. It can resume an exact draft or verify an already published exact
release; conflicting source, tags or bytes fail without deletion or replacement.
Final readback checks the tag, source and complete two-asset inventory. It does not
move a runtime or production channel.

Consumers pin the exact public repository, tag, archive and manifest checksums.
Prepared local artifacts and passing CI artifact builds do not establish that the
public release exists; verify the actual published assets before adopting pins.
