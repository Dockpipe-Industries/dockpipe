# Repository maintainer Makefile.
#
# This file is a convenience layer for working inside the Dockpipe source checkout.
# It is not the package/runtime contract itself. Product/package lifecycle behavior
# should live behind Dockpipe commands such as:
#   dockpipe build
#   dockpipe package build
#
# Main local dev entrypoint:
#   make -f tooling/Makefile build
#     Builds the standalone Dockpipe CLI.
#   make -f tooling/Makefile build-dockpipe-launcher
#     Builds the optional Dockpipe GUI shell separately.
#
# Contributors who want plain `dockpipe ...` on PATH should run:
#   make -f tooling/Makefile dev-install
# Thin maintainer aliases. The released bootstrap CLI orchestrates each build.
TOOLING_MAKEFILE := $(abspath $(lastword $(MAKEFILE_LIST)))
REPOSITORY_ROOT := $(abspath $(dir $(TOOLING_MAKEFILE))/..)
DOCKPIPE_WORKFLOW ?= python3 "$(REPOSITORY_ROOT)/workflows/shared/launch.py"
DEB_VERSION ?= $(shell test -f "$(REPOSITORY_ROOT)/VERSION" && tr -d ' \t\r\n' < "$(REPOSITORY_ROOT)/VERSION" || echo 0.6.0)
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
	$(DOCKPIPE_WORKFLOW) workflows/build/desktop-development/config.yml -- --output src/dockpipe-app/build

# Repo test sweep: core Go tests plus Dockpipe-owned package/workflow test hooks.
test:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build
	$(DOCKPIPE_WORKFLOW) workflows/ci/repository/config.yml

install-dockpipe-launcher:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build-dockpipe-launcher
	$(DOCKPIPE_WORKFLOW) workflows/install/desktop/config.yml

# Internal/local desktop helper: installs the repo-built Dockpipe GUI shell under ~/.local/share.
install-dockpipe-launcher-global:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build-dockpipe-launcher
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

# Developer loop: compile then install (same as: make -f tooling/Makefile build && make -f tooling/Makefile install).
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

# Terminal demo GIFs for sharing (requires asciinema + agg + Docker — see workflows/demo/record/README.md).
demo-record:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- all

demo-record-short:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- short

demo-record-long:
	$(MAKE) -C "$(REPOSITORY_ROOT)" -f "$(TOOLING_MAKEFILE)" build
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/config.yml -- long

# Optional dev tools: CI (govulncheck, gosec) + best-effort demo-record (asciinema, agg). Not for end users.
dev-deps:
	$(DOCKPIPE_WORKFLOW) workflows/setup/developer-dependencies/config.yml
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/dependencies.yml

# Optional tools for `make -f tooling/Makefile demo-record` only (see workflows/demo/record/README.md).
install-record-deps:
	$(DOCKPIPE_WORKFLOW) workflows/demo/record/dependencies.yml

