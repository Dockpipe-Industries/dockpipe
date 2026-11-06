//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func writeInstallerPayloadFixture(t *testing.T, root string, binary []byte) string {
	t.Helper()
	for _, directory := range []string{filepath.Join(root, "bin"), filepath.Join(root, "desktop")} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	helper := filepath.Join(root, "bin/dockpipe-dsm-host")
	if err := os.WriteFile(helper, binary, 0755); err != nil {
		t.Fatal(err)
	}
	base := []byte("services:\n  dockpipe:\n    image: fixture-cache:local\n    pull_policy: never\n    volumes:\n      - /var/packages/dockpipe-desktop/var:/config\n")
	if err := os.WriteFile(filepath.Join(root, "desktop/compose.base.yml"), base, 0644); err != nil {
		t.Fatal(err)
	}
	return helper
}

func TestInstallerPayloadRootBindsTheExecutedHelperToOneSupportedLayout(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%t", nested), func(t *testing.T) {
			temporary := t.TempDir()
			payload := temporary
			if nested {
				payload = filepath.Join(temporary, "package")
			}
			helper := writeInstallerPayloadFixture(t, payload, []byte("fixture executable"))
			actual, err := installerPayloadRoot(temporary, helper)
			if err != nil || actual != payload {
				t.Fatal("did not bind the executed candidate payload", actual, err)
			}
			if _, err := installerPayloadRoot(temporary, filepath.Join(t.TempDir(), "bin/dockpipe-dsm-host")); err == nil {
				t.Fatal("accepted a helper outside this installer's two payload roots")
			}
		})
	}
	for _, invalid := range []string{"", "/", "relative/staging"} {
		if _, err := installerPayloadRoot(invalid, "/tmp/bin/dockpipe-dsm-host"); err == nil {
			t.Errorf("accepted invalid staging root %q", invalid)
		}
	}
	temporary := t.TempDir()
	helper := writeInstallerPayloadFixture(t, temporary, []byte("fixture executable"))
	writeInstallerPayloadFixture(t, filepath.Join(temporary, "package"), []byte("another fixture executable"))
	if _, err := installerPayloadRoot(temporary, helper); err == nil {
		t.Fatal("accepted ambiguous payload roots")
	}
}

