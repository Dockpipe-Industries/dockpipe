package remote

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type terminalBroker struct {
	epoch    string
	records  map[string]*terminalRecord
	catalogs map[string]contract.FeatureCatalog
}

type terminalRecord struct {
	open            contract.TerminalOpen
	admission       operatorAdmission
	state           string
	reason          string
	close           bool
	released        bool
	input           *contract.TerminalInput
	pendingSequence uint64
	pendingDigest   string
	ackSequence     uint64
	ackDigest       string
	columns         int
	rows            int
	output          *terminalBuffer
	finalCursor     uint64
}

func (record *terminalRecord) release() {
	if record.input != nil {
		clear(record.input.Data)
	}
	if record.output != nil {
		record.finalCursor = record.output.end
		clear(record.output.data[:])
	}
	record.input, record.output, record.released = nil, nil, true
}

func (broker *Broker) terminalState() *terminalBroker {
	if broker.terminals == nil {
		// Fresh random epoch is intentionally not persisted. Old open intents
		// cannot recreate shells or replay input after a broker restart.
		epoch := Hash(rand.Text())
		broker.terminals = &terminalBroker{epoch: epoch, records: map[string]*terminalRecord{}, catalogs: map[string]contract.FeatureCatalog{}}
	}
	return broker.terminals
}

func terminalPath(path string) bool {
	return strings.HasPrefix(path, "/v1/terminal-")
}

func (broker *Broker) publishTerminalCatalog(catalog contract.FeatureCatalog) contract.FeatureCatalog {
	state := broker.terminalState()
	terminal, connected := state.catalogs[catalog.Target.Node]
	available := connected && terminal.Target == catalog.Target && broker.featureCatalogCurrent(terminal)
	catalog.Roots = append([]contract.FeatureRoot(nil), catalog.Roots...)
	for index := range catalog.Roots {
		catalog.Roots[index].Terminal = catalog.Roots[index].Terminal && available
	}
	if available {
		catalog.TerminalEpoch = state.epoch
	}
	return catalog
}

func terminalRoot(catalog contract.FeatureCatalog, root string) bool {
	for _, grant := range catalog.Roots {
		if grant.ID == root {
			return grant.Terminal
		}
	}
	return false
}

func (record *terminalRecord) view(cursor *uint64) (contract.TerminalView, error) {
	view := contract.TerminalView{End: record.finalCursor}
	if record.output != nil {
		view.End = record.output.end
	}
	if cursor != nil && record.output != nil {
		var err error
		view, err = record.output.read(*cursor)
		if err != nil {
			return view, err
		}
	}
	if cursor != nil && record.output == nil && record.released {
		view.Cursor, view.Oldest, view.Truncated = record.finalCursor, record.finalCursor, *cursor < record.finalCursor
	}
	view.Reference, view.State, view.AckSequence, view.Reason = record.open.Reference(), record.state, record.ackSequence, record.reason
	return view, nil
}

