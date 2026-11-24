package remote

import (
	"context"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// terminalInputAndWait enters with broker.mutex held and returns with it held.
// Admission/submission happens exactly once. Waiting never reapplies input and
// never blocks worker reports, other terminals, or authority revocation.
func (broker *Broker) terminalInputAndWait(ctx context.Context, raw []byte, admission operatorAdmission) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, err := broker.terminalOperator("/v1/terminal-input", raw, admission)
	if err != nil {
		return nil, err
	}
	view, ok := value.(contract.TerminalView)
	if !ok {
		return nil, errors.New("invalid terminal input response")
	}
	var input contract.TerminalInput
	if decodeFeature(raw, &input) != nil {
		return nil, errors.New("invalid terminal input intent")
	}
	// The pending record already owns the input. Retain only routing/sequence
	// metadata while waiting; this second decode must not keep another plaintext copy.
	clear(input.Data)
	input.Data = nil
	record := broker.terminalState().records[input.Reference.ID]
	if record == nil || record.inputWaiting || view.State != "running" || view.AckSequence >= input.Sequence {
		return view, nil
	}
	record.inputWaiting = true
	defer func() { record.inputWaiting = false }()
	deadline := time.Now().Add(time.Second)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		if record.changed == nil {
			record.changed = make(chan struct{})
		}
		changed := record.changed
		broker.mutex.Unlock()
		finished := false
		select {
		case <-ctx.Done():
		case <-timer.C:
			finished = true
		case <-changed:
		}
		broker.mutex.Lock()
		finished = finished || !time.Now().Before(deadline)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if broker.poisoned || broker.security == nil || broker.security.CheckAdmission(admission) != nil {
			return nil, errors.New("terminal input authority expired while awaiting native acknowledgement")
		}
		state := broker.terminalState()
		if state.epoch != input.Reference.Epoch || state.records[input.Reference.ID] != record || record.open.Reference() != input.Reference || !terminalOwner(record, admission) {
			return nil, errors.New("terminal input identity changed while awaiting native acknowledgement")
		}
		view, err = record.view(nil)
		if err != nil {
			return nil, err
		}
		if finished || view.AckSequence >= input.Sequence || view.State != "running" || record.close || record.released {
			return view, nil
		}
	}
}
