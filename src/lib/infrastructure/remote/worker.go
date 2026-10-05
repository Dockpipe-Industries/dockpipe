package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type WorkerJournal struct {
	Job     contract.Submission `json:"job"`
	Session string              `json:"session"`
	Result  *contract.Result    `json:"result,omitempty"`
}

func RunWorker(ctx context.Context, root, executable string, config contract.WorkerConfig) error {
	if config.Schema != contract.Version || !contract.ValidID(config.Node) || len(config.Token) != 64 {
		return errors.New("invalid worker identity")
	}
	if err := ValidateProfiles(config.Profiles); err != nil {
		return err
	}
	if err := PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := Lock(filepath.Join(root, "worker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	client, err := NewClient(config.Endpoint, config.Token)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	session, err := Secret()
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		var job *contract.Job
		if err := client.callWithRetry(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil {
			return err
		}
		if job == nil {
			if !pause(ctx, 2*time.Second) {
				break
			}
			continue
		}
		if job.Node != config.Node || job.Validate() != nil {
			return errors.New("broker returned an invalid assignment")
		}
		if job.Status == "unknown" && job.Result != nil {
			return errors.New("an interrupted job needs operator inspection before this node can accept more work")
		}
		journalPath := filepath.Join(root, "jobs", job.ID+".json")
		journal := WorkerJournal{Job: job.Submission, Session: job.Session}
		err := ReadPrivate(journalPath, &journal)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if journal.Job != job.Submission {
			return errors.New("job ID changed its local assignment")
		}
		if journal.Session != job.Session {
			return errors.New("job changed its worker session")
		}
		if os.IsNotExist(err) && job.Session != session {
			return errors.New("another worker session owns this node's assignment; recover that worker's journal before continuing")
		}
		if err == nil && journal.Result == nil || os.IsNotExist(err) && job.Status == "unknown" {
			// The previous process may have run work. Do not guess or execute again.
			journal.Result = &contract.Result{Status: "unknown", ExitCode: -1, Log: "Worker execution was interrupted; inspect the checkout before submitting a new job ID."}
		}
		if journal.Result == nil {
			if err := WritePrivate(journalPath, journal); err != nil {
				return err
			}
			profile, approved := config.Profiles[job.Profile]
			if !approved {
				journal.Result = &contract.Result{Status: "failure", ExitCode: -1, Log: "Workflow profile is not approved on this node."}
			} else if job.CancelRequested {
				journal.Result = &contract.Result{Status: "cancelled", ExitCode: -1}
			} else {
				result := executeWithHeartbeat(ctx, client, executable, profile, job.ID, job.Session)
				journal.Result = &result
			}
			if err := WritePrivate(journalPath, journal); err != nil {
				return err
			}
		}
		// Persist the result before delivery. A network retry never reruns work.
		if err := WritePrivate(journalPath, journal); err != nil {
			return err
		}
		request := map[string]any{"id": job.ID, "session": journal.Session, "result": journal.Result}
		if err := client.callWithRetry(ctx, "/v1/result", request, nil); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func executeWithHeartbeat(ctx context.Context, client *Client, executable string, profile contract.Profile, id, session string) contract.Result {
	runContext, cancel := context.WithCancelCause(ctx)
	defer cancel(context.Canceled)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for pause(runContext, 2*time.Second) {
			var response struct {
				Cancel bool `json:"cancel"`
			}
			if err := client.Call(runContext, "/v1/pulse", map[string]string{"id": id, "session": session}, &response); err != nil {
				cancel(errors.New("broker heartbeat failed; execution stopped"))
				return
			}
			if response.Cancel {
				cancel(context.Canceled)
				return
			}
		}
	}()
	result := Execute(runContext, executable, profile)
	cancel(context.Canceled)
	<-done
	return result
}

func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
