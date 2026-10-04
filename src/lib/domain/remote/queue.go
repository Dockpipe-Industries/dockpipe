package remote

import (
	"errors"
	"time"
)

const MaxJobs = 128

// Queue evaluates transitions against retained job identities. It never mutates
// its map: the broker publishes accepted changes through its durable boundary.
// A true changed result means the returned job must be persisted before reply.
type Queue map[string]Job

func (q Queue) Submit(request Submission, nodeEnrolled bool, now time.Time) (job Job, changed bool, err error) {
	if err := request.Validate(); err != nil {
		return Job{}, false, err
	}
	if previous, exists := q[request.ID]; exists {
		if previous.Submission != request {
			return Job{}, false, errors.New("job ID already binds different work")
		}
		return previous, false, nil
	}
	if !nodeEnrolled || len(q) >= MaxJobs {
		return Job{}, false, errors.New("node is not enrolled or job capacity reached")
	}
	return Job{Submission: request, Status: "queued", CreatedAt: now.UTC()}, true, nil
}

func (q Queue) Next(node, session string) (job *Job, changed bool, err error) {
	if len(session) != 64 || !ValidID(session) {
		return nil, false, errors.New("worker session identity required")
	}
	// Unknown work requires journal reconciliation. A disconnect or a different
	// worker session is never permission to assign another job on the same node.
	var selected *Job
	for _, candidate := range q {
		if candidate.Node != node {
			continue
		}
		if candidate.Status == "running" || candidate.Status == "unknown" {
			return &candidate, false, nil
		}
		if candidate.Status == "queued" && (selected == nil || candidate.CreatedAt.Before(selected.CreatedAt)) {
			selected = &candidate
		}
	}
	if selected == nil {
		return nil, false, nil
	}
	selected.Status = "running"
	selected.Session = session
	return selected, true, nil
}

func (q Queue) Assigned(id, node, session string) (Job, error) {
	job, exists := q[id]
	if !exists || job.Node != node {
		return Job{}, errors.New("job does not belong to node")
	}
	if session != job.Session || session == "" {
		return Job{}, errors.New("job belongs to a different worker session")
	}
	return job, nil
}
