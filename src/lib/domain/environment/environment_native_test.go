package environment

import (
	"strings"
	"testing"
)

func TestWindowsInheritedEnvironmentCanonicalizesBeforePathInjection(t *testing.T) {
	environment := environToMap([]string{
		`Path=C:\fixture\old`,
		`PATH=C:\fixture\go\bin;C:\Windows\System32`,
		"PathExt=.COM;.EXE;.BAT;.CMD",
		"Selected=old", "SELECTED=", "PrivateValue= preserve exact whitespace ",
	}, "windows")
	if environment["PATH"] != `C:\fixture\go\bin;C:\Windows\System32` || environment["SELECTED"] != "" || environment["PRIVATEVALUE"] != " preserve exact whitespace " {
		t.Fatal("native environment lost ordered overrides or changed values")
	}
	environment["PATH"] = `C:\fixture\dockpipe\bin;` + environment["PATH"]
	for sample := 0; sample < 100; sample++ {
		paths := 0
		for _, entry := range EnvMapToSlice(environment) {
			key, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "PATH") {
				paths++
				if !strings.Contains(value, `fixture\go\bin`) {
					t.Fatal("injected PATH discarded inherited tools")
				}
			}
		}
		if paths != 1 {
			t.Fatal("case-insensitive duplicate PATH reached process boundary")
		}
	}
}

func TestUnixInheritedEnvironmentPreservesCaseAndLastDuplicate(t *testing.T) {
	environment := environToMap([]string{"Path=lower", "PATH=first", "PATH=last", "EMPTY="}, "linux")
	if len(environment) != 3 || environment["Path"] != "lower" || environment["PATH"] != "last" || environment["EMPTY"] != "" {
		t.Fatal("Unix case-sensitive environment semantics changed")
	}
}
