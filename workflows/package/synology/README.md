# Synology SPK producer

Build a DSM 7+ package from an already produced Linux CLI:

```sh
python3 workflows/shared/launch.py workflows/package/synology/config.yml -- \
  0.6.34 amd64 --binary /absolute/path/to/dockpipe-linux-amd64 --output /tmp/synology-packages
```

Repeat with `arm64` and its matching binary. The producer checks ELF64 machine
identity and allows only the architecture's standard glibc loader and libc,
libdl and libpthread dependencies. Native authentication uses purego/dlopen, so
`CGO_ENABLED=0` does not imply a static binary. Actual DSM library compatibility
remains a hardware qualification requirement. The producer needs Python 3,
GNU `readelf` (binutils) for dynamic binaries and the existing product icons;
it does not need Docker, a Synology
toolchain, root privileges or a NAS. The normal Linux platform release phase
packages the exact CLI it just built, for each architecture.

Outputs are `dockpipe_VERSION_synology_x64.spk` and
`dockpipe_VERSION_synology_arm64.spk`. DSM's `INFO` architecture families are
`x86_64` and `armv8`, respectively; the latter is the DSM family name for supported
64-bit ARM platforms, not a Go target. `INFO` uses numeric `VERSION-1` (packaging
revision 1). Changing packaging for an already published version requires a new
runtime release; published bytes must not be replaced.

The uncompressed outer tar contains `INFO`, the gzip payload, lifecycle scripts,
package icons and `conf/privilege`. The payload contains `bin/dockpipe` and its
license under `share/doc/dockpipe/`.
Both archives use fixed metadata and gzip timestamps. The legacy DSM MD5 field
checks `package.tgz`; the release's `SHA256SUMS.txt` covers the whole SPK
against the selected release. Packaging writes to a temporary directory and
atomically replaces the output only after success.

DSM owns extraction and removal. All package scripts use the package identity,
with no elevated script overrides, global symlinks, enrollment, service setup or
changes to user homes. Start/status indicate an available CLI package; there is
no resident process and stop is disabled in Package Center.

See [installation and NAS qualification](../../../docs/install-synology.md).
Archive/unit checks and Linux CLI execution do not prove DSM installation,
remote delivery or NAS workflow compatibility. SynoCommunity submission and
publication remain separate from producing these release artifacts.

Format references: [DSM INFO](https://help.synology.com/developer-guide/synology_package/INFO.html),
[architecture families](https://help.synology.com/developer-guide/appendix/platarchs.html),
[DSM 7 changes](https://help.synology.com/developer-guide/breaking_changes.html),
[privilege configuration](https://help.synology.com/developer-guide/privilege/privilege_config.html).
