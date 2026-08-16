package wslbridge

import "testing"

// TestBashSingleQuote escapes strings for safe bash single-quoted literals.
func TestBashSingleQuote(t *testing.T) {
	if got := bashSingleQuote(`hello`); got != `'hello'` {
		t.Fatalf("got %q", got)
	}
	if got := bashSingleQuote(`it's`); got != `'it'\''s'` {
		t.Fatalf("got %q", got)
	}
}

// TestPathToWSLFallback maps Windows paths to /mnt/c/... and normalizes UNC for WSL.
func TestPathToWSLFallback(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"C:/Users/me/repo", "/mnt/c/Users/me/repo"},
		{"c:/tmp/x", "/mnt/c/tmp/x"},
		{`C:\Users\me\repo`, "/mnt/c/Users/me/repo"},
		{`C:\`, "/mnt/c"},
		{`d:\`, "/mnt/d"},
		// UNC: must not run through filepath.Clean on Unix (would become "/server/share").
		{`\\srv\share\path`, "//srv/share/path"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := PathToWSLFallback(tc.in); got != tc.want {
				t.Fatalf("PathToWSLFallback(%q) = %q want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestIsUNCPathNormalized detects //server/share style paths used after UNC normalization.
func TestIsUNCPathNormalized(t *testing.T) {
	if !isUNCPathNormalized("//srv/share") {
		t.Fatal("expected UNC")
	}
	if isUNCPathNormalized("/mnt/c/x") {
		t.Fatal("linux path is not UNC")
	}
	if isUNCPathNormalized("//") {
		t.Fatal("too short")
	}
}

// TestBuildBashForwardScript builds the inner bash -lc script for WSL bridge (cd + exec dockpipe).
func TestBuildBashForwardScript(t *testing.T) {
	got := buildBashForwardScript("/mnt/c/proj", []string{"--", "echo", "a b"})
	want := "cd '/mnt/c/proj' && exec dockpipe '--' 'echo' 'a b'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestTranslatorForwardScript ensures translated argv with spaces survives bash quoting.
func TestTranslatorForwardScript(t *testing.T) {
	argv := []string{"--workdir", `C:\Program Files\repo`, "--", "echo", "ok"}
	got := testTranslator().ForwardScript("/mnt/c/proj", argv)
	want := "cd '/mnt/c/proj' && exec dockpipe '--workdir' '/mnt/c/Program Files/repo' '--' 'echo' 'ok'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
