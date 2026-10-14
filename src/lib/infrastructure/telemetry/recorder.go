package telemetry

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrBusy = errors.New("execution history busy")

// Recorder has one owner, a finite queue, and an explicit close/drain boundary.
// Submit never waits for storage. Overflow is returned to the owner, not hidden.
type Recorder struct {
	mutex   sync.Mutex
	queue   chan Execution
	done    chan struct{}
	cancel  context.CancelFunc
	closed  bool
	failure error
}

func NewRecorder(capacity int, write func(context.Context, Execution) error) (*Recorder, error) {
	if capacity < 1 || capacity > 64 {
		return nil, errors.New("execution queue capacity must be between 1 and 64")
	}
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &Recorder{queue: make(chan Execution, capacity), done: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(recorder.done)
		for {
			select {
			case <-ctx.Done():
				return
			case record, open := <-recorder.queue:
				if !open {
					return
				}
				if err := write(ctx, record); err != nil {
					recorder.mutex.Lock()
					recorder.failure = err
					recorder.mutex.Unlock()
				}
			}
		}
	}()
	return recorder, nil
}

func (recorder *Recorder) Submit(record Execution) bool {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	if recorder.closed {
		return false
	}
	select {
	case recorder.queue <- record:
		return true
	default:
		recorder.failure = errors.New("execution telemetry queue full")
		return false
	}
}

func (recorder *Recorder) Close(ctx context.Context) error {
	recorder.mutex.Lock()
	if !recorder.closed {
		recorder.closed = true
		close(recorder.queue)
	}
	recorder.mutex.Unlock()
	defer recorder.cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-recorder.done:
		recorder.mutex.Lock()
		defer recorder.mutex.Unlock()
		return recorder.failure
	}
}

// RecordContext retries only lock contention, with a 250 ms total budget. A slow
// or inaccessible history store cannot turn observation into an execution gate.
func (store Store) RecordContext(ctx context.Context, record Execution) error {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := store.Record(record)
		if !errors.Is(err, ErrBusy) {
			return err
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
