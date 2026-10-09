package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	clean := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
		{Key: "vcs.modified", Value: "false"},
	}}
	dirty := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
		{Key: "vcs.modified", Value: "true"},
	}}
	for _, test := range []struct {
		name, override, expected string
		info                     *debug.BuildInfo
	}{
		{"clean", "dev", "dev+0123456789ab", clean},
		{"dirty", "dev", "dev+0123456789ab.dirty", dirty},
		{"empty-override", "", "dev+0123456789ab.dirty", dirty},
		{"no-vcs", "dev", "dev+unknown", &debug.BuildInfo{}},
		{"no-metadata", "dev", "dev+unknown", nil},
		{"release", "0.6.12", "0.6.12", dirty},
		{"staging", "0.6.12-staging.123.1.0123456789ab", "0.6.12-staging.123.1.0123456789ab", dirty},
	} {
		t.Run(test.name, func(t *testing.T) {
			if actual := buildVersion(test.override, test.info); actual != test.expected {
				t.Fatalf("got %q, want %q", actual, test.expected)
			}
		})
	}
}
