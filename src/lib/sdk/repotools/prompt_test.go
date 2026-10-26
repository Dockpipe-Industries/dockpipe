package repotools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptRejectsTheWrongExistingResourceKind(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.txt", "second.txt"} {
		if err := os.WriteFile(filepath.Join(base, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name      string
		spec      PromptSpec
		answers   string
		want      string
		wantError string
	}{
		{
			name:      "file dialog rejects directory",
			spec:      PromptSpec{Type: "file", PathMode: "open-file"},
			answers:   "folder\nfirst.txt\n",
			want:      "first.txt",
			wantError: "File not found: folder",
		},
		{
			name:      "directory dialog rejects file",
			spec:      PromptSpec{Type: "file", PathMode: "open-dir"},
			answers:   "first.txt\nfolder\n",
			want:      "folder",
			wantError: "Directory not found: first.txt",
		},
		{
			name:      "single file resource rejects directory",
			spec:      PromptSpec{Type: "resource", ResourceKind: "file"},
			answers:   "folder\nfirst.txt\n",
			want:      "first.txt",
			wantError: "File not found: folder",
		},
		{
			name:      "default resource kind is file",
			spec:      PromptSpec{Type: "resource"},
			answers:   "folder\nfirst.txt\n",
			want:      "first.txt",
			wantError: "File not found: folder",
		},
		{
			name:      "multiple file resources reject mixed kinds",
			spec:      PromptSpec{Type: "resource", ResourceKind: "file", ResourceSelection: "multi"},
			answers:   "first.txt;folder\nfirst.txt;second.txt\n",
			want:      `["first.txt","second.txt"]`,
			wantError: "File not found: folder",
		},
		{
			name:      "directory resource rejects file",
			spec:      PromptSpec{Type: "resource", ResourceKind: "directory"},
			answers:   "first.txt\nfolder\n",
			want:      "folder",
			wantError: "Directory not found: first.txt",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DOCKPIPE_SDK_PROMPT_MODE", "terminal")
			spec := test.spec
			spec.MustExist = true
			spec.BaseDir = base
			input, err := os.CreateTemp(t.TempDir(), "input-")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if _, err := input.WriteString(test.answers); err != nil {
				t.Fatal(err)
			}
			if _, err := input.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			output, err := os.CreateTemp(t.TempDir(), "output-")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			originalInput, originalError := os.Stdin, os.Stderr
			os.Stdin, os.Stderr = input, output
			defer func() {
				os.Stdin, os.Stderr = originalInput, originalError
			}()

			got, err := Prompt(spec)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Prompt = %q; want %q", got, test.want)
			}
			diagnostics, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(diagnostics), test.wantError) {
				t.Fatalf("missing rejection %q in %q", test.wantError, diagnostics)
			}
		})
	}
}

func TestResourceExistsPreservesAnyKindAndRejectsMissingPaths(t *testing.T) {
	base := t.TempDir()
	if !resourceExists(base, "any", ".") {
		t.Fatal("an existing directory must remain valid for any resource kind")
	}
	for _, kind := range []string{"file", "directory", "any"} {
		if resourceExists(base, kind, "missing") {
			t.Fatalf("missing path accepted for %s", kind)
		}
	}
}