func (broker *Broker) terminalOperator(path string, raw []byte, admission operatorAdmission) (any, error) {
	state := broker.terminalState()
	now := time.Now()
	for id, record := range state.records {
		if record.released && now.After(record.open.Deadline) {
			delete(state.records, id)
		}
	}
	if path == "/v1/terminal-open" {
		var request contract.TerminalOpen
		if decodeFeature(raw, &request) != nil || request.Validate() != nil || request.Epoch != state.epoch {
			return nil, errors.New("invalid terminal open intent or broker epoch")
		}
		if previous := state.records[request.ID]; previous != nil {
			if !terminalOwner(previous, admission) {
				return nil, errors.New("terminal belongs to another OS session")
			}
			return previous.view(nil)
		}
		catalog, exists := state.catalogs[request.Target.Node]
		if !now.Before(request.Deadline) || request.Deadline.Sub(now) > contract.FeatureLifetime || !exists ||
			catalog.Target != request.Target || !broker.featureCatalogCurrent(catalog) || !terminalRoot(catalog, request.Root) {
			return nil, errors.New("terminal target, grant, or open deadline is unavailable")
		}
		retained, onNode := 0, 0
		for _, record := range state.records {
			if record.released {
				continue
			}
			retained++
			if record.open.Target.Node == request.Target.Node {
				onNode++
			}
		}
		if retained >= contract.MaxTerminalBrokerSessions || onNode >= contract.MaxTerminalSessions || len(state.records) >= 512 {
			return nil, errors.New("terminal session capacity reached")
		}
		record := &terminalRecord{open: request, admission: admission, state: "queued", columns: request.Columns, rows: request.Rows, output: &terminalBuffer{}}
		state.records[request.ID] = record
		return record.view(nil)
	}
	var reference contract.TerminalReference
	var input contract.TerminalInput
	var resize contract.TerminalResize
	var read contract.TerminalRead
	var err error
	switch path {
	case "/v1/terminal-input":
		err = decodeFeature(raw, &input)
		reference = input.Reference
		if err == nil {
			err = input.Validate()
		}
	case "/v1/terminal-resize":
		err = decodeFeature(raw, &resize)
		reference = resize.Reference
		if !contract.ValidTerminalSize(resize.Columns, resize.Rows) {
			err = errors.New("invalid terminal size")
		}
	case "/v1/terminal-read", "/v1/terminal-pulse":
		err = decodeFeature(raw, &read)
		reference = read.Reference
	case "/v1/terminal-close", "/v1/terminal-release":
		err = decodeFeature(raw, &reference)
	default:
		return nil, errors.New("unknown terminal operator action")
	}
	if err != nil || reference.Validate() != nil || reference.Epoch != state.epoch {
		return nil, errors.New("invalid terminal reference")
	}
	record := state.records[reference.ID]
	if record == nil || record.open.Reference() != reference || !terminalOwner(record, admission) {
		return nil, errors.New("terminal does not belong to this target and OS session")
	}
	if path == "/v1/terminal-read" {
		return record.view(&read.Cursor)
	}
	if path == "/v1/terminal-release" {
		if record.state != "closed" {
			return nil, errors.New("terminal cleanup has not completed")
		}
		record.release()
		return record.view(nil)
	}
	if path == "/v1/terminal-close" {
		record.close = true
		if record.state == "queued" {
			record.state, record.reason = "closed", "closed"
			if record.input != nil {
				clear(record.input.Data)
				record.input = nil
			}
		}
		return record.view(nil)
	}
	if record.close || record.released || record.state == "closed" || record.state == "unknown" {
		if path == "/v1/terminal-pulse" {
			return record.view(&read.Cursor)
		}
		return record.view(nil)
	}
	// Only these execute-scoped calls renew an existing owner's authority.
	record.admission = admission
	if path == "/v1/terminal-input" {
		digest := contract.TerminalInputDigest(input.Data)
		if input.Sequence == record.ackSequence && digest == record.ackDigest {
			return record.view(nil)
		}
		if record.input != nil {
			if input.Sequence == record.input.Sequence && digest == contract.TerminalInputDigest(record.input.Data) {
				return record.view(nil)
			}
			if input.Sequence == record.input.Sequence {
				record.close = true
			}
			return nil, errors.New("terminal input pending or changed retry")
		}
		if input.Sequence != record.ackSequence+1 {
			if input.Sequence == record.ackSequence {
				record.close = true
			}
			return nil, errors.New("terminal input sequence is not next")
		}
		input.Data = append([]byte(nil), input.Data...)
		record.input = &input
		record.pendingSequence, record.pendingDigest = input.Sequence, digest
	}
	if path == "/v1/terminal-resize" {
		record.columns, record.rows = resize.Columns, resize.Rows
	}
	if path == "/v1/terminal-pulse" {
		return record.view(&read.Cursor)
	}
	return record.view(nil)
}

func terminalOwner(record *terminalRecord, admission operatorAdmission) bool {
	return record.admission.session == admission.session && record.admission.generation == admission.generation
}

func (broker *Broker) terminalLease(record *terminalRecord) int64 {
	if record.close || record.released || record.state == "closed" || record.state == "unknown" || broker.security == nil || broker.security.CheckAdmission(record.admission) != nil {
		return 0
	}
	catalog, exists := broker.terminalState().catalogs[record.open.Target.Node]
	if !exists || catalog.Target != record.open.Target || !broker.featureCatalogCurrent(catalog) || !terminalRoot(catalog, record.open.Root) {
		return 0
	}
	until := time.Now().Add(contract.FeatureLease)
	if record.admission.deadline.Before(until) {
		until = record.admission.deadline
	}
	if (record.state == "queued" || record.state == "opening") && record.open.Deadline.Before(until) {
		until = record.open.Deadline
	}
	return max(int64(0), time.Until(until).Milliseconds())
}

