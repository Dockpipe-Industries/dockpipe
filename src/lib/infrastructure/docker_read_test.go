package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

func TestDockerReadValidationAndAuthorityPrecedeExecutableLookup(t *testing.T) {
	t.Setenv("DOCKPIPE_DOCKER_BIN", filepath.Join(t.TempDir(), "missing-docker"))
	for _, request := range []ContainerReadRequest{
		{Action: "exec", Kind: "container", Object: strings.Repeat("a", 64)},
		{Action: "inspect", Kind: "container", Object: "short"},
		{Action: "inspect", Kind: "network", Object: "--all"},
		{Action: "logs", Kind: "volume", Object: "data"},
		{Action: "inspect", Kind: "volume", Object: "--help"},
		{Action: "inspect", Kind: "volume", Object: "data\x00other"},
	} {
		if _, err := ReadContainer(context.Background(), request); err == nil {
			t.Fatal("invalid read accepted")
		}
	}
	request := ContainerReadRequest{Action: "inspect", Kind: "container", Object: strings.Repeat("a", 64)}
	target, identity := nativeinvocation.Local()
	denied := errors.New("denied")
	_, err := ReadContainerWithAuthority(context.Background(), request, target, identity, func(_ context.Context, binding invocation.Binding) error {
		if binding.Contract.Action != "inspect" || len(binding.Digest) != 64 {
			t.Fatal("detail read is not exactly bound")
		}
		return denied
	})
	if !errors.Is(err, denied) {
		t.Fatalf("authority denial lost: %v", err)
	}
	target = invocation.Target{Kind: invocation.Remote, ID: "selected", Fingerprint: strings.Repeat("b", 64), Generation: strings.Repeat("c", 64)}
	if _, err := ReadContainerWithAuthority(context.Background(), request, target, identity, nil); err == nil {
		t.Fatal("remote read without authorization accepted")
	}
}

func dockerReadFixture(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	path := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", path)
}

func TestDockerReadExactArgumentsIdentityAndOutput(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, kind := range []string{"container", "network", "volume"} {
		t.Run(kind, func(t *testing.T) {
			object := id
			field := "Id"
			if kind == "volume" {
				object, field = "data", "Name"
			}
			dockerReadFixture(t, `test "$#" = 3 && test "$1" = "`+kind+`" && test "$2" = inspect && test "$3" = "`+object+`" || exit 8
printf '%s' '[{"`+field+`":"`+object+`","fixture":true}]'
`)
			result, err := ReadContainer(context.Background(), ContainerReadRequest{Action: "inspect", Kind: kind, Object: object})
			if err != nil || result.Schema != "dockpipe.containers/v1" || result.Object != object || !strings.Contains(string(result.Inspect), `"fixture":true`) {
				t.Fatalf("invalid inspect: %+v (%v)", result, err)
			}
		})
	}
	dockerReadFixture(t, `test "$#" = 5 && test "$1:$2:$3:$4:$5" = 'container:logs:--tail:200:`+id+`' || exit 8
printf stdout
printf stderr >&2
`)
	result, err := ReadContainer(context.Background(), ContainerReadRequest{Action: "logs", Kind: "container", Object: id})
	if err != nil || !strings.Contains(result.Logs, "stdout") || !strings.Contains(result.Logs, "stderr") {
		t.Fatalf("invalid logs: %+v (%v)", result, err)
	}
	dockerReadFixture(t, `printf '%s' '[{"Id":"`+strings.Repeat("b", 64)+`"}]'`)
	if _, err := ReadContainer(context.Background(), ContainerReadRequest{Action: "inspect", Kind: "container", Object: id}); err == nil {
		t.Fatal("different inspected identity accepted")
	}
}

func TestDockerReadOutputLimitAndCancellation(t *testing.T) {
	for _, script := range []string{
		`head -c 524289 /dev/zero`,
		`exec sleep 30`,
	} {
		t.Run(script, func(t *testing.T) {
			dockerReadFixture(t, script)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			started := time.Now()
			result, err := ReadContainer(ctx, ContainerReadRequest{Action: "logs", Kind: "container", Object: strings.Repeat("a", 64)})
			if err == nil || result.Schema != "" || time.Since(started) > 2*time.Second {
				t.Fatalf("failed read escaped its bound: schema=%q bytes=%d error=%v", result.Schema, len(result.Logs), err)
			}
		})
	}
	output := &dockerReadOutput{}
	if n, err := output.Write(make([]byte, maxDockerReadBytes)); err != nil || n != maxDockerReadBytes {
		t.Fatal("exact output bound rejected")
	}
	if _, err := output.Write([]byte{1}); err == nil || output.buffer.Len() != maxDockerReadBytes {
		t.Fatal("oversized output accepted")
	}
}
