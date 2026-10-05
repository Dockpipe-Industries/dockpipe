package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func ListGitSessions(workdir string) ([]*GitSession, error) {
	roots, err := gitSessionRootsForWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	sessions := []*GitSession{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name(), "session.json")
			session, err := ReadGitSessionFile(path)
			if err != nil {
				continue
			}
			key := filepath.Clean(session.Storage.Metadata)
			if key == "" {
				key = session.SessionID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			sessions = append(sessions, session)
		}
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessionSortTime(sessions[i]).After(sessionSortTime(sessions[j]))
	})
	return sessions, nil
}

func LoadGitSession(workdir, selector string) (*GitSession, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("session id is empty")
	}
	sessions, err := ListGitSessions(workdir)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found under %s", workdir)
	}
	if selector == "latest" {
		return sessions[0], nil
	}
	var matches []*GitSession
	for _, session := range sessions {
		if session == nil {
			continue
		}
		switch {
		case session.SessionID == selector:
			return session, nil
		case strings.HasPrefix(session.SessionID, selector):
			matches = append(matches, session)
		case session.Repo.SessionRef == selector:
			return session, nil
		case strings.HasSuffix(session.Repo.SessionRef, "/"+selector):
			matches = append(matches, session)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var ids []string
		for _, match := range matches {
			ids = append(ids, match.SessionID)
		}
		return nil, fmt.Errorf("session selector %q is ambiguous: %s", selector, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("session %q not found", selector)
}

func ReadGitSessionFile(path string) (*GitSession, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session GitSession
	if err := json.Unmarshal(b, &session); err != nil {
		return nil, err
	}
	if strings.TrimSpace(session.Storage.Metadata) == "" {
		session.Storage.Metadata = filepath.Dir(path)
	}
	ensureGitSessionDerivedStorage(&session)
	return &session, nil
}

func GitSessionRoot(workdir, sessionID string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions", sanitizeSessionID(sessionID)), nil
}

func GitSessionsRoot(workdir string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions"), nil
}

func gitSessionRootsForWorkdir(workdir string) ([]string, error) {
	if strings.TrimSpace(workdir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workdir = wd
	}
	workdir = HostPathForGit(workdir)
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	var roots []string
	addRoot := func(root string) {
		root = filepath.Clean(root)
		for _, existing := range roots {
			if existing == root {
				return
			}
		}
		roots = append(roots, root)
	}
	if root, err := GitSessionsRoot(abs); err == nil {
		addRoot(root)
	}
	if top, err := GitTopLevel(abs); err == nil {
		if root, err := GitSessionsRoot(top); err == nil {
			addRoot(root)
		}
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "workspace" {
			sessionDir := filepath.Dir(dir)
			sessionsRoot := filepath.Dir(sessionDir)
			if filepath.Base(sessionsRoot) == "sessions" {
				addRoot(sessionsRoot)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return roots, nil
}

func sessionSortTime(session *GitSession) time.Time {
	if session == nil {
		return time.Time{}
	}
	for _, raw := range []string{session.UpdatedAt, session.CreatedAt} {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
			return t
		}
	}
	return time.Time{}
}

func writeGitSession(session *GitSession, workdir string) error {
	dir, err := gitSessionMetadataDir(session, workdir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(session.Storage.Metadata) == "" {
		session.Storage.Metadata = dir
	}
	ensureGitSessionDerivedStorage(session)
	if err := os.MkdirAll(filepath.Join(dir, "checkpoints"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "workers"), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), append(b, '\n'), 0o644)
}

func ensureGitSessionDerivedStorage(session *GitSession) {
	if session == nil {
		return
	}
	metadata := strings.TrimSpace(session.Storage.Metadata)
	if metadata == "" {
		return
	}
	session.Storage.Metadata = filepath.Clean(metadata)
	if strings.TrimSpace(session.Storage.EventLog) == "" {
		session.Storage.EventLog = gitSessionEventLogPath(session.Storage.Metadata)
	}
}

func gitSessionEventLogPath(metadataDir string) string {
	return filepath.Join(filepath.Clean(metadataDir), "events.jsonl")
}

func writeGitSyncResult(session *GitSession, res *GitSyncResult) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "last-sync.json"), append(b, '\n'), 0o644)
}

func writeGitPublishResult(session *GitSession, res *GitPublishResult) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "publish.json"), append(b, '\n'), 0o644)
}

func writeGitWorkerLease(session *GitSession, lease *GitWorkerLease) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	workerDir := filepath.Join(dir, "workers")
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workerDir, lease.WorkerID+".json"), append(b, '\n'), 0o644)
}

func writeGitCheckpoint(session *GitSession, cp *GitCheckpoint) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	cpDir := filepath.Join(dir, "checkpoints")
	if err := os.MkdirAll(cpDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cpDir, cp.CheckpointID+".json"), append(b, '\n'), 0o644)
}

func appendGitSessionEvent(session *GitSession, fields map[string]string, metadataDir string) error {
	dir := strings.TrimSpace(metadataDir)
	if dir == "" {
		var err error
		dir, err = gitSessionMetadataDir(session, metadataDir)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ev := map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)}
	for k, v := range fields {
		if strings.TrimSpace(v) != "" {
			ev[k] = v
		}
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func gitSessionMetadataDir(session *GitSession, fallbackWorkdir string) (string, error) {
	if session != nil && strings.TrimSpace(session.Storage.Metadata) != "" {
		return filepath.Clean(session.Storage.Metadata), nil
	}
	sessionID := ""
	if session != nil {
		sessionID = session.SessionID
	}
	return GitSessionRoot(fallbackWorkdir, sessionID)
}

func listWorkerLeases(session *GitSession) ([]GitWorkerLease, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return nil, err
	}
	workerDir := filepath.Join(dir, "workers")
	entries, err := os.ReadDir(workerDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	leases := make([]GitWorkerLease, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(workerDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var lease GitWorkerLease
		if err := json.Unmarshal(raw, &lease); err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}
