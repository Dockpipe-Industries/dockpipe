#!/usr/bin/env bash
set -euo pipefail
jobs="$1"
cd "$DOCKPIPE_SOURCE_ROOT"

GOMAXPROCS="$jobs" go test -p "$jobs" ./src/lib/infrastructure -run '^TestWorkspaceGit' -count=1
git_cli="$RUNNER_TEMP/dockpipe-git-runtime"
GOMAXPROCS="$jobs" go build -p "$jobs" -trimpath -ldflags='-s -w' -o "$git_cli" ./src/cmd
export DOCKPIPE_GIT_TEST_CLI="$git_cli"

python3 -m venv "$RUNNER_TEMP/aqt"
"$RUNNER_TEMP/aqt/bin/python" -m pip install aqtinstall==3.3.0
"$RUNNER_TEMP/aqt/bin/python" -m aqt install-qt mac desktop 6.10.3 clang_64 --outputdir "$RUNNER_TEMP/Qt" --modules qtwebengine qtwebchannel qtpositioning
export PATH="$RUNNER_TEMP/Qt/6.10.3/macos/bin:$PATH"
export CMAKE_PREFIX_PATH="$RUNNER_TEMP/Qt/6.10.3/macos"

case "$(uname -m)" in
  arm64) browser_platform=darwin-arm64 ;;
  x86_64) browser_platform=darwin-amd64 ;;
  *) echo 'Unsupported macOS Browser SDK platform' >&2; exit 1 ;;
esac
"$DOCKPIPE_BIN" --workflow-file workflows/dependencies/browser-sdk/config.yml --workdir "$DOCKPIPE_SOURCE_ROOT" -- \
  --lock workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json \
  --platform "$browser_platform" --output "$RUNNER_TEMP/browser-sdk"
export CMAKE_PREFIX_PATH="$RUNNER_TEMP/browser-sdk:$CMAKE_PREFIX_PATH"

python3 -m unittest discover -s workflows/build/desktop/tests -p test_native_symbol_boundaries.py -v
python3 workflows/qualify/native-desktop/assets/scripts/dmg-proof.py

build="$RUNNER_TEMP/dockpipe-native-symbols"
# Match the release configuration and collect bounded layout timings.
# Profiling is compiled out of normal product builds.
cmake -S src/dockpipe-app -B "$build" \
  -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=ON \
  -DCMAKE_CXX_FLAGS=-DDOCKPIPE_PROFILE_DASHBOARD_LAYOUT \
  "-DDOCKPIPE_RELEASE_VERSION=$(cat VERSION)" \
  "-DCMAKE_OSX_ARCHITECTURES=$(uname -m)" -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0
cmake --build "$build" --parallel "$jobs" --target \
  dockpipe-launcher dockpipe-launcher-ui qualification-workflow-launch-test \
  dashboard-workspace-test dashboard-browser-performance-test \
  terminal-shell-catalog-test terminal-session-test terminal-widget-test \
  workspace-package-manager-test project-files-graph-test project-files-markdown-test web-mermaid-renderer-test dashboard-files-test project-files-test dashboard-page-test launcher-status-bar-test git-commit-queue-test git-repository-conflict-test git-history-widget-test project-git-test browser-extensions-test browser-extension-package-test browser-extension-store-test browser-extension-crx-test browser-extension-compatibility-test browser-test canvas-widget-presentation-test
macdeployqt "$build/DockPipe.app" -always-overwrite
python3 workflows/package/macos/assets/scripts/protect-owned-binaries.py \
  "$build/DockPipe.app" "$RUNNER_TEMP/dockpipe-native-symbol-audit.json"

QT_LOGGING_RULES=qt.webenginecontext.debug=true QT_QUICK_BACKEND=software QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu \
ctest --test-dir "$RUNNER_TEMP/dockpipe-native-symbols" --verbose --no-tests=error --timeout 1200 --parallel "$jobs" \
  -R '^(qualification-workflow-launch|dashboard-(workspace|browser-performance|files|file-views)|terminal-(shell-catalog|session|widget)|workspace-package-manager|project-files(-graph|-markdown)?|web-mermaid-renderer|launcher-status-bar|browser|browser-canvas-mode|dashboard-canvas-widgets|canvas-widget-presentation|browser-extensions|browser-extension-(package|store|crx)|project-git|git-(commit-queue|repository-conflict|history-widget))$'
