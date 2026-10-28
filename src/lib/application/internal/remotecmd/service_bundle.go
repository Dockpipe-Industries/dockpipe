package remotecmd

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var bundleIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// A standalone CLI has no application to associate. For a bundled executable,
// use its own enclosing app rather than assuming the installed product's ID.
func serviceBundleInfoPath(executable string) string {
	directory := filepath.Dir(executable)
	if filepath.Base(directory) != "Helpers" && filepath.Base(directory) != "MacOS" {
		return ""
	}
	contents := filepath.Dir(directory)
	if filepath.Base(contents) != "Contents" || !strings.HasSuffix(filepath.Dir(contents), ".app") {
		return ""
	}
	return filepath.Join(contents, "Info.plist")
}

func validateServiceBundleIdentifier(identifier string) error {
	// Apple's bundle ID is a reverse-DNS string. Keep the generated launch-agent
	// field bounded independently of the enclosing application's plist contents.
	if len(identifier) > 255 || !bundleIdentifierPattern.MatchString(identifier) {
		return errors.New("application bundle identifier is invalid")
	}
	return nil
}
