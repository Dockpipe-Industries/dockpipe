package domain

import "testing"

func TestContainerMountModeConstantsMatchAuthoredVocabulary(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"read only", string(ContainerMountReadOnly), "ro"},
		{"read write", string(ContainerMountReadWrite), "rw"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q want %q", test.got, test.want)
			}
		})
	}
}

func TestContainerMountModeWireFieldRemainsString(t *testing.T) {
	var mode string = (WorkflowContainerMount{}).Mode
	_ = mode
}

func TestContainerMountModeValidationAcceptsTrimmedWireValueWithoutMutation(t *testing.T) {
	cfg := WorkflowContainerConfig{
		Mounts: []WorkflowContainerMount{{Host: "../repo", Guest: "/work", Mode: " ro "}},
	}
	if err := ValidateWorkflowContainerConfig("container", cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Mounts[0].Mode != " ro " {
		t.Fatalf("validation mutated authored mount mode to %q", cfg.Mounts[0].Mode)
	}
}

func TestWorkspaceValueConstantsMatchRuntimeVocabulary(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"managed mode", string(WorkspaceModeManaged), "managed"},
		{"bind mode", string(WorkspaceModeBind), "bind"},
		{"bind storage", string(WorkspaceStorageBind), "bind"},
		{"volume storage", string(WorkspaceStorageVolume), "volume"},
		{"worktree storage", string(WorkspaceStorageWorktree), "worktree"},
		{"clone storage", string(WorkspaceStorageClone), "clone"},
		{"bind backend", string(WorkspaceStorageBackendBind), "bind"},
		{"worktree backend", string(WorkspaceStorageBackendWorktree), "worktree"},
		{"docker volume backend", string(WorkspaceStorageBackendDockerVolume), "docker_volume"},
		{"manual checkpoint", string(WorkspaceCheckpointManual), "manual"},
		{"automatic checkpoint", string(WorkspaceCheckpointAuto), "auto"},
		{"step checkpoint", string(WorkspaceCheckpointStep), "step"},
		{"no publication", string(WorkspacePublishNone), "none"},
		{"branch publication", string(WorkspacePublishBranch), "branch"},
		{"review publication", string(WorkspacePublishReview), "review"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q want %q", test.got, test.want)
			}
		})
	}
}

func TestWorkspaceWireFieldsRemainStrings(t *testing.T) {
	var mode string = (WorkflowWorkspaceConfig{}).Mode
	var storage string = (WorkflowWorkspaceConfig{}).Storage
	var checkpoint string = (WorkflowWorkspaceLifecycleConfig{}).Checkpoint
	var publish string = (WorkflowWorkspaceLifecycleConfig{}).Publish
	_ = []string{mode, storage, checkpoint, publish}
}

func TestWorkspaceValidationAcceptsTrimmedWireValuesWithoutMutation(t *testing.T) {
	cfg := WorkflowWorkspaceConfig{
		Mode:    " managed ",
		Storage: " worktree ",
		Lifecycle: WorkflowWorkspaceLifecycleConfig{
			Checkpoint: " auto ",
			Publish:    " review ",
		},
	}
	if err := ValidateWorkflowWorkspaceConfig("workspace", cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != " managed " || cfg.Storage != " worktree " || cfg.Lifecycle.Checkpoint != " auto " || cfg.Lifecycle.Publish != " review " {
		t.Fatalf("validation mutated authored workspace values: %+v", cfg)
	}
}

func TestNormalizeWorkspaceStorageBackendPreservesCaseInsensitiveCompatibility(t *testing.T) {
	if got := NormalizeWorkspaceStorageBackend(" Docker_Volume "); got != WorkspaceStorageBackendDockerVolume {
		t.Fatalf("normalized backend = %q", got)
	}
}
