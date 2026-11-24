//go:build linux || darwin

package remote

import (
	"context"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type terminalInputWaitResult struct {
	view contract.TerminalView
	err  error
}

func terminalWaitingFixture(t *testing.T) (*Broker, *Client, *Client, contract.FeatureCatalog, contract.TerminalReference) {
	t.Helper()
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	catalog.Roots[0].Terminal = true
	var response contract.TerminalExchangeResult
	exchange := contract.TerminalExchange{Schema: contract.TerminalVersion, Catalog: catalog}
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	exchange.Epoch = response.Epoch
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, nil); err != nil {
		t.Fatal(err)
	}
	catalog.TerminalEpoch = response.Epoch
	intent := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	reference := intent.Reference()
	exchange.Reports = []contract.TerminalReport{{Reference: reference, State: "running"}}
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, nil); err != nil {
		t.Fatal(err)
	}
	return broker, admin, worker, exchange.Catalog, reference
}

func waitForTerminalInputWaiter(t *testing.T, broker *Broker, reference contract.TerminalReference) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		broker.mutex.Lock()
		record := broker.terminals.records[reference.ID]
		waiting := record.inputWaiting && record.input != nil
		broker.mutex.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("input did not reach its bounded native ACK waiter")
}

func TestTerminalInputWaitAllowsWorkerProgressAndBoundsDuplicateWaiters(t *testing.T) {
	broker, admin, worker, catalog, reference := terminalWaitingFixture(t)
	input := contract.TerminalInput{Reference: reference, Sequence: 1, Data: []byte("private-input-marker")}
	finished := make(chan terminalInputWaitResult, 1)
	go func() {
		view, err := admin.InputTerminal(context.Background(), input)
		finished <- terminalInputWaitResult{view: view, err: err}
	}()
	waitForTerminalInputWaiter(t, broker, reference)
	probe, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := admin.FeatureCatalog(probe); err != nil {
		t.Fatal("native ACK wait held the global broker mutex", err)
	}
	duplicate, err := admin.InputTerminal(probe, input)
	if err != nil || duplicate.AckSequence != 0 {
		t.Fatal("same input retry created another waiter or optimistic ACK", err)
	}
	select {
	case <-finished:
		t.Fatal("input was acknowledged before the worker's native write report")
	default:
	}
	exchange := contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: reference.Epoch, Catalog: catalog,
		Reports: []contract.TerminalReport{{Reference: reference, State: "running", AckSequence: 1, AckDigest: contract.TerminalInputDigest(input.Data)}}}
	if err := worker.Call(probe, "/v1/terminal-worker", exchange, nil); err != nil {
		t.Fatal("worker could not publish its native acknowledgment during wait", err)
	}
	select {
	case result := <-finished:
		if result.err != nil || result.view.AckSequence != 1 || result.view.Reference != reference || len(result.view.Data) != 0 {
			t.Fatal("input did not return exactly the native ACK metadata", result.err)
		}
	case <-probe.Done():
		t.Fatal("native ACK notification did not wake its original input call")
	}
	broker.mutex.Lock()
	record := broker.terminals.records[reference.ID]
	retained := record.inputWaiting || record.input != nil || record.ackSequence != 1
	broker.mutex.Unlock()
	if retained {
		t.Fatal("completed input retained a waiter/input slot or lost its exact sequence")
	}
}

func TestTerminalInputWaitRechecksAuthorityAndReferenceWithoutReplaying(t *testing.T) {
	for _, scenario := range []string{"lock", "reference", "owner", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			broker, admin, _, _, reference := terminalWaitingFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan terminalInputWaitResult, 1)
			go func() {
				view, err := admin.InputTerminal(ctx, contract.TerminalInput{Reference: reference, Sequence: 1, Data: []byte("private-input-marker")})
				finished <- terminalInputWaitResult{view: view, err: err}
			}()
			waitForTerminalInputWaiter(t, broker, reference)
			if scenario == "lock" {
				broker.security.Lock()
			}
			broker.mutex.Lock()
			record := broker.terminals.records[reference.ID]
			switch scenario {
			case "reference":
				record.open.Target.Generation = Hash("changed-generation")
			case "owner":
				record.admission.generation++
			case "cancel":
				cancel()
			}
			record.notify()
			broker.mutex.Unlock()
			select {
			case result := <-finished:
				if result.err == nil || result.view.AckSequence != 0 || len(result.view.Data) != 0 {
					t.Fatal("changed authority/identity or cancellation became a confirmed result")
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled or invalid native ACK wait did not terminate")
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				broker.mutex.Lock()
				waiting := record.inputWaiting
				broker.mutex.Unlock()
				if !waiting {
					break
				}
				time.Sleep(time.Millisecond)
			}
			broker.mutex.Lock()
			unconfirmed := !record.inputWaiting && record.ackSequence == 0 && record.input != nil && record.pendingSequence == 1
			broker.mutex.Unlock()
			if !unconfirmed {
				t.Fatal("lost acknowledgment erased or replayed the original pending input")
			}
		})
	}
}

func TestTerminalInputWaitReturnsPendingAtItsBound(t *testing.T) {
	broker, admin, _, _, reference := terminalWaitingFixture(t)
	started := time.Now()
	view, err := admin.InputTerminal(context.Background(), contract.TerminalInput{Reference: reference, Sequence: 1, Data: []byte("private-input-marker")})
	if err != nil || view.AckSequence != 0 || time.Since(started) > 2*time.Second {
		t.Fatal("bounded pending input was relabelled successful or waited indefinitely", err)
	}
	broker.mutex.Lock()
	record := broker.terminals.records[reference.ID]
	invalid := record.inputWaiting || record.ackSequence != 0 || record.input == nil
	broker.mutex.Unlock()
	if invalid {
		t.Fatal("timeout failed to release only its waiter while preserving unknown input")
	}
}
