# Consumer CLI compatibility

Ordinary repositories should accept installed Dockpipe releases that support
their workflows. Set a compatibility floor in `dockpipe.config.json`:

```json
{
  "schema": 1,
  "min_dockpipe_version": "0.6.11"
}
```

This example accepts `0.6.11`, `0.6.33` and newer versions. Choose the oldest
released CLI that actually supports and has been tested with your workflow
features. This field does not install a CLI or prove compatibility with every
future release; workflow validation and qualification still matter.

To migrate a consumer project, **remove its legacy `dockpipe_version` exact pin**
and declare `min_dockpipe_version`. Merely adding the minimum leaves the exact
pin in force. Keep exact artifact versions, release URLs, checksums and sizes in
bootstrap/lock files used to reproduce CI, SDK builds or qualification. Review
hardcoded bootstrap scripts separately: their exact selected artifact is a
reproducibility choice, not an ordinary runtime compatibility check. Do not
replace checksum verification with a minimum-version check.

Producer repositories and qualification lanes may intentionally retain
`dockpipe_version`. Its meaning is unchanged: exact full identity, including
prerelease and build metadata. Both declarations may coexist when the exact pin
meets the floor, but a different executable still fails the exact check. An exact
pin below the minimum is a configuration error.

Both project fields require complete semantic versions without `v`, whitespace,
ranges, tags, null or empty values. Minimum comparisons follow SemVer precedence:

| Minimum | Executable | Result |
| --- | --- | --- |
| `0.6.33` | `0.6.33-rc.1` | Rejected: below the stable minimum |
| `0.6.33-rc.2` | `0.6.33-rc.10` | Accepted |
| `0.6.33` | `0.6.34-rc.1` | Accepted: newer release prerelease |
| `0.6.33+build.1` | `0.6.33+build.2` | Accepted: build metadata has no precedence |

Project constraints require a versioned executable: `dev` and `dev+hash` cannot
prove the floor and fail with guidance. Existing package manifests keep their
explicit unversioned-development bypass. Package `min_dockpipe_version` remains
an independent contract checked when loading or installing that package.

Admission uses the nearest configuration from the current directory or explicit
`--workdir`, before workflow resolution or execution. Help, version, diagnostics
and installation commands remain usable to repair a mismatch. A repository
`VERSION` file cannot impersonate the executable.

Older CLIs that predate the project minimum field may ignore it as an unknown
configuration key. Keep package-level floors and an appropriate consumer
bootstrap where those older executables must be excluded. Changing configuration
alone cannot retroactively add enforcement to an older CLI.

This engine change does not migrate external repositories, change producer pins,
or update exact bootstrap/lock artifacts. Consumer owners should apply the
configuration migration in their own repositories after selecting the required
release and workflow coverage.
