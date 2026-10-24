# VM boot display geometry

The generic QEMU VM runner accepts an optional standard-VGA EDID preference at
startup. Set `DOCKPIPE_VM_BOOT_DISPLAY_WIDTH` and
`DOCKPIPE_VM_BOOT_DISPLAY_HEIGHT` together. Resolver defaults use the corresponding
`DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_WIDTH` and
`DOCKPIPE_RESOLVER_VM_BOOT_DISPLAY_HEIGHT` names.

Both dimensions are empty by default, which adds no QEMU arguments. Values must
be plain decimal integers: width 640–4096, height 480–2160, with at most 4,194,304
pixels (a 16 MiB 32-bit framebuffer). For example, 1920×1080 and 2560×1600 fit;
3840×2160 exceeds this primitive's standard-VGA memory budget and is rejected.
Unpaired, malformed or oversized values fail before VM startup.

When configured on the virtual-display path, the runner creates one explicit
standard VGA device with `-device VGA,xres=WIDTH,yres=HEIGHT`. Device-scoped
properties retain the display; VGA class globals can suppress QEMU's implicit
VGA device. This does not increase video memory or enable GPU passthrough.

When a PCI device is selected as the primary passthrough GPU, the runner adds no
extra VGA device and leaves geometry to that GPU. Ordinary non-primary PCI
passthrough retains the virtual display. Other video devices need their own
package-owned configuration.

This is an EDID preference for the next VM start, not a live display-mode command.
Firmware and the guest driver decide which modes they expose and select. A
Windows Basic Display guest may retain a single mode; this setting does not
install a driver or prove that Windows will expose the preferred dimensions.
Guest-specific fields and presentation belong to the owning VM package.

For macOS host prerequisites and lifecycle behavior, see [Darwin VM runtime](vm-darwin.md).
