package localterminal

import (
	"path/filepath"
	"runtime"
	"strings"

	"dockpipe/src/lib/infrastructure/terminalsession"
)

// ValidateProfile checks only the private native-provider protocol's shape and
// resource bounds. It performs no filesystem lookup, ambient default resolution
// or policy evaluation. Counts use UTF-16 units to match the native provider and
// Windows process environment contract; the transport separately bounds bytes.
func ValidateProfile(profile terminalsession.Profile) error {
	if profile.ID == "" || utf16Units(profile.ID) > 128 || !filepath.IsAbs(profile.Executable) || !filepath.IsAbs(profile.Dir) || len(profile.Args) > 64 {
		return ErrProtocol
	}
	if profile.Route != "native" && profile.Route != "host" || profile.Route == "host" && profile.EnvironmentMode != "inherit-host" || profile.Route == "native" && profile.EnvironmentMode != "complete" {
		return ErrProtocol
	}
	maxEntries := 4096
	if profile.Route == "host" {
		maxEntries = 66
	}
	if len(profile.Env) > maxEntries {
		return ErrProtocol
	}
	total := 0
	for _, value := range []string{profile.ID, profile.Executable, profile.Dir} {
		if strings.ContainsRune(value, 0) {
			return ErrProtocol
		}
		total += utf16Units(value)
	}
	for _, value := range profile.Args {
		units := utf16Units(value)
		if strings.ContainsRune(value, 0) || units > 64*1024 {
			return ErrProtocol
		}
		total += units
	}
	environmentUnits := 0
	keys := make(map[string]bool, len(profile.Env))
	for _, entry := range profile.Env {
		units := utf16Units(entry)
		if strings.ContainsRune(entry, 0) || units > 32767 {
			return ErrProtocol
		}
		start := 0
		if strings.HasPrefix(entry, "=") && profile.Route == "native" {
			start = 1
		}
		separator := strings.IndexByte(entry[start:], '=')
		if separator < 1 {
			return ErrProtocol
		}
		key := entry[:start+separator]
		if runtime.GOOS == "windows" && profile.Route == "native" {
			key = strings.ToUpper(key)
		}
		if keys[key] {
			return ErrProtocol
		}
		keys[key] = true
		environmentUnits += units + 1
		total += units
	}
	if environmentUnits > 512*1024 || total > 768*1024 {
		return ErrProtocol
	}
	if profile.Route == "host" && (!keys["TERM"] || !keys["COLORTERM"]) {
		return ErrProtocol
	}
	return nil
}

func utf16Units(value string) int {
	units := 0
	for _, character := range value {
		units++
		if character > 0xffff {
			units++
		}
	}
	return units
}
