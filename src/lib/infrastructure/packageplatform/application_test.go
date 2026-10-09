package packageplatform

import "testing"

func TestApplicationIDForHostIntegration(t *testing.T) {
	for _, id := range []string{"", "--command=sh", "../../app", "com.example.App\nExecStart=sh", "com.example.App extra"} {
		t.Setenv("FLATPAK_ID", id)
		if _, err := ApplicationID(); err == nil {
			t.Fatalf("accepted invalid ID %q", id)
		}
	}
	t.Setenv("FLATPAK_ID", "com.example.App")
	if id, err := ApplicationID(); err != nil || id != "com.example.App" {
		t.Fatalf("valid ID rejected: %q %v", id, err)
	}
}
