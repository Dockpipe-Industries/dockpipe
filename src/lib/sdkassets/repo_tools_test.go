package sdkassets

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonSDKExplicitBinaryAndDiscoveryPrecedence(t *testing.T) {
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
    selected.refresh(str(project))
    assert selected.dockpipe_bin is None
    try:
        selected.scope()
    except RuntimeError as error:
        assert "set DOCKPIPE_BIN" in str(error)
    else:
        raise AssertionError("Missing binary selection did not fail")
`
	command := exec.Command(python, "-B", "-", helper, root)
	command.Stdin = strings.NewReader(driver)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("portable Python SDK contract failed: %v\n%s", err, output)
	}
}
