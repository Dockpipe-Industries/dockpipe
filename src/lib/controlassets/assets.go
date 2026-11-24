// Package controlassets contains the fixed native adapter for compiled workflow controls.
package controlassets

import "embed"

// Files contains the native consumer sources. The owning materializer supplies
// the generated PipeLang source and InputDecode.inc for a specific control.
//
//go:embed native
var Files embed.FS
