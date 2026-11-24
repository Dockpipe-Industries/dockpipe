package remote

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
)

func TestContainerDetailFeaturesRetainReadAuthorityAndStrictTargets(t *testing.T) {
	target := contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("b", 64), Generation: strings.Repeat("c", 64)}
	id := strings.Repeat("a", 64)
	for _, action := range []string{"inspect", "logs"} {
		request, err := NewFeatureRequest(target, "", "containers."+action, map[string]string{"kind": "container", "object": id}, time.Now().Add(time.Minute))
		if err != nil || validateFeaturePayload(request) != nil || contract.FeatureAccess(request.Action) != "read" {
			t.Fatalf("typed Docker read was not admitted: %v", err)
		}
		permissions := FeaturePermissions{Schema: contract.FeatureVersion, DockerRead: true}
		if !featureCatalogAllows(permissions.catalog(target), request) {
			t.Fatal("Docker read requires unnecessary control grant")
		}
		permissions.DockerRead = false
		if featureCatalogAllows(permissions.catalog(target), request) {
			t.Fatal("Docker read bypassed worker-local grant")
		}
		denied := errors.New("denied detail read")
		_, err = executeFeatureContainer(context.Background(), request,
			invocation.Target{Kind: invocation.Remote, ID: target.Node, Fingerprint: target.Fingerprint, Generation: target.Generation},
			invocation.Identity{Issuer: "authenticated", Subject: "operator"},
			func(_ context.Context, binding invocation.Binding) error {
				if binding.Target.ID != target.Node || binding.Target.Fingerprint != target.Fingerprint || binding.Target.Generation != target.Generation || len(binding.Digest) != 64 {
					t.Fatal("Docker read lost selected target or input binding")
				}
				return denied
			})
		if !errors.Is(err, denied) {
			t.Fatalf("Docker read bypassed admission: %v", err)
		}
	}
	for _, payload := range []string{
		`{"kind":"network","object":"short"}`,
		`{"kind":"volume","object":"--all"}`,
		`{"kind":"container","object":"` + id + `","container":"` + id + `"}`,
		`{"kind":"container","object":"` + id + `","args":["--all"]}`,
		`{"kind":"container","object":"` + id + `","object":"` + id + `"}`,
	} {
		request := contract.FeatureRequest{Action: "containers.inspect", Payload: json.RawMessage(payload)}
		if validateFeaturePayload(request) == nil {
			t.Fatalf("invalid detail selection accepted: %s", payload)
		}
	}
}

func TestRemoteContainerDetailsUseCapturedIdentityAndReadGrant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	id := strings.Repeat("a", 64)
	path := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s' '[{\"Id\":\""+id+"\"}]'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", path)
	target := contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("b", 64), Generation: strings.Repeat("c", 64)}
	request, err := NewFeatureRequest(target, "", "containers.inspect", map[string]string{"kind": "container", "object": id}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	result := executeFeature(context.Background(), FeaturePermissions{Schema: contract.FeatureVersion, DockerRead: true}, request,
		invocation.Identity{Issuer: "authenticated", Subject: "operator"}, func(context.Context, invocation.Binding) error { return nil })
	var receipt infrastructure.ContainerReadResult
	if result.Status != "success" || json.Unmarshal(result.Payload, &receipt) != nil || receipt.Object != id || receipt.Kind != "container" {
		t.Fatalf("invalid remote detail receipt: %+v", result)
	}
}
