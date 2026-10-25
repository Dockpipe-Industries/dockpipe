# Repository maintainer Makefile.
#
# This file is a convenience layer for working inside the Dockpipe source checkout.
# It is not the package/runtime contract itself. Product/package lifecycle behavior
# should live behind Dockpipe commands such as:
#   dockpipe build
#   dockpipe package build
#
# Main local dev entrypoint:
#   make build
#     Builds the standalone Dockpipe CLI.
#   make build-dockpipe-launcher
#     Builds the optional Dockpipe Launcher separately.
#
# Contributors who want plain `dockpipe ...` on PATH should run:
#   make dev-install
# Thin maintainer aliases. The released bootstrap CLI orchestrates each build.
DOCKPIPE_WORKFLOW ?= python3 workflows/shared/launch.py
DEB_VERSION ?= $(shell test -f VERSION && tr -d ' \t\r\n' < VERSION || echo 0.6.0)
BUILD_VERSION ?= dev
WINDOWS_OUT ?= src/bin/dockpipe.exe

.PHONY: build build-windows test-core

build:
	$(DOCKPIPE_WORKFLOW) workflows/build/cli/config.yml -- --output src/bin/dockpipe.bin --alias src/bin/dockpipe --version "$(BUILD_VERSION)"

build-windows:
	$(DOCKPIPE_WORKFLOW) workflows/build/cli/config.yml -- --target windows/amd64 --output "$(WINDOWS_OUT)" --version "$(BUILD_VERSION)"

test-core:
	$(DOCKPIPE_WORKFLOW) workflows/ci/core/config.yml

.PHONY: \
	test \
	build-dockpipe-launcher install-dockpipe-launcher install-dockpipe-launcher-global \
	install dev-install test-quick check-paths deb deb-all demo-record demo-record-short demo-record-long \
	dev-deps install-record-deps ci package-dockpipe-language-support \
	package-vscode-language-support install-dockpipe-language-support

build-dockpipe-launcher:
	$(DOCKPIPE_WORKFLOW) workflows/build/desktop-development/config.yml -- --output src/app/tooling/dockpipe-launcher/build

# Repo test sweep: core Go tests plus Dockpipe-owned package/workflow test hooks.
test:
	$(MAKE) build
	$(DOCKPIPE_WORKFLOW) workflows/ci/repository/config.yml

install-dockpipe-launcher:
	$(MAKE) build-dockpipe-launcher
	$(DOCKPIPE_WORKFLOW) workflows/install/desktop/config.yml

# Internal/local desktop helper: installs the repo-built Dockpipe Launcher under ~/.local/share.
install-dockpipe-launcher-global:
	$(MAKE) build-dockpipe-launcher
	$(DOCKPIPE_WORKFLOW) workflows/install/desktop/config.yml -- --global

# Editor build and installation use the repository-owned Dockpipe workflows.
package-dockpipe-language-support:
	$(DOCKPIPE_WORKFLOW) workflows/editor.build/config.yml -- $(ARGS)

package-vscode-language-support: package-dockpipe-language-support

install-dockpipe-language-support:
	$(DOCKPIPE_WORKFLOW) workflows/editor.install/config.yml -- $(ARGS)

# Install pre-built binary to a local PATH directory (~/.local/bin, %USERPROFILE%\\bin, …). Does not run go build.
install:
	$(DOCKPIPE_WORKFLOW) workflows/install/developer-cli/config.yml

# Developer loop: compile then install (same as: make build && make install).
dev-install: build install

# Go + template guard + bash unit tests (no Docker). Faster than full CI.
test-quick:
	$(DOCKPIPE_WORKFLOW) workflows/ci/quick/config.yml

# Run the real GitHub Actions CI test job locally through nektos/act.
ci:
	$(DOCKPIPE_WORKFLOW) workflows/ci/ci-emulate/config.yml


# Docs/code guardrail: obsolete templates/core paths (pre-assets layout). See CONTRIBUTING.md.
check-paths:
	$(DOCKPIPE_WORKFLOW) workflows/ci/repository-layout/config.yml

# Debian packages (requires dpkg-deb). Default arch amd64; deb-all builds amd64 + arm64.
deb:
	$(DOCKPIPE_WORKFLOW) workflows/package/linux-cli/config.yml -- "$(DEB_VERSION)" amd64 --output release/artifacts

deb-all:
	$(DOCKPIPE_WORKFLOW) workflows/package/linux-cli/all.yml -- "$(DEB_VERSION)" --output release/artifacts

# Terminal demo GIFs for sharing (requires asciinema + agg + Docker — see release/demo/README.md).
demo-record:
	$(MAKE) build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- all

demo-record-short:
	$(MAKE) build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- short

demo-record-long:
	$(MAKE) build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- long

# Optional dev tools: CI (govulncheck, gosec) + best-effort demo-record (asciinema, agg). Not for end users.
dev-deps:
	$(DOCKPIPE_WORKFLOW) workflows/setup/developer-dependencies/config.yml
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/dependencies.yml

# Optional tools for `make demo-record` only (see release/demo/README.md).
install-record-deps:
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/dependencies.yml

