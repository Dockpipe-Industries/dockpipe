package infrastructure

import (
	"strings"
	"testing"
)

func TestTarWorkflowMemberAdmissionIsHostIndependent(t *testing.T) {
	for _, member := range []string{"", ".", "..", "../outside.yml", "/outside.yml", "//server/file.yml", "C:/outside.yml", "C:outside.yml", `workflows\fixture\config.yml`, "workflows/../config.yml", "workflows//fixture/config.yml", "config.yml\x00extra"} {
		t.Run(member, func(t *testing.T) {
			locator := "tar:///missing/fixture.tar.gz##" + member
			if _, _, valid := SplitTarWorkflowURI(locator); valid {
				t.Fatal("unsafe archive member was admitted")
			}
			if _, err := WorkflowConfigOnDiskPath(t.TempDir(), locator); err == nil || !strings.Contains(err.Error(), "invalid tar workflow path") {
				t.Fatalf("unsafe member was not rejected before extraction: %v", err)
			}
			if _, err := loadWorkflowFromTarball("/missing/fixture.tar.gz", member); err == nil || !strings.Contains(err.Error(), "invalid contained workflow archive member") {
				t.Fatalf("direct archive loading did not reject the member before I/O: %v", err)
			}
		})
	}
	locator := "tar:///selected/archive.tar.gz##workflows/fixture/config.yml"
	archive, member, valid := SplitTarWorkflowURI(locator)
	if !valid || archive != "/selected/archive.tar.gz" || member != "workflows/fixture/config.yml" {
		t.Fatalf("canonical locator changed: %q, %q, %t", archive, member, valid)
	}
}
