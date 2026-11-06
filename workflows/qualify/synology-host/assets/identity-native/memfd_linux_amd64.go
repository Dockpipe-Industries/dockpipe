//go:build linux && amd64

package main

// Linux x86-64 __NR_memfd_create, from asm/unistd_64.h.
const memfdCreateTrap = 319
