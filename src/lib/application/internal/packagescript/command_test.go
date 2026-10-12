package packagescript

import (
	"reflect"
	"testing"
)

func TestUpsertEnvReplacesDuplicates(t *testing.T) {
	got := UpsertEnv([]string{"A=old", "B=keep", "A=duplicate"}, "A", "new")
	want := []string{"A=new", "B=keep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpsertEnv() = %#v, want %#v", got, want)
	}
}

func TestBashIsWSLPath(t *testing.T) {
	for _, path := range []string{`C:\Windows\System32\bash.exe`, `C:\Users\me\AppData\Local\Microsoft\WindowsApps\bash.exe`, `/wsl/bash`} {
		if !bashIsWSLPath(path) {
			t.Fatalf("bashIsWSLPath(%q) = false", path)
		}
	}
}
