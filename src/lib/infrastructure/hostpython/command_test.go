package hostpython

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestPlatformInterpreterFallback(t *testing.T) {
	for _, operatingSystem := range []string{"windows", "linux", "darwin"} {
		t.Run(operatingSystem, func(t *testing.T) {
			var requested []string
			command, err := commandForPlatform("literal script.py", nil, operatingSystem, func(name string) (string, error) {
				requested = append(requested, name)
				if name == "python" {
					return "/fixture/python", nil
				}
				return "", exec.ErrNotFound
			})
			if operatingSystem == "windows" {
				if err != nil || !reflect.DeepEqual(requested, []string{"python3", "python"}) || command.Args[1] != "literal script.py" {
					t.Fatalf("Windows fallback = %v, %v", requested, err)
				}
			} else if err == nil || !reflect.DeepEqual(requested, []string{"python3"}) {
				t.Fatalf("Unix selection = %v, %v", requested, err)
			}
		})
	}
}

func TestPresentEmptyOverrideDoesNotSearchPath(t *testing.T) {
	_, err := commandForPlatform("script.py", []string{"DOCKPIPE_HOST_PYTHON_BIN="}, "linux", func(name string) (string, error) {
		t.Fatalf("invalid override searched PATH: %s", name)
		return "", exec.ErrNotFound
	})
	if err == nil {
		t.Fatal("empty override accepted")
	}
}
