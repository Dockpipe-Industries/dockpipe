# VM guest preparation

The VM runtime accepts prepared disks (`DOCKPIPE_VM_BOOT_SOURCE=image`) or
installer media (`installer-iso`). Supply the installer with
`DOCKPIPE_VM_CDROM`, its destination with `DOCKPIPE_VM_DISK`, and the size of a
new disk with `DOCKPIPE_VM_DISK_SIZE`. These paths retain ordinary validation.
The runtime does not select a guest operating system.

`DOCKPIPE_VM_GUEST_PREPARATION` controls optional preparation:

| Value | Behavior |
| --- | --- |
| `automatic` (default) | Retains optional driver-media prompting, generated bootstrap and resolver Guest Tools. Existing Windows workflows retain this behavior. |
| `provided-only` | Uses explicitly supplied optional media and bootstrap payloads. It does not prompt for absent optional driver media, generate builtin bootstrap or inject resolver Guest Tools. |

`DOCKPIPE_RESOLVER_VM_GUEST_PREPARATION` supplies the resolver default when the
runtime value is unset. A workflow can select the policy through its existing
`vars` map; explicit CLI variables retain their normal precedence. Unknown values
fail before host dependency checks, prompts or VM state creation.

Required installer inputs remain required in either mode. Explicit
`DOCKPIPE_VM_VIRTIO_ISO` is still attached and validated. Explicit
`DOCKPIPE_VM_BOOTSTRAP_PATH` files or directories retain their supplied contents,
including package-provided seed media such as NoCloud data. In `provided-only`
mode no builtin reserved files or resolver tools are added or substituted.
Combining this policy with an unattended installer mode that generates builtin
bootstrap is rejected; select automatic preparation or supply your own seed.

The policy is independent of the host backend and adds no guest-OS selector.
Package metadata owns Linux media choices and input defaults. A local candidate
must bind the effective compiled-core script containing this contract; setting
the variable against an older runtime does not establish support.
