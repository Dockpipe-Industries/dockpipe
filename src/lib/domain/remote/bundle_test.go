package remote

import "testing"

func TestBundleRejectsUnsafePathsAndBounds(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a/../b", "a\\b", "a/.env", ".ssh/id_rsa", "cert.pem", "credentials.json", "dockpipe.config.json", "x\x00y", "a/ending.", "a/é"} {
		t.Run(name, func(t *testing.T) {
			bundle := Bundle{WorkflowFile: "config.yml", Files: []BundleFile{{Path: "config.yml"}, {Path: name}}}
			if bundle.Validate() == nil {
				t.Fatalf("accepted unsafe path %q", name)
			}
		})
	}
	for _, files := range [][]BundleFile{
		{{Path: "config.yml"}, {Path: "CONFIG.yml"}},
		{{Path: "config.yml"}, {Path: "a"}, {Path: "a/b"}},
		{{Path: "config.yml", Data: make([]byte, MaxBundleBytes+1)}},
		{{Path: "missing.yml"}},
	} {
		if (Bundle{WorkflowFile: "config.yml", Files: files}).Validate() == nil {
			t.Fatal("invalid bundle accepted")
		}
	}
}

func TestDeliveryIdentityIncludesSourcesAndArtifactSelection(t *testing.T) {
	bundle := Bundle{WorkflowFile: "config.yml", Files: []BundleFile{{Path: "config.yml", Data: []byte("first")}}}
	first, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle.Files[0].Data = []byte("second")
	second, _ := bundle.Digest()
	bundle.Artifacts = []string{"result.json"}
	third, _ := bundle.Digest()
	if first == second || second == third {
		t.Fatal("delivery identity omitted source or artifact selection")
	}
	request := Submission{ID: "job", Node: "node", BundleHash: first}
	if request.Validate() != nil {
		t.Fatal("valid delivery rejected")
	}
	request.Profile = "local"
	if request.Validate() == nil {
		t.Fatal("ambiguous authority accepted")
	}
}
