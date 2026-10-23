package telemetry

import (
	"context"
	"errors"
)

// finishNativeInspection admits only one live kernel process across the sample.
// Optional counters from that sample never survive an exit or PID reuse.
func finishNativeInspection(ctx context.Context, process Process, current ProcessIdentity, live bool) (Process, error) {
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	if !live || !SameProcess(process.ProcessIdentity, current) {
		return Process{}, errors.New("process exited or changed identity during inspection")
	}
	return process, nil
}
