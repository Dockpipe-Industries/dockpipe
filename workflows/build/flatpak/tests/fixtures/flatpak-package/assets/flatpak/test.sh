#!/usr/bin/env bash
set -euo pipefail
compiled="${1:?compiled store}"
package="$compiled/workflows/flatpak-example"
[[ "$("$package/assets/tooling/bin/example")" == 'bundled-dependency-ok' ]]
/app/bin/dockpipe --workflow-file "$package/config.yml" --workdir "$compiled"
