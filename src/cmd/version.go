package main

import (
	"runtime/debug"
	"strings"
)

// Release builds supply Version with -ldflags "-X main.Version=X.Y.Z".
// Local builds use Go's embedded VCS metadata, never the working directory at runtime.
var Version = "dev"

func versionString() string {
	info, _ := debug.ReadBuildInfo()
	return buildVersion(Version, info)
}

func buildVersion(override string, info *debug.BuildInfo) string {
	if version := strings.TrimSpace(override); version != "" && version != "dev" {
		return version
	}

	revision := "unknown"
	modified := false
	if info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					revision = setting.Value
					if len(revision) > 12 {
						revision = revision[:12]
					}
				}
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
	}
	version := "dev+" + revision
	if modified {
		version += ".dirty"
	}
	return version
}