func (broker *Broker) terminalWorker(node string, raw []byte) (any, error) {
	state := broker.terminalState()
	enrollment := broker.state.Nodes[node]
	var exchange contract.TerminalExchange
	if enrollment.MachineKey == "" || enrollment.Revoked || decodeFeature(raw, &exchange) != nil || exchange.Schema != contract.TerminalVersion ||
		exchange.Catalog.Validate() != nil || exchange.Catalog.TerminalEpoch != "" || len(exchange.Reports) > contract.MaxTerminalSessions ||
		exchange.Catalog.Target.Node != node || exchange.Catalog.Target.Fingerprint != remoteidentity.Fingerprint(enrollment.MachineKey) {
		return nil, errors.New("invalid authenticated terminal worker exchange")
	}
	response := contract.TerminalExchangeResult{Schema: contract.TerminalVersion, Epoch: state.epoch, Directives: []contract.TerminalDirective{}}
	if exchange.Epoch != state.epoch {
		return response, nil
	}
	exchange.Catalog.ReceivedAt = time.Now().UTC()
	state.catalogs[node] = exchange.Catalog
	broker.features.catalogs[node] = exchange.Catalog
	for _, report := range exchange.Reports {
		if report.Validate() != nil || report.Reference.Epoch != state.epoch || report.Reference.Target != exchange.Catalog.Target {
			return nil, errors.New("invalid terminal report identity")
		}
		record := state.records[report.Reference.ID]
		if record == nil {
			response.Forget = append(response.Forget, report.Reference)
			continue
		}
		if record.open.Reference() != report.Reference {
			return nil, errors.New("terminal report changed target")
		}
		if record.released {
			response.Forget = append(response.Forget, report.Reference)
			continue
		}
		if report.AckSequence != record.ackSequence {
			if report.AckSequence != record.pendingSequence || report.AckDigest != record.pendingDigest {
				return nil, errors.New("terminal input acknowledgement changed")
			}
			record.ackSequence, record.ackDigest = report.AckSequence, report.AckDigest
			if record.input != nil {
				clear(record.input.Data)
			}
			record.input = nil
			record.pendingSequence, record.pendingDigest = 0, ""
		} else if report.AckDigest != record.ackDigest {
			return nil, errors.New("terminal input acknowledgement digest changed")
		}
		if record.output != nil {
			if err := record.output.append(report.OutputStart, report.Data, report.Truncated); err != nil {
				return nil, err
			}
		}
		if record.state != "closed" && record.state != "unknown" || report.State == "unknown" {
			record.state, record.reason = report.State, report.Reason
		}
		if record.state == "closed" || record.state == "unknown" {
			if record.input != nil {
				clear(record.input.Data)
				record.input = nil
			}
		}
		// Once native cleanup is proven and the original owner is no longer
		// admitted, discard private output and free quota. Keep only the open
		// tombstone until its deadline; unknown cleanup is never reclaimed here.
		if record.state == "closed" && (broker.security == nil || broker.security.CheckAdmission(record.admission) != nil) {
			record.release()
			response.Forget = append(response.Forget, report.Reference)
		}
	}
	for _, record := range state.records {
		if record.released {
			continue
		}
		if record.open.Target.Node != node {
			continue
		}
		if record.open.Target != exchange.Catalog.Target {
			if record.state != "closed" {
				record.state, record.reason, record.close = "unknown", "authority-lost", true
			}
			continue
		}
		lease := broker.terminalLease(record)
		if lease == 0 {
			record.close = true
			if record.input != nil {
				clear(record.input.Data)
				record.input = nil
			}
		}
		if lease == 0 && record.state == "queued" {
			record.state, record.reason, record.close = "closed", "authority-lost", true
		}
		// Once dispatched, cancellation must await the worker's cleanup proof.
		// A lost assignment response is not evidence that no PTY was started.
		if lease > 0 && record.state == "queued" {
			record.state = "opening"
		}
		directive := contract.TerminalDirective{Open: record.open, Operator: Hash(fmt.Sprintf("%s:%d", record.admission.session, record.admission.generation)),
			LeaseMillis: lease, Close: lease == 0 || record.close, Input: record.input, Columns: record.columns, Rows: record.rows, Forget: record.released}
		if record.output != nil {
			directive.OutputAck = record.output.end
		}
		response.Directives = append(response.Directives, directive)
	}
	return response, nil
}
