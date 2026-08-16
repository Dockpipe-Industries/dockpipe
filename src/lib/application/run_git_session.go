package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func createWorkflowGitSession(wf *domain.Workflow, sourceDir, wfRoot string, opts *CliOpts) (*infrastructure.GitSession, error) {
	if wf == nil || wf.Workspace.IsEmpty() {
		return nil, nil
	}
	cfg := wf.Workspace
	sessionSourceDir, workspaceID := resolveWorkflowWorkspaceSource(cfg.Repo, sourceDir, wfRoot, wf.Name)
	sessionID := strings.TrimSpace(os.Getenv("DOCKPIPE_SESSION_ID"))
	if sessionID == "" {
		name := strings.TrimSpace(wf.Name)
		if name == "" && opts != nil {
			name = strings.TrimSpace(opts.Workflow)
		}
		if name == "" {
			name = "workflow"
		}
		sessionID = timeNowSessionSlug(name)
	}
	return infrastructure.CreateSessionBranch(infrastructure.GitSessionRequest{
		WorkspaceID:  workspaceID,
		SourceDir:    sessionSourceDir,
		Mode:         cfg.Mode,
		Storage:      cfg.Storage,
		BaseRef:      cfg.Base,
		BranchPrefix: cfg.Lifecycle.BranchPrefix,
		BranchName:   cfg.Lifecycle.Branch,
		SessionID:    sessionID,
		Checkpoint:   cfg.Lifecycle.Checkpoint,
		Publish:      cfg.Lifecycle.Publish,
	})
}

func resolveWorkflowWorkspaceSource(repo, sourceDir, wfRoot, wfName string) (string, string) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return sourceDir, firstNonEmpty(wfName, "workspace")
	}
	for _, base := range []string{sourceDir, wfRoot} {
		if base == "" {
			continue
		}
		candidate := repo
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(base, repo)
		}
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, filepath.Base(filepath.Clean(candidate))
		}
	}
	if filepath.IsAbs(repo) {
		if st, err := os.Stat(repo); err == nil && st.IsDir() {
			return repo, filepath.Base(filepath.Clean(repo))
		}
	}
	return sourceDir, firstNonEmpty(repo, wfName, "workspace")
}

func finalizeWorkflowGitSession(session *infrastructure.GitSession, wf *domain.Workflow) error {
	if err := checkpointWorkflowGitSession(session, wf); err != nil {
		return err
	}
	return autoCleanupWorkflowGitSessionVolume(session, wf)
}

func checkpointWorkflowGitSession(session *infrastructure.GitSession, wf *domain.Workflow) error {
	if session == nil || wf == nil {
		return nil
	}
	mode := strings.TrimSpace(wf.Workspace.Lifecycle.Checkpoint)
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "auto", "step":
		ids := map[string]string{
			"session": session.SessionID,
		}
		var cp *infrastructure.GitCheckpoint
		_, err := infrastructure.RunOperationWithResult(os.Stderr, "session.checkpoint", "Checkpointing session workspace…", ids, func() error {
			var opErr error
			cp, opErr = infrastructure.CheckpointSession(session, "workflow completed")
			if cp != nil {
				ids["checkpoint"] = cp.CheckpointID
				if strings.TrimSpace(cp.Commit) != "" {
					ids["commit"] = cp.Commit
				}
				ids["result"] = cp.Status
			}
			return opErr
		})
		if err != nil {
			return err
		}
		return nil
	case "manual":
		return nil
	default:
		return fmt.Errorf("workspace.lifecycle.checkpoint must be manual, auto, or step")
	}
}

func autoCleanupWorkflowGitSessionVolume(session *infrastructure.GitSession, wf *domain.Workflow) error {
	if session == nil || wf == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(session.Storage.Backend), "docker_volume") {
		return nil
	}
	policy := strings.TrimSpace(os.Getenv("DOCKPIPE_SESSION_VOLUME_AUTOCLEANUP"))
	if policy == "" {
		policy = "true"
	}
	switch strings.ToLower(policy) {
	case "1", "true", "yes", "on":
		return infrastructure.CleanupSessionVolume(session)
	case "0", "false", "no", "off":
		return nil
	default:
		return fmt.Errorf("DOCKPIPE_SESSION_VOLUME_AUTOCLEANUP must be true or false")
	}
}

func timeNowSessionSlug(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "workflow"
	}
	return time.Now().UTC().Format("20060102-150405") + "-" + slug
}
