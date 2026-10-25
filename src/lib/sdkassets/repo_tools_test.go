package sdkassets

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonSDKExplicitBinaryAndDiscoveryPrecedence(t *testing.T) {
	driver := `import importlib.util
import os
from pathlib import Path
import sys
from unittest import mock

helper, root = Path(sys.argv[1]), Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("portable_sdk", helper)
sdk = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sdk)
project = root / "project"
local = project / "src/bin/dockpipe"
local.parent.mkdir(parents=True)
local.write_text("fixture")
local.chmod(0o700)
path_bin = root / "path-bin"
path_bin.mkdir()
path_cli = path_bin / ("dockpipe.exe" if os.name == "nt" else "dockpipe")
path_cli.write_text("fixture")
path_cli.chmod(0o700)
override_path = root / "explicit/dockpipe"
override_path.parent.mkdir()
override_path.write_text("fixture")
override_path.chmod(0o700)
override = str(override_path)
with mock.patch.dict(os.environ, {"DOCKPIPE_BIN": override, "PATH": str(path_bin), "PATHEXT": ".EXE"}, clear=True):
    selected = sdk.DockpipeSDK(str(project))
    assert selected.dockpipe_bin == override
    with mock.patch.object(sdk.subprocess, "check_output", return_value='{"scope":"selected"}') as run:
        assert selected.scope() == {"scope": "selected"}
        run.assert_called_once_with([override, "scope", "--workdir", str(project.resolve())], text=True)
    os.environ.pop("DOCKPIPE_BIN")
    selected.refresh(str(project))
    assert selected.dockpipe_bin == str(local.resolve())
    local.unlink()
    selected.refresh(str(project))
    assert selected.dockpipe_bin is not None, "PATH discovery did not select its fixture"
    assert Path(selected.dockpipe_bin).samefile(path_cli), (selected.dockpipe_bin, str(path_cli))
    path_cli.unlink()
    windows_local = project / "src/bin/dockpipe.exe"
    windows_local.write_text("fixture")
    windows_local.chmod(0o700)
    with mock.patch.object(sdk.sys, "platform", "win32"):
        assert sdk.resolve_dockpipe_bin(str(project)) == str(windows_local.resolve())
    windows_local.unlink()
    selected.refresh(str(project))
    assert selected.dockpipe_bin is None
    try:
        selected.scope()
    except RuntimeError as error:
        assert "set DOCKPIPE_BIN" in str(error)
    else:
        raise AssertionError("Missing binary selection did not fail")
`
	runPythonSDKDriver(t, driver)
}

func runPythonSDKDriver(t *testing.T, driver string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		python, err = exec.LookPath("python")
	}
	if err != nil {
		t.Fatal("Python 3 is required for the portable SDK contract")
	}
	root := t.TempDir()
	content, err := Scripts.ReadFile("scripts/repo_tools.py")
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, "repo_tools.py")
	if err := os.WriteFile(helper, content, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(python, "-B", "-", helper, root)
	command.Stdin = strings.NewReader(driver)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("portable Python SDK contract failed: %v\n%s", err, output)
	}
}

func TestPythonSDKPromptsRequireMatchingResourceKind(t *testing.T) {
	driver := `import contextlib
import importlib.util
import io
import os
from pathlib import Path
import sys
from unittest import mock

helper, root = Path(sys.argv[1]), Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("portable_sdk", helper)
sdk = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sdk)
file = root / "file.txt"
file.write_text("fixture")
directory = root / "directory"
directory.mkdir()
with mock.patch.dict(os.environ, {"DOCKPIPE_WORKDIR": str(root), "DOCKPIPE_SDK_PROMPT_MODE": "terminal"}):
    assert sdk._resource_exists(str(file), "file")
    assert not sdk._resource_exists(str(directory), "file"), "directory accepted as file"
    assert not sdk._resource_exists(str(file), "directory"), "file accepted as directory"
    assert sdk._resource_exists(str(directory), "directory")
    for kind in ("file", "resource"):
        for expected_kind, invalid, valid in (("file", directory, file), ("directory", file, directory)):
            arguments = {"must_exist": True}
            if kind == "file":
                arguments["path_mode"] = "open-dir" if expected_kind == "directory" else "open-file"
            else:
                arguments["resource_kind"] = expected_kind
            diagnostics = io.StringIO()
            with mock.patch("builtins.input", side_effect=[str(invalid), str(valid)]) as read:
                with contextlib.redirect_stderr(diagnostics):
                    assert sdk.prompt(kind, **arguments) == str(valid)
                assert read.call_count == 2, "invalid path kind was accepted"
            assert "not found" in diagnostics.getvalue()
`
	runPythonSDKDriver(t, driver)
}
