package hostdependencies

import (
	"dockpipe/src/lib/domain"
)

// Request carries workflow admission inputs without CLI parsing state.
type Request struct {
	Workflow             *domain.Workflow
	WorkflowRoot         string
	Config               string
	Workdir              string
	ApproveSystemChanges bool
}
