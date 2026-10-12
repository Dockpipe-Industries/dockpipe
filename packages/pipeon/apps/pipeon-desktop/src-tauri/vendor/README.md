# glib 0.18.5 security backport

Pipeon's GTK 0.18 dependency requires glib 0.18. The compatible upstream branch
still contains RUSTSEC-2024-0429; glib 0.20 cannot satisfy that requirement.

This directory contains the crates.io glib 0.18.5 source distribution, with the
upstream two-line fix applied to `src/variant_iter.rs`: the output pointer is
mutable and passed as `&mut p` to `g_variant_get_child`. All other source files
and the upstream MIT `LICENSE` are unchanged. Cargo's VCS metadata file is omitted.

- Archive: https://static.crates.io/crates/glib/glib-0.18.5.crate
- Archive SHA-256: `233daaf6e83ae6a12a52055f568f9d7cf4671dabb78ff9560ab6da230ce00ee5`
- Upstream source revision: `42b9caf98e03ded086362d9653ca58fe94dc8658`
- Reviewed fix: https://github.com/gtk-rs/gtk-rs-core/pull/1343
- Advisory: https://rustsec.org/advisories/RUSTSEC-2024-0429.html

The package version remains 0.18.5. This backport fixes the implementation; it
does not claim to be an upstream patched release or justify an advisory ignore.
Remove the Cargo patch and vendored source when the GTK/Tauri dependency graph
can use an upstream patched version, then run the same regression tests.

`tests/glib_variant.rs` exercises the affected iterator operations. The test
profile compiles glib at optimization level 3 so the regression is detectable.
Run from the repository root:

```sh
cargo test --locked --manifest-path packages/pipeon/apps/pipeon-desktop/src-tauri/Cargo.toml --test glib_variant
```
