package packagebuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure/packageplatform"
)

func TestBuildCompiledStore(t *testing.T) {
	root := t.TempDir()
	core := filepath.Join(root, "core")
	if err := os.MkdirAll(filepath.Join(core, "runtimes", "dockerimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, "package.yml"), []byte(`schema: 1
name: dockpipe.core
version: 1.2.3
title: Core
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: core
`), 0o644); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "package.yml"), []byte(`schema: 1
name: demo
hidden: true
version: 0.0.1
min_dockpipe_version: 0.6.14
title: Demo
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
provider: cloudflare
requires_capabilities: [cli.codex]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "config.yml"), []byte("name: demo\nrun: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	m, err := BuildCompiledStore(root, out, "9.9.9", "all")
	if err != nil {
		t.Fatal(err)
	}
	if m.Packages.Core != nil {
		t.Fatal("new stores must not export legacy core trees")
	}
	if len(m.Packages.Workflows) != 1 || m.Packages.Workflows[0].Name != "demo" || m.Packages.Workflows[0].Provider != "cloudflare" {
		t.Fatalf("workflows: %+v", m.Packages.Workflows)
	}
	if !m.Packages.Workflows[0].Hidden || m.Packages.Workflows[0].MinDockpipeVersion != "0.6.14" {
		t.Fatal("compiled store lost the minimum engine version")
	}
	b, err := os.ReadFile(filepath.Join(out, "packages-store-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var dec StoreBuildManifest
	if err := json.Unmarshal(b, &dec); err != nil {
		t.Fatal(err)
	}
	if !dec.Packages.Workflows[0].Hidden || dec.Packages.Workflows[0].MinDockpipeVersion != "0.6.14" {
		t.Fatal("catalog JSON lost the minimum engine version")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["store_root"]; present {
		t.Fatal("exported manifest contains the builder's private store path")
	}
	platform, err := packageplatform.Current()
	if err != nil || dec.Platform != platform {
		t.Fatalf("store platform = %q; want %q (%v)", dec.Platform, platform, err)
	}
	if dec.Packages.Core != nil {
		t.Fatal("new manifest must omit core")
	}
	assertStoreArchiveSizes(t, out, &dec)
	if len(dec.Packages.Workflows) != 1 || dec.Packages.Workflows[0].Provider != "cloudflare" {
		t.Fatalf("workflow provider in JSON: %+v", dec.Packages.Workflows)
	}
	if len(dec.Packages.Workflows[0].RequiresCapabilities) != 1 || dec.Packages.Workflows[0].RequiresCapabilities[0] != "cli.codex" {
		t.Fatalf("requires_capabilities in JSON: %+v", dec.Packages.Workflows)
	}
}

func TestBuildCompiledStoreFromTarballLayout(t *testing.T) {
	root := t.TempDir()
	coreDir := filepath.Join(root, "core")
	wfDir := filepath.Join(root, "workflows")
	resDir := filepath.Join(root, "resolvers")
	for _, dir := range []string{coreDir, wfDir, resDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	coreSrc := filepath.Join(t.TempDir(), "core-src")
	if err := os.MkdirAll(filepath.Join(coreSrc, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreSrc, "package.yml"), []byte(`schema: 1
name: dockpipe.core
version: 0.6.0
title: Core
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: core
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(coreSrc, filepath.Join(coreDir, "dockpipe-core-0.6.0.tar.gz"), "core"); err != nil {
		t.Fatal(err)
	}

	wfSrc := filepath.Join(t.TempDir(), "wf-src")
	if err := os.MkdirAll(wfSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfSrc, "package.yml"), []byte(`schema: 1
name: demo
version: 0.6.0
title: Demo
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
provider: cloudflare
requires_capabilities: [cli.codex]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfSrc, "config.yml"), []byte("name: demo\nnamespace: acme\nrun: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(wfSrc, filepath.Join(wfDir, "dockpipe-workflow-demo-0.6.0.tar.gz"), "workflows/demo"); err != nil {
		t.Fatal(err)
	}

	resSrc := filepath.Join(t.TempDir(), "res-src")
	if err := os.MkdirAll(filepath.Join(resSrc, "profile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resSrc, "package.yml"), []byte(`schema: 1
name: codex
version: 0.6.0
title: Codex
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: resolver
capability: cli.codex
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(resSrc, filepath.Join(resDir, "dockpipe-resolver-codex-0.6.0.tar.gz"), "resolvers/codex"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out")
	m, err := BuildCompiledStore(root, out, "9.9.9", "all")
	if err != nil {
		t.Fatal(err)
	}
	if m.Packages.Core != nil {
		t.Fatal("new stores must not copy historical core archives")
	}
	if len(m.Packages.Workflows) != 1 || m.Packages.Workflows[0].Name != "demo" {
		t.Fatalf("workflows: %+v", m.Packages.Workflows)
	}
	if len(m.Packages.Resolvers) != 1 || m.Packages.Resolvers[0].Name != "codex" {
		t.Fatalf("resolvers: %+v", m.Packages.Resolvers)
	}
	assertStoreArchiveSizes(t, out, m)
	if _, err := os.Stat(filepath.Join(out, "dockpipe-workflow-demo-0.6.0.tar.gz")); err != nil {
		t.Fatalf("expected workflow tarball copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "dockpipe-resolver-codex-0.6.0.tar.gz")); err != nil {
		t.Fatalf("expected resolver tarball copy: %v", err)
	}
}

func assertStoreArchiveSizes(t *testing.T, root string, manifest *StoreBuildManifest) {
	t.Helper()
	artifacts := append([]StoreArtifact{}, manifest.Packages.Workflows...)
	artifacts = append(artifacts, manifest.Packages.Resolvers...)
	if manifest.Packages.Core != nil {
		artifacts = append(artifacts, *manifest.Packages.Core)
	}
	for _, artifact := range artifacts {
		info, err := os.Stat(filepath.Join(root, artifact.Tarball))
		if err != nil {
			t.Fatal(err)
		}
		if artifact.SizeBytes <= 0 || artifact.SizeBytes != info.Size() {
			t.Fatalf("archive %s: reported %d bytes; actual %d", artifact.Tarball, artifact.SizeBytes, info.Size())
		}
	}
}

func TestBuildCompiledStoreAllowsEmptyStoreAndOmitsCore(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-store")
	output := t.TempDir()
	manifest, err := BuildCompiledStore(root, output, "1.2.3", "all")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Packages.Core != nil || len(manifest.Packages.Workflows) != 0 || len(manifest.Packages.Resolvers) != 0 {
		t.Fatalf("unexpected empty-store packages: %+v", manifest.Packages)
	}
	data, err := os.ReadFile(filepath.Join(output, "packages-store-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if _, exists := wire.Packages["core"]; exists {
		t.Fatal("empty store emitted a core entry")
	}
}

func TestBuildCompiledStoreRejectsRetiredCoreSlice(t *testing.T) {
	output := filepath.Join(t.TempDir(), "uncreated")
	if _, err := BuildCompiledStore(t.TempDir(), output, "1.2.3", "core"); err == nil {
		t.Fatal("retired core producer remained available")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("invalid core selection created output: %v", err)
	}
}

func TestStoreManifestStillDecodesHistoricalCore(t *testing.T) {
	var manifest StoreBuildManifest
	if err := json.Unmarshal([]byte(`{"schema":1,"packages":{"core":{"name":"core","version":"1.2.3","tarball":"dockpipe-core-1.2.3.tar.gz","sha256":"abc"}}}`), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Packages.Core == nil || manifest.Packages.Core.Version != "1.2.3" {
		t.Fatal("historical core metadata is no longer readable")
	}
}
