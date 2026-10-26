package infrastructure

import (
	"os"
	"testing"
)

// TestRenderBannerForWidth picks compact vs full stderr banner based on terminal width.
func TestRenderBannerForWidth(t *testing.T) {
	if got := renderBannerForWidth(69); got != compactBanner {
		t.Fatalf("width 69 should use compact banner")
	}
	if got := renderBannerForWidth(70); got != banner {
		t.Fatalf("width 70 should use full banner")
	}
}

// TestUseDockerInteractiveTTYNonTTYFiles returns false when stdin/stdout/stderr are regular files, not TTYs.
func TestUseDockerInteractiveTTYNonTTYFiles(t *testing.T) {
	a, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	c, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if useDockerInteractiveTTY(a, b, c) {
		t.Fatal("temp files are not terminals")
	}
}
