//go:build linux && arm64

package main

// Linux AArch64 uses asm-generic __NR_memfd_create.
const memfdCreateTrap = 279
