package skillsrender

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTargetOverridesAndManagedUpdates(t *testing.T) {
	assetsDir := t.TempDir()
	sourceDir := filepath.Join(assetsDir, "skills", "demo-skill")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(sourceDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("skill.yml", "name: demo-skill\ndescription: Demo skill\n")
	write("instructions.md", "# Demo\n\n## Capture\n\nShared capture.\n\n## Delivery\n\nNative delivery.\n")
	write("instructions.claude.md", "## Delivery\n\nManual delivery.\n")
	env := map[string]string{"DOCKPIPE_ASSETS_DIR": assetsDir}

	for _, target := range []string{"claude", "codex", "generic"} {
		t.Run(target, func(t *testing.T) {
			outputDir := t.TempDir()
			render := func() string {
				t.Helper()
				var report bytes.Buffer
				if err := Run([]string{"--target", target, "--output", outputDir}, env, &report, &bytes.Buffer{}); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(report.String(), "skipped:") {
					t.Fatal(report.String())
				}
				filename := "SKILL.md"
				if target == "generic" {
					filename = "instructions.md"
				}
				content, err := os.ReadFile(filepath.Join(outputDir, "demo-skill", filename))
				if err != nil {
					t.Fatal(err)
				}
				managed, err := unchangedManagedRender(filepath.Join(outputDir, "demo-skill"), "demo-skill", target)
				if err != nil || !managed {
					t.Fatalf("render marker does not match output: managed=%v, err=%v", managed, err)
				}
				return string(content)
			}
			content := render()
			if !strings.Contains(content, "Shared capture.") {
				t.Fatal("override removed shared capture section")
			}
			if target == "claude" {
				if !strings.Contains(content, "Manual delivery.") || strings.Contains(content, "Native delivery.") {
					t.Fatalf("override did not replace delivery: %s", content)
				}
				write("instructions.claude.md", "## Delivery\n\nUpdated manual delivery.\n")
				if content := render(); !strings.Contains(content, "Updated manual delivery.") {
					t.Fatal("managed output did not pick up override update")
				}
			} else if !strings.Contains(content, "Native delivery.") || strings.Contains(content, "manual delivery.") {
				t.Fatalf("provider override leaked into %s: %s", target, content)
			}
		})
	}
}

func TestReplaceInstructionSections(t *testing.T) {
	base := "# Shared\n\n## First\n\nKeep first.\n\n## Second\n\nOld second.\n\n## Third\n\nKeep third."
	override := "## Second\n\nNew second.\n\n````text\n## Example heading\n```\n````\n\n~~~text\n## Another example\n~~~\n"
	got, err := replaceInstructionSections(base, override)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(base, "Old second.", strings.TrimSpace(strings.TrimPrefix(override, "## Second\n\n")), 1)
	if got != want {
		t.Fatalf("unexpected section replacement:\ngot: %s\nwant: %s", got, want)
	}
}

func TestRejectInvalidInstructionOverrides(t *testing.T) {
	for _, test := range []struct {
		name     string
		base     string
		override string
	}{
		{"empty", "## Delivery\nDefault", ""},
		{"preamble", "## Delivery\nDefault", "Extra text\n## Delivery\nOverride"},
		{"unknown", "## Delivery\nDefault", "## Typo\nOverride"},
		{"duplicate override", "## Delivery\nDefault", "## Delivery\nOne\n## Delivery\nTwo"},
		{"ambiguous shared heading", "## Delivery\nOne\n## Delivery\nTwo", "## Delivery\nOverride"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := replaceInstructionSections(test.base, test.override); err == nil {
				t.Fatal("invalid override was accepted")
			}
		})
	}
}
