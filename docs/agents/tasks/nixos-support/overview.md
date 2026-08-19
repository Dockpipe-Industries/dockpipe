# TASK-031 NixOS Declarative-Host Support

## Goal

Make DockPipe usable and supportable on NixOS without assuming mutable FHS paths, conventional
system packages, or imperative service installation.

## Scope

- Select exact stable NixOS releases/architectures and decide whether nixpkgs unstable is unsupported
  or governed by a rolling freshness policy.
- Define supported installation and integration: Nix package/flake, declarative services, resolver
  tool discovery, store paths, shells, CLI, packages/workflows, runtimes, and diagnostics.
- Remove unjustified FHS, `/usr/bin`, mutable global-install, dynamic-linker, and PATH assumptions
  through general path/tool primitives rather than NixOS conditionals in engine code.
- Distinguish running DockPipe on NixOS from using Nix as a resolver or build tool elsewhere.

## Acceptance Criteria

- A reproducible Nix expression installs and runs all claimed surfaces on exact stable baselines.
- Engine/package boundaries remain generic, immutable-store behavior is respected, and generated
  state uses declared writable scopes.
- Unsupported channels or configurations fail clearly; TASK-025 records accepted claims and evidence.

## First Bounded Slice

Inventory FHS/path/linker/service/global-install assumptions and propose the package/flake plus test
matrix. Do not install NixOS, publish to nixpkgs, add CI, modify source, or claim support.
