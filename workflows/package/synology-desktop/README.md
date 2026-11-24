# Synology full desktop bundle

Stages the existing Linux GUI and CLI DEBs into a browser desktop container build
context. The GUI executes inside the container with Webtop's Ubuntu XFCE desktop;
it is independent of the CLI SPK running directly on DSM. There is no GUI port or
Qt-to-web conversion. Source ownership and engine contracts remain unchanged.

The producer needs Python 3 and `dpkg-deb`, and performs no network or Docker
operations. Supply the actual DEBs from the same release and architecture:

```sh
"$DOCKPIPE_BIN" --workflow-file workflows/package/synology-desktop/config.yml -- \
  --cli-deb /absolute/release/dockpipe_VERSION_arm64.deb \
  --desktop-deb /absolute/release/dockpipe-desktop_VERSION_arm64.deb \
  --arch arm64 --output-dir /absolute/task-artifacts
```

The script can also run directly:

```sh
python3 workflows/package/synology-desktop/assets/scripts/stage.py \
  --cli-deb /absolute/release/dockpipe_VERSION_amd64.deb \
  --desktop-deb /absolute/release/dockpipe-desktop_VERSION_amd64.deb \
  --arch amd64 --output-dir /absolute/task-artifacts
```

Output: `dockpipe-desktop_VERSION_synology_ARCH.tar.gz`, containing constant DEB
filenames, Dockerfile, Compose configuration and setup instructions. The producer
checks control package names (`dockpipe`, `dockpipe-desktop`), matching exact
versions and requested architecture before creating output. Package filenames
are never interpreted as shell code. Existing output is replaced atomically only
after a complete archive has been written. Temporary files are cleaned on failure.

The default base is the LinuxServer `ubuntu-xfce` multiarchitecture index:

```text
lscr.io/linuxserver/webtop:ubuntu-xfce@sha256:432475b977d9ff06f8f8adfb9608541bbde51a566ac0cbe62af44f33265e1a37
```

The index was resolved through reviewed host registry access on 2026-11-02 and
contains amd64 and arm64 manifests. See [Webtop's upstream documentation](https://docs.linuxserver.io/images/docker-webtop/).
`--webtop-image` accepts a replacement explicitly pinned Ubuntu XFCE reference;
unpinned values fail. A replacement needs separate image/runtime qualification.
Compose preserves the upstream entrypoint, requires nonempty credentials and UID/
GID inputs, maps only HTTPS 3001 to loopback by default, uses 1 GiB shared memory,
and mounts only its persistent config directory. It requests neither privileged
mode nor host Docker access. Credentials are operator inputs, never release data.

Read [the bundled setup instructions](assets/SETUP.md) before starting the container.
The archive's bytes are reproducible for identical inputs. Apt resolution during
Docker build remains network-dependent, so this is not a reproducible image claim.
Staging/tests do not establish NAS browser, GPU or GUI execution qualification.

Local x64 qualification on 2026-11-02 used checksum-verified 0.6.35 staging DEBs:
the image built, the GUI's installation check and a CLI host workflow passed,
HTTPS rejected absent/wrong credentials, and user sign-in exposed the streamed
Dockpipe window with working native menus and embedded browser rendering. This
was a local Linux container test, not a DSM or native ARM64 acceptance result.
GUI-launched jobs, persistence/upgrade and NAS workload testing remain pending.

Run focused checks with:

```sh
python3 -m unittest discover -s workflows/package/synology-desktop/tests -v
"$DOCKPIPE_BIN" workflow validate workflows/package/synology-desktop/config.yml
```
