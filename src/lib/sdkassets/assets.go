// Package sdkassets owns the portable helpers exported by Dockpipe's SDK.
// These library resources contain no workflow or tool implementation.
package sdkassets

import "embed"

// Scripts is the fixed set of generic SDK helpers. Callers materialize only the
// resource they need; importing the SDK does not extract an asset tree.
//
//go:embed scripts/dockpipe-sdk.sh scripts/repo-tools.sh scripts/repo-tools.ps1 scripts/repo_tools.py scripts/package-source-build.sh
var Scripts embed.FS
