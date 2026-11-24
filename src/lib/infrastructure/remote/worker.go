package remote

import (
	"context"
	"errors"
	"fmt"
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
	if err := ValidateWorkerAuthority(config); err != nil {
		return err
	}
	if err := PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := LockWorker(root)
	if err != nil {
		return err
	}
	defer unlock()
	client, err := NewWorkerClient(config)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	if config.Features && client.Identity == nil {
		return errors.New("remote features require paired worker and broker machine identities")
	}
	instance := ""
	workflowOnly := !config.Features && config.Delivery != nil && client.Identity != nil
	if config.Features || workflowOnly {
		instance, err = Secret()
		if err != nil {
			return err
		}
	}
	if workflowOnly {
		discoveryContext, stopDiscovery := context.WithCancel(ctx)
		discoveryDone := make(chan struct{})
		go func() {
			defer close(discoveryDone)
			publishWorkflowOnly(discoveryContext, client, workflowOnlyTarget(config, instance))
		}()
		defer func() { stopDiscovery(); <-discoveryDone }()
	}
	if config.Features {
		featureContext, stopFeatures := context.WithCancel(ctx)
		featuresDone := make(chan struct{})
		go func() {
			defer close(featuresDone)
			if err := runFeatureWorkerWithInstance(featureContext, root, config, client, instance); err != nil && featureContext.Err() == nil {
				fmt.Fprintln(os.Stderr, "Remote features stopped; inspect the worker's feature journal and local configuration:", err)
			}
		}()
		terminalsDone := make(chan struct{})
		go func() {
			defer close(terminalsDone)
			if err := runTerminalWorker(featureContext, root, config, client, instance); err != nil && featureContext.Err() == nil {
				fmt.Fprintln(os.Stderr, "Remote terminal service stopped; inspect worker configuration and session cleanup.")
			}
		}()
		defer func() { stopFeatures(); <-featuresDone; <-terminalsDone }()
	}
	networkContext, stopNetwork := context.WithCancel(ctx)
	networkDone := make(chan struct{})
	go func() {
		defer close(networkDone)
		runNetworkService(networkContext, root, func(networkContext context.Context) error {
			return RunNetworkWorker(networkContext, root, config, client, nil)
		})
	}()
	defer func() {
		stopNetwork()
		<-networkDone
	}()
	monitorContext, stopMonitoring := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		RunMonitoring(monitorContext, root, config.Node, client)
	}()
	defer func() {
		stopMonitoring()
		<-monitorDone
	}()
	session, err := Secret()
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		var job *contract.Job
		err := waitForWorkerConnection(ctx, func() error {
			return client.callWithRetry(ctx, "/v1/next", map[string]string{"session": session}, &job)
		}, pause)
		if err != nil {
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
		err = ReadPrivate(journalPath, &journal)
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
			if job.CancelRequested {
				journal.Result = &contract.Result{Status: "cancelled", ExitCode: -1}
			} else if job.BundleHash != "" {
				if err := validateWorkerDeliveryTarget(root, config, instance, job.Submission); err != nil {
					journal.Result = &contract.Result{Status: "failure", ExitCode: -1, Log: err.Error()}
				}
				if journal.Result == nil {
					profile, err = deliveryProfile(ctx, client, root, *job, config.Delivery)
					approved = err == nil
					if err != nil {
						journal.Result = &contract.Result{Status: "failure", ExitCode: -1, Log: err.Error()}
					}
				}
			}
			if journal.Result == nil {
				if err := validateWorkerDeliveryTarget(root, config, instance, job.Submission); err != nil {
					journal.Result = &contract.Result{Status: "failure", ExitCode: -1, Log: err.Error()}
				}
			}
			if journal.Result == nil {
				if !approved {
					journal.Result = &contract.Result{Status: "failure", ExitCode: -1, Log: "Workflow profile is not approved on this node."}
				} else {
					result := executeWithHeartbeat(ctx, client, executable, profile, job.ID, job.Session)
					journal.Result = &result
				}
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

func waitForWorkerConnection(ctx context.Context, poll func() error, wait func(context.Context, time.Duration) bool) error {
	const reconnectDelay = 30 * time.Second
	for ctx.Err() == nil {
		err := poll()
		if err == nil || !retryable(err) {
			return err
		}
		if !wait(ctx, reconnectDelay) {
			break
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
			cancelRequested, err := pollWorkflowCancellation(runContext, client, id, session)
			if err != nil {
				cancel(workflowHeartbeatFailure(err))
				return
			}

			if cancelRequested {
				cancel(context.Canceled)
				return
			}
		}
	}()
	log := &boundedLog{}
	if client.Identity != nil {
		logDone := make(chan struct{})
		go func() {
			defer close(logDone)
			publishWorkflowLog(runContext, client, id, session, log)
		}()
		defer func() {
			cancel(context.Canceled)
			<-logDone
		}()
	}

	result := execute(runContext, executable, profile, log)
	cancel(context.Canceled)
	<-done
	return result
}

func pollWorkflowCancellation(ctx context.Context, client *Client, id, session string) (bool, error) {
	// Preserve the existing 20-second request ceiling for the entire heartbeat,
	// while allowing a transient failure to recover within that same window.
	const heartbeatBudget = 20 * time.Second
	const requestBudget = 8 * time.Second
	heartbeatContext, cancelHeartbeat := context.WithTimeout(ctx, heartbeatBudget)
	defer cancelHeartbeat()

	request := map[string]string{"id": id, "session": session}
	var cancelRequested bool
	err := retryCall(heartbeatContext, func() error {
		requestContext, cancelRequest := context.WithTimeout(heartbeatContext, requestBudget)
		defer cancelRequest()

		var response struct {
			Cancel *bool `json:"cancel"`
		}
		if err := client.Call(requestContext, "/v1/pulse", request, &response); err != nil {
			return err
		}
		if response.Cancel == nil {
			return errors.New("broker heartbeat omitted cancellation state")
		}

		cancelRequested = *response.Cancel
		return nil
	}, pause)

	return cancelRequested, err
}

func workflowHeartbeatFailure(err error) error {
	// The result leaves this machine. Keep endpoint URLs, transport details,
	// and broker response bodies out of the diagnostic.
	var status *HTTPError
	switch {
	case errors.As(err, &status):
		return fmt.Errorf("broker heartbeat failed; execution stopped: HTTP %d", status.StatusCode)
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("broker heartbeat failed; execution stopped: response deadline exceeded")
	case retryable(err):
		return errors.New("broker heartbeat failed; execution stopped: transient connection failure")
	default:
		return errors.New("broker heartbeat failed; execution stopped: invalid or unverified response")
	}
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
