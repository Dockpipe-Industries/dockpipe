package packagecatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestFlatpakStoreSelectionAndPinnedInstall(t *testing.T) {
	const target = "linux-amd64-flatpak-org.kde.Platform-6.10"
	archive := testArchive(t, nil)
	_, _, entry := fixture(t, archive)
	for _, test := range []struct {
		name, request, declared string
		direct, wantError       bool
	}{
		{name: "qualified release", request: target, declared: target},
		{name: "qualified pinned store", request: target, declared: target, direct: true},
		{name: "native-only release", request: "linux-arm64-flatpak-org.kde.Platform-6.10", declared: target, wantError: true},
		{name: "unlabelled pinned store", request: target, direct: true, wantError: true},
		{name: "native pinned store", request: target, declared: "linux-amd64", direct: true, wantError: true},
		{name: "wrong runtime branch", request: target, declared: "linux-amd64-flatpak-org.kde.Platform-6.9", direct: true, wantError: true},
		{name: "native cannot install Flatpak store", request: "linux-amd64", declared: target, direct: true, wantError: true},
		{name: "legacy native store", request: "linux-amd64", direct: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/release.json", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"schema": 1, "stores": map[string]any{target: map[string]string{"manifest": "store/index.json"}}})
			})
			mux.HandleFunc("/store/index.json", func(w http.ResponseWriter, r *http.Request) {
				manifest := packagebuild.StoreBuildManifest{Schema: 1, Platform: test.declared}
				manifest.Packages.Workflows = []packagebuild.StoreArtifact{entry.StoreArtifact}
				json.NewEncoder(w).Encode(manifest)
			})
			mux.HandleFunc("/store/"+entry.Tarball, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
			server := httptest.NewTLSServer(mux)
			defer server.Close()
			client := NewClient()
			client.http.Transport = server.Client().Transport
			remote := server.URL + "/release.json"
			if test.direct {
				remote = server.URL + "/store/index.json"
			}
			catalog, err := client.Load(context.Background(), remote, test.request)
			if (err != nil) != test.wantError {
				t.Fatalf("Load error = %v; want error=%v", err, test.wantError)
			}
			if err != nil {
				return
			}
			pinned, err := client.Load(context.Background(), catalog.Manifest, test.request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Install(context.Background(), pinned, pinned.Packages[0], filepath.Join(t.TempDir(), "packages")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
