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

When configured, the runner adds only `-global VGA.xres=WIDTH` and
`-global VGA.yres=HEIGHT`. It does not replace a video device, alter another
adapter's properties, increase video memory or enable physical GPU passthrough.
Other video devices need their own package-owned configuration.

This is an EDID preference for the next VM start, not a live display-mode command.
Firmware and the guest driver decide which modes they expose and select. A
Windows Basic Display guest may retain a single mode; this setting does not
install a driver or prove that Windows will expose the preferred dimensions.
Guest-specific fields and presentation belong to the owning VM package.