func TestInstallerPayloadRootRejectsLinkedOrIncompleteCandidates(t *testing.T) {
	for _, entry := range []string{"bin/dockpipe-dsm-host", "desktop/compose.base.yml", "bin", "desktop"} {
		t.Run(entry, func(t *testing.T) {
			temporary := t.TempDir()
			payload := filepath.Join(temporary, "package")
			helper := writeInstallerPayloadFixture(t, payload, []byte("fixture executable"))
			path := filepath.Join(payload, entry)
			outside := filepath.Join(t.TempDir(), "moved")
			if err := os.Rename(path, outside); err != nil {
				t.Fatal(err)
			}
			if _, err := installerPayloadRoot(temporary, helper); err == nil {
				t.Fatal("accepted an incomplete candidate payload")
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if _, err := installerPayloadRoot(temporary, helper); err == nil {
				t.Fatal("accepted a candidate through a symbolic link")
			}
		})
	}
}

func TestInstallerPayloadRootMatchesPhysicalIdentityThroughAParentAlias(t *testing.T) {
	root := t.TempDir()
	physicalParent := filepath.Join(root, "physical parent")
	physicalPayload := filepath.Join(physicalParent, "installer")
	helper := writeInstallerPayloadFixture(t, physicalPayload, []byte("fixture executable"))
	alias := filepath.Join(root, "parent alias")
	if err := os.Symlink(physicalParent, alias); err != nil {
		t.Fatal(err)
	}
	logicalPayload := filepath.Join(alias, "installer")
	selected, err := installerPayloadRoot(logicalPayload, helper)
	if err != nil || selected != logicalPayload {
		t.Fatal("trusted parent alias did not bind to the same payload and helper inodes", selected, err)
	}
	if err := os.Rename(helper, helper+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(helper+".original", helper); err != nil {
		t.Fatal(err)
	}
	if _, err := installerPayloadRoot(logicalPayload, helper); err == nil {
		t.Fatal("matching parent alias must not admit a linked helper")
	}
}

func TestRealInstallerHelperRestoresVolumeStateInBothStagingLayouts(t *testing.T) {
	if os.Getuid() == 0 || os.Getgid() == 0 {
		t.Skip("real installer helper deliberately requires an ordinary package identity")
	}
	buildContext, cancelBuild := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelBuild()
	compilerRoot := exec.CommandContext(buildContext, "go", "env", "GOROOT")
	rootOutput, err := compilerRoot.Output()
	if err != nil {
		t.Fatalf("locate the selected Go toolchain: %v", err)
	}
	root := strings.TrimSpace(string(rootOutput))
	if !filepath.IsAbs(root) {
		t.Fatal("Go reported a non-absolute toolchain root")
	}
	built := filepath.Join(t.TempDir(), "dockpipe-dsm-host")
	build := exec.CommandContext(buildContext, filepath.Join(root, "bin/go"), "build", "-buildvcs=false", "-o", built, ".")
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual installer helper: %v\n%s", err, output)
	}
	binary, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	control, err := os.ReadFile("../scripts/package-control.sh")
	if err != nil {
		t.Fatal(err)
	}
	wizardTemplate, err := os.ReadFile("../install_uifile.sh")
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := os.ReadFile("../scripts/restore-discovery.sh")
	if err != nil {
		t.Fatal(err)
	}
	control = bytes.ReplaceAll(control, []byte("@RESTORE_DISCOVERY_HELPERS@"), discovery)
	wizardTemplate = bytes.ReplaceAll(wizardTemplate, []byte("@RESTORE_DISCOVERY_HELPERS@"), discovery)
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%t", nested), func(t *testing.T) {
			root := t.TempDir()
			temporary := filepath.Join(root, "installer staging")
			payload := temporary
			if nested {
				payload = filepath.Join(temporary, "package")
			}
			helper := writeInstallerPayloadFixture(t, payload, binary)
			base, err := os.ReadFile(filepath.Join(payload, "desktop/compose.base.yml"))
			if err != nil {
				t.Fatal(err)
			}
			volume := filepath.Join(root, "volume1")
			home := filepath.Join(volume, "@apphome/dockpipe-desktop")
			settings := filepath.Join(home, "desktop-settings")
			data := filepath.Join(volume, "@appdata/dockpipe-desktop")
			configuration := filepath.Join(volume, "@appconf/dockpipe-desktop")
			tools := filepath.Join(root, "probe tools")
			for _, directory := range []string{settings, data, configuration, tools} {
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
			}
			credentials := map[string]string{
				"username": "Retained.operator", "password": "fixture-only-password-0000",
				"uid": strconv.Itoa(os.Getuid()), "gid": strconv.Itoa(os.Getgid()),
			}
			for name, value := range credentials {
				if err := os.WriteFile(filepath.Join(settings, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			workspace := filepath.Join(data, "retained workspace")
			if err := os.WriteFile(workspace, []byte("preserved project contents"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"curl", "synosystemctl"} {
				if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\nexit 97\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			preinst := filepath.Join(root, "preinst")
			if err := os.WriteFile(preinst, control, 0755); err != nil {
				t.Fatal(err)
			}
			removed := filepath.Join(root, "absent package metadata")
			wizardOutput := filepath.Join(root, "wizard output.json")
			environment := append(os.Environ(),
				"PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
				"SYNOPKG_PKGINST_TEMP_DIR="+temporary, "SYNOPKG_PKGDEST_VOL="+volume,
				"SYNOPKG_PKGHOME="+filepath.Join(removed, "home"),
				"SYNOPKG_PKGVAR="+filepath.Join(removed, "var"),
				"SYNOPKG_PKGETC="+filepath.Join(removed, "etc"),
				"SYNOPKG_PKGDEST="+filepath.Join(removed, "target"), "SYNOPKG_PKG_STATUS=INSTALL",
				"SYNOPKG_TEMP_LOGFILE="+wizardOutput, "wizard_host_folder_action=keep", "wizard_host_folders=",
				"wizard_username=", "wizard_password=", "wizard_password_confirm=",
			)
			run := func(executable string, arguments ...string) ([]byte, error) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, executable, arguments...)
				command.Env = environment
				return command.CombinedOutput()
			}
			wizard := filepath.Join(root, "install_uifile.sh")
			wizardContent := strings.ReplaceAll(string(wizardTemplate), "@HOST_FOLDER_WIZARD@", `{"custom_render_name":"folder-fixture","custom_render_fn":"@DOCKPIPE_RESTORE_MODE@:@DOCKPIPE_RESTORE_VOLUMES@:@DOCKPIPE_CHECKED_VOLUMES@:@DOCKPIPE_UNKNOWN_VOLUMES@"}`)
			wizardContent = strings.ReplaceAll(wizardContent, "@INSTALL_WIZARD@", "[]")
			// Namespace the authored discovery function to this temporary filesystem;
			// production still invokes it with /. No package links or hints exist.
			quotedRoot := "'" + strings.ReplaceAll(root, "'", "'\\''") + "'"
			wizardContent = strings.Replace(wizardContent, "\nrender_install_wizard /\n", "\nrender_install_wizard "+quotedRoot+"\n", 1)
			if err := os.WriteFile(wizard, []byte(wizardContent), 0755); err != nil {
				t.Fatal(err)
			}
			wizardContext, cancelWizard := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelWizard()
			wizardCommand := exec.CommandContext(wizardContext, wizard)
			wizardCommand.Env = append(append([]string{}, environment...), "SYNOPKG_PKGDEST_VOL=", "SYNOPKG_PKGHOME=")
			if output, err := wizardCommand.CombinedOutput(); err != nil {
				t.Fatalf("render retained-volume restore wizard: %v\n%s", err, output)
			}
			wizardData, err := os.ReadFile(wizardOutput)
			if err != nil || !bytes.Contains(wizardData, []byte("Restore Dockpipe")) || !bytes.Contains(wizardData, []byte("restore:/volume1:")) || bytes.Contains(wizardData, []byte("wizard_password")) {
				t.Fatal("reinstall did not retain its sign-in through the normal restore wizard", err)
			}
			saved := filepath.Join(configuration, "host-folder.json")
			if err := os.WriteFile(saved, []byte("damaged retained manifest"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := run(preinst); err == nil {
				t.Fatal("actual helper did not read retained volume configuration")
			}
			original := []byte("{\n  \"schema\": \"dockpipe.synology_host_folders.v2\",\n  \"folders\": []\n}\n")
			if err := os.WriteFile(saved, original, 0600); err != nil {
				t.Fatal(err)
			}
			if output, err := run(preinst); err != nil {
				t.Fatalf("actual preinst and candidate helper failed: %v\n%s", err, output)
			}
			staged, err := readHostFolders(filepath.Join(payload, "desktop/host-folder.json"))
			if err != nil || len(staged.Folders) != 0 {
				t.Fatal("actual helper did not prepare this executed payload", staged, err)
			}
			compose, err := os.ReadFile(filepath.Join(payload, "desktop/compose.yml"))
			if err != nil || !bytes.Equal(compose, base) {
				t.Fatal("candidate preparation changed the cached image or built-in mounts", err)
			}
			retained, err := os.ReadFile(saved)
			if err != nil || !bytes.Equal(retained, original) {
				t.Fatal("preinst changed persistent configuration before the DSM project worker", err)
			}
			for name, value := range credentials {
				actual, err := os.ReadFile(filepath.Join(settings, name))
				if err != nil || string(actual) != value {
					t.Fatalf("retained sign-in or identity field %s changed", name)
				}
			}
			if contents, err := os.ReadFile(workspace); err != nil || string(contents) != "preserved project contents" {
				t.Fatal("retained workspace changed", err)
			}
			if _, err := os.Lstat(removed); !os.IsNotExist(err) {
				t.Fatal("preinst recreated or used the removed package metadata links")
			}
			// Exercise the same real helper's binding refusal as well: it must
			// never reinterpret this executable as another temporary installation.
			other := filepath.Join(root, "other installation")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mismatched := exec.CommandContext(ctx, helper, "folder-prepare")
			mismatched.Env = append(environment, "SYNOPKG_PKGINST_TEMP_DIR="+other)
			if output, err := mismatched.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("does not match")) {
				t.Fatalf("executed helper accepted an unrelated staging root: %v\n%s", err, output)
			}
		})
	}
}

func TestHostFolderRejectsSystemRootsAndInvalidSources(t *testing.T) {
	for _, path := range []string{"/", "/volume1", "/volume1/", "/etc", "/volume1/@appconf/dockpipe-desktop", "/volume1/share/@private", "/volume1/share/../other", "/volume1//share", "/volume1/share/", "/volume0/share", "/volume1/share\nservices:", "/volume1/share\x00suffix", "relative/share"} {
		if err := validateHostFolder(hostFolder{Schema: hostFolderSchema, Source: path, Access: "ro"}); err == nil {
			t.Errorf("accepted invalid host folder %q", path)
		}
	}
	for _, path := range []string{"/volume1/source", "/volume12/projects/My project", "/volume1/projects/$source:notes"} {
		if err := validateHostFolder(hostFolder{Schema: hostFolderSchema, Source: path, Access: "rw"}); err != nil {
			t.Errorf("rejected valid host folder %q: %v", path, err)
		}
	}
	for _, mode := range []string{"", "readwrite", "ro,rw", "rw\n"} {
		if validateHostFolder(hostFolder{Schema: hostFolderSchema, Source: "/volume1/source", Access: mode}) == nil {
			t.Errorf("accepted access mode %q", mode)
		}
	}
}

func TestHostFolderSelectionPreservesExplicitlyAndRemovesWithoutDeletion(t *testing.T) {
	saved := hostFolders{Schema: hostFoldersSchema, Folders: []hostFolder{{Source: "/volume1/projects", Name: "projects", Access: "ro"}}}
	for _, action := range []string{"", "keep"} {
		actual, err := selectHostFolders(saved, action, "ignored")
		if err != nil || !reflect.DeepEqual(actual, saved) {
			t.Fatal("keep changed the stored selection", actual, err)
		}
	}
	removed, err := selectHostFolders(saved, "remove", "")
	if err != nil || len(removed.Folders) != 0 {
		t.Fatal("remove must clear only the mapping", removed, err)
	}
	if _, err := selectHostFolders(saved, "set", ""); err == nil {
		t.Fatal("blank replacement path was accepted")
	}
	if _, err := selectHostFolders(saved, "unknown", ""); err == nil {
		t.Fatal("unknown installer operation was accepted")
	}
}

func TestHostFolderAccessRejectsSymlinkAncestorsAndChecksActualPermissions(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "projects")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := checkHostFolderAccess(folder, "rw"); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := checkHostFolderAccess(filepath.Join(alias, "projects"), "ro"); err == nil {
		t.Fatal("symlink ancestor was followed")
	}
	if err := checkHostFolderAccess(filepath.Join(root, "missing"), "ro"); err == nil {
		t.Fatal("missing directory was accepted")
	}
	if os.Geteuid() != 0 {
		parent := filepath.Join(root, "traverse-only")
		child := filepath.Join(parent, "readable-project")
		if err := os.MkdirAll(child, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(parent, 0100); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(parent, 0700)
		if err := checkHostFolderAccess(child, "rw"); err != nil {
			t.Fatal("ancestor requires traversal, not permission to list its siblings", err)
		}
		if err := os.Chmod(folder, 0500); err != nil {
			t.Fatal(err)
		}
		if err := checkHostFolderAccess(folder, "ro"); err != nil {
			t.Fatal("read-only folder rejected", err)
		}
		if err := checkHostFolderAccess(folder, "rw"); err == nil {
			t.Fatal("read/write mapping accepted without write permission")
		}
	}
}

func TestHostFolderComposeHasNamedTargetNoHostCreationAndEscapedSource(t *testing.T) {
	base := []byte("services:\n  dockpipe:\n    volumes:\n      - /var/packages/dockpipe-desktop/var:/config\n    restart: unless-stopped\n")
	for _, access := range []string{"ro", "rw"} {
		folder := hostFolder{Name: "chosen", Source: "/volume1/source/My $project: \"quoted\"", Access: access}
		rendered, err := renderHostFolderCompose(base, hostFolders{Schema: hostFoldersSchema, Folders: []hostFolder{folder}})
		if err != nil {
			t.Fatal(err)
		}
		var mapping struct {
			Type     string `json:"type"`
			Source   string `json:"source"`
			Target   string `json:"target"`
			ReadOnly bool   `json:"read_only"`
			Bind     struct {
				CreateHostPath bool `json:"create_host_path"`
			} `json:"bind"`
		}
		line := bytes.Split(rendered, []byte("\n"))[4]
		if json.Unmarshal(bytes.TrimPrefix(line, []byte("      - ")), &mapping) != nil {
			t.Fatal("mount is not a JSON-compatible YAML value")
		}
		if mapping.Type != "bind" || mapping.Target != "/host/chosen" || mapping.ReadOnly != (access == "ro") || mapping.Bind.CreateHostPath || mapping.Source != strings.ReplaceAll(folder.Source, "$", "$$") {
			t.Fatal("mount contract changed", mapping)
		}
		if !bytes.HasSuffix(rendered, []byte("    restart: unless-stopped\n")) || bytes.Count(rendered, []byte("/config")) != 1 {
			t.Fatal("unrelated service configuration changed")
		}
	}
	if _, err := renderHostFolderCompose(bytes.Repeat(base, 2), emptyHostFolders()); err == nil {
		t.Fatal("ambiguous configuration template was accepted")
	}
}

func TestHostFolderInstallerStagesBeforePersistingAndKeepsSavedStateOnFailure(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "package")
	desktop := filepath.Join(stage, "desktop")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	base := []byte("services:\n  dockpipe:\n    volumes:\n      - /var/packages/dockpipe-desktop/var:/config\n")
	if err := os.WriteFile(filepath.Join(desktop, "compose.base.yml"), base, 0644); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(root, "host-folder.json")
	if err := prepareHostFolder(stage, saved, "keep", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Fatal("preinst wrote persistent package state")
	}
	if err := saveHostFolder(stage, saved); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareHostFolder(stage, saved, "set", `[{"source":"/volume1/@appconf","access":"rw"}]`); err == nil {
		t.Fatal("unsafe mapping was accepted")
	}
	actual, _ := os.ReadFile(saved)
	if !bytes.Equal(actual, original) {
		t.Fatal("failed preflight changed saved state")
	}
	// Explicit removal can recover an obsolete or damaged manifest, without
	// silently interpreting that manifest as a valid empty selection.
	if err := os.WriteFile(saved, []byte("damaged selection"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareHostFolder(stage, saved, "keep", ""); err == nil {
		t.Fatal("keep silently accepted damaged settings")
	}
	if err := prepareHostFolder(stage, saved, "remove", ""); err != nil {
		t.Fatal(err)
	}
	rendered, _ := os.ReadFile(filepath.Join(desktop, "compose.yml"))
	if !bytes.Equal(rendered, base) {
		t.Fatal("removing the host mapping changed built-in mounts")
	}
	metadata, _ := os.Stat(saved)
	if metadata.Mode().Perm() != 0600 {
		t.Fatal("persisted mapping is not package-private")
	}
}

func TestHostFolderStoredSettingsRejectLinkedNonregularAndUnboundedInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "host-folder.json")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := readHostFolders(path); finished <- err }()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("FIFO settings were accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO settings blocked the installer")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for _, contents := range []string{`{"schema":"dockpipe.synology_host_folder.v1","extra":true}`, strings.Repeat("x", maximumHostFolderBytes+1), `{"schema":"wrong"}`} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readHostFolders(path); err == nil {
			t.Fatal("invalid saved settings accepted")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := readHostFolders(path); err == nil {
		t.Fatal("linked settings accepted")
	}
}

func TestHostFolderRejectsNestedMountsWithoutConfusingSiblingPrefixes(t *testing.T) {
	root := "1 0 0:1 / / rw - rootfs rootfs rw\n"
	for _, mounted := range []string{"/volume1/source/subdir", `/volume1/source/My\040Mount`} {
		table := []byte(root + "2 1 0:2 / " + mounted + " rw - tmpfs tmpfs rw\n")
		if err := rejectNestedHostMounts("/volume1/source", table); err == nil {
			t.Fatal("nested mount could escape read-only semantics", mounted)
		}
	}
	for _, mounted := range []string{"/volume1/source", "/volume1/source-other/subdir", "/volume1"} {
		table := []byte(root + "2 1 0:2 / " + mounted + " rw - tmpfs tmpfs rw\n")
		if err := rejectNestedHostMounts("/volume1/source", table); err != nil {
			t.Fatal("unrelated or selected mount rejected", mounted, err)
		}
	}
	for _, table := range [][]byte{nil, []byte("malformed"), bytes.Repeat([]byte("x"), maximumMountInfoBytes+1)} {
		if err := rejectNestedHostMounts("/volume1/source", table); err == nil {
			t.Fatal("invalid or exhausted mount inspection was accepted")
		}
	}
}

func TestHostFoldersDefaultNamesOverrideAndDuplicateHandling(t *testing.T) {
	configuration, err := selectHostFolders(emptyHostFolders(), "set", `[{
		"source":"/volume1/projects","name":"","access":"rw"
	},{"source":"/volume2/media","name":"videos","access":"ro"}]`)
	if err != nil || len(configuration.Folders) != 2 || configuration.Folders[0].Name != "projects" || configuration.Folders[1].Name != "videos" {
		t.Fatal("default basename or name override failed", configuration, err)
	}
	for _, entries := range []string{
		`[{"source":"/volume1/a","name":"same","access":"ro"},{"source":"/volume2/a","name":"same","access":"ro"}]`,
		`[{"source":"/volume1/a","access":"ro"},{"source":"/volume2/a","access":"ro"}]`,
		`[{"source":"/volume1/a","name":"../escape","access":"ro"}]`,
		`[{"source":"/volume1/a","name":".","access":"ro"}]`,
		`[{"source":"/volume1/a","name":"x\\y","access":"ro"}]`,
		`[{"source":"/volume1/a","name":"x\ny","access":"ro"}]`,
		`[{"source":"/volume1/a","name":"a","access":"ro","extra":true}]`,
		`[{"source":"/volume1/a","schema":"unexpected","access":"ro"}]`,
		`[]`, `null`,
	} {
		if _, err := selectHostFolders(emptyHostFolders(), "set", entries); err == nil {
			t.Errorf("accepted invalid or ambiguous mapping: %s", entries)
		}
	}
	configuration.Folders[0].Name = strings.Repeat("é", 65)
	if _, err := normalizeHostFolders(configuration); err == nil {
		t.Fatal("UTF-8 byte name limit was not enforced")
	}
	configuration.Folders[0].Name = "My projects"
	configuration.Folders[1].Name = "@media"
	if _, err := normalizeHostFolders(configuration); err != nil {
		t.Fatal("spaces and non-reserved names should remain supported", err)
	}
}

func TestHostFoldersBoundedCountAndIndependentMountModes(t *testing.T) {
	configuration := emptyHostFolders()
	for index := 0; index < maximumHostFolders; index++ {
		configuration.Folders = append(configuration.Folders, hostFolder{Source: "/volume1/source", Name: fmt.Sprintf("folder%d", index), Access: "ro"})
	}
	configuration.Folders[1].Access = "rw"
	if _, err := normalizeHostFolders(configuration); err != nil {
		t.Fatal(err)
	}
	base := []byte("services:\n  dockpipe:\n    volumes:\n      - /var/packages/dockpipe-desktop/var:/config\n")
	rendered, err := renderHostFolderCompose(base, configuration)
	if err != nil || bytes.Count(rendered, []byte(`"type":"bind"`)) != maximumHostFolders || bytes.Count(rendered, []byte(`"read_only":false`)) != 1 {
		t.Fatal("mapping count or independent read-only modes changed", err)
	}
	configuration.Folders = append(configuration.Folders, hostFolder{Source: "/volume1/extra", Name: "extra", Access: "ro"})
	if _, err := normalizeHostFolders(configuration); err == nil {
		t.Fatal("more than 16 mappings were accepted")
	}
	if _, err := selectHostFolders(emptyHostFolders(), "set", strings.Repeat(" ", maximumHostFolderBytes+1)); err == nil {
		t.Fatal("aggregate JSON budget was not enforced")
	}
}

func TestHostFoldersV1MigrationKeepsInstalledSourceAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host-folder.json")
	if err := os.WriteFile(path, []byte(`{"schema":"dockpipe.synology_host_folder.v1","source":"/volume1/projects","access":"rw"}`), 0600); err != nil {
		t.Fatal(err)
	}
	configuration, err := readHostFolders(path)
	if err != nil || len(configuration.Folders) != 1 || configuration.Folders[0].Name != "source" || configuration.Folders[0].Access != "rw" {
		t.Fatal("legacy alias was not preserved", configuration, err)
	}
	data, err := json.Marshal(configuration)
	if err != nil || bytes.Contains(data, []byte(hostFolderSchema)) {
		t.Fatal("migration did not produce v2 settings", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := readHostFolders(path)
	if err != nil || !reflect.DeepEqual(actual, configuration) {
		t.Fatal("v2 settings did not round trip", actual, err)
	}
}
