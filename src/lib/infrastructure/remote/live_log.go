package remote

import (
	"bytes"
	"context"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func (b *Broker) publishLiveLog(node string, raw []byte) error {
	var update contract.LiveLogUpdate
	if err := Decode(raw, &update); err != nil || len(update.Log.Data) == 0 || len(update.Log.Data) > contract.MaxLiveLog ||
		(update.Log.Truncated && len(update.Log.Data) != contract.MaxLiveLog) {
		return errors.New("invalid workflow log preview")
	}

	job, exists := b.state.Jobs[update.ID]
	if !exists || job.Node != node || job.Status != "running" || update.Session == "" || job.Session != update.Session {
		return errors.New("workflow log preview requires the running assignment")
	}

	previous := b.liveLogs[job.ID]
	if !bytes.HasPrefix(update.Log.Data, previous.Data) || (previous.Truncated && !update.Log.Truncated) {
		return errors.New("workflow log preview changed its published prefix")
	}

	if b.liveLogs == nil {
		b.liveLogs = make(map[string]contract.LiveLog)
	}
	b.liveLogs[job.ID] = update.Log
	return nil
}

func (b *Broker) readLiveLog(raw []byte) (contract.LiveLogView, error) {
	var request struct {
		ID string `json:"id"`
	}
	if err := Decode(raw, &request); err != nil || !contract.ValidID(request.ID) {
		return contract.LiveLogView{}, errors.New("valid workflow job identity required")
	}

	job, _, err := b.lookupJob(request.ID)
	if err != nil {
		return contract.LiveLogView{}, err
	}

	view := contract.LiveLogView{ID: job.ID, Status: job.Status}
	enrollment, exists := b.state.Nodes[job.Node]
	if job.Status == "running" && exists && b.enrollmentActive(enrollment, time.Now()) {
		view.Log = b.liveLogs[job.ID]
	}
	return view, nil
}

func publishWorkflowLog(ctx context.Context, client *Client, id, session string, log *boundedLog) {
	var publishedSize int
	var publishedTruncated bool
	for pause(ctx, time.Second) {
		snapshot, changed := log.preview(publishedSize, publishedTruncated)
		if !changed {
			continue
		}

		request, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := client.Call(request, "/v1/job-log-publish", contract.LiveLogUpdate{ID: id, Session: session, Log: snapshot}, nil)
		cancel()
		if err != nil {
			// Log preview is optional on older brokers. Cancellation and result
			// delivery retain their independent paths and retry rules.
			var response *HTTPError
			if errors.As(err, &response) && response.StatusCode >= 400 && response.StatusCode < 500 {
				return
			}
			continue
		}

		publishedSize, publishedTruncated = len(snapshot.Data), snapshot.Truncated
		if publishedTruncated {
			return
		}
	}
}
