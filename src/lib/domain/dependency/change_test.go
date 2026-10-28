package dependency

import "testing"

func TestChangeApprovalBindsEveryInputAndOperation(t *testing.T) {
	request := ChangeRequest{Schema: ChangeSchema, Resolver: "example.tool", ProviderVersion: "1.2.3", Machine: "local", Parameters: map[string]string{"target": "/private/tool", "sha256": "digest"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	approval := request.ApprovalID("apply")
	if approval == request.ApprovalID("recover") {
		t.Fatal("recovery reused apply authority")
	}
	request.Parameters["target"] = "/different/tool"
	if approval == request.ApprovalID("apply") {
		t.Fatal("target change preserved approval")
	}
	request.Machine = "paired-host"
	if request.Validate() == nil {
		t.Fatal("remote mutation accepted without broker authorization")
	}
}
