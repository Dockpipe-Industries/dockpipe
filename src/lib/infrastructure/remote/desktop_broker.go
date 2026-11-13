package remote

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type desktopBroker struct {
	epoch    string
	records  map[string]*desktopRecord
	catalogs map[string]contract.DesktopCatalog
}

type desktopRecord struct {
	open      contract.DesktopOpen
	admission operatorAdmission
	state     string
	reason    string
	close     bool
	closingAt time.Time
	viewer    desktopSignalQueue
	worker    desktopSignalQueue
	bytes     int
	relay     *desktopRelay
}

type desktopSignalQueue struct {
	signals     []contract.DesktopSignal
	ack         uint64
	described   bool
	parameters  bool
	fingerprint string
}

func (broker *Broker) desktopState() *desktopBroker {
	if broker.desktops == nil {
		broker.desktops = &desktopBroker{epoch: Hash(rand.Text()), records: map[string]*desktopRecord{}, catalogs: map[string]contract.DesktopCatalog{}}
	}
	return broker.desktops
}

func desktopOperatorPath(path string) bool {
	switch path {
	case contract.DesktopRelayTicketPath, "/v1/desktop-catalog", "/v1/desktop-open", "/v1/desktop-status", "/v1/desktop-pulse", "/v1/desktop-close", "/v1/desktop-signal":
		return true
	}
	return false
}

func (broker *Broker) desktopCatalogCurrent(catalog contract.DesktopCatalog) bool {
	enrollment, exists := broker.state.Nodes[catalog.Target.Node]
	return exists && !enrollment.Revoked && enrollment.MachineKey != "" &&
		remoteidentity.Fingerprint(enrollment.MachineKey) == catalog.Target.Fingerprint && time.Since(catalog.ReceivedAt) <= contract.DesktopCatalogFreshness
}

func (broker *Broker) desktopLease(record *desktopRecord) int64 {
	catalog, exists := broker.desktopState().catalogs[record.open.Target.Node]
	if record.close || !exists || catalog.Target != record.open.Target || catalog.Provider != record.open.Provider ||
		!broker.desktopCatalogCurrent(catalog) || !catalog.View || record.open.Mode == "control" && !catalog.Input ||
		broker.security == nil || broker.security.CheckAdmission(record.admission) != nil || !time.Now().Before(record.open.Deadline) {
		return 0
	}
	until := time.Now().Add(contract.DesktopLease)
	if record.admission.deadline.Before(until) {
		until = record.admission.deadline
	}
	if record.open.Deadline.Before(until) {
		until = record.open.Deadline
	}
	return max(0, time.Until(until).Milliseconds())
}

func (record *desktopRecord) view(after uint64) (contract.DesktopView, error) {
	if after > uint64(len(record.worker.signals)) {
		return contract.DesktopView{}, errors.New("desktop signaling cursor exceeds publication")
	}
	view := contract.DesktopView{Reference: record.open.Reference(), State: record.state, Reason: record.reason, Ack: record.viewer.ack}
	if !record.close && record.state != "closed" && record.state != "unknown" {
		view.Signals = append([]contract.DesktopSignal(nil), record.worker.signals[after:]...)
	}
	return view, nil
}

func (broker *Broker) desktopView(record *desktopRecord, after uint64) (contract.DesktopView, error) {
	view, err := record.view(after)
	if err == nil && record.state != "closing" && record.state != "closed" && record.state != "unknown" {
		view.LeaseMillis = broker.desktopLease(record)
	}
	return view, err
}

func (record *desktopRecord) addSignal(queue *desktopSignalQueue, signal contract.DesktopSignal, viewer bool) error {
	if signal.Validate() != nil || signal.Reference != record.open.Reference() || record.close || record.state == "closed" || record.state == "unknown" ||
		viewer && signal.Type == "answer" || !viewer && (signal.Type == "offer" || signal.Type == "parameters") {
		return errors.New("desktop signaling does not match the admitted endpoint")
	}
	if signal.Sequence <= uint64(len(queue.signals)) {
		previous, _ := json.Marshal(queue.signals[signal.Sequence-1])
		current, _ := json.Marshal(signal)
		if string(previous) != string(current) {
			return errors.New("desktop signaling retry changed its original sequence")
		}
		return nil
	}
	if signal.Sequence != uint64(len(queue.signals))+1 || len(queue.signals) >= contract.MaxDesktopSignals {
		return errors.New("desktop signaling sequence or message budget exceeded")
	}
	switch signal.Type {
	case "binding":
		if queue.fingerprint != "" || queue.described {
			return errors.New("desktop endpoint binding is immutable before negotiation")
		}
	case "parameters":
		if !viewer || queue.parameters || queue.described || signal.Sequence != 1 {
			return errors.New("desktop connection parameters must precede the first description")
		}
	case "offer", "answer":
		if queue.described || !viewer && !record.viewer.described {
			return errors.New("desktop description order changed; renegotiation requires new authorization")
		}
	default:
		if !queue.described {
			return errors.New("desktop endpoint must publish its description before ICE")
		}
	}
	encoded, _ := json.Marshal(signal)
	if record.bytes+len(encoded) > contract.MaxDesktopSignalBytes {
		return errors.New("desktop aggregate signaling budget exhausted")
	}
	queue.signals = append(queue.signals, signal)
	if signal.Type == "binding" {
		queue.fingerprint = signal.Fingerprint
	}
	queue.described = queue.described || signal.Type == "offer" || signal.Type == "answer"
	queue.parameters = queue.parameters || signal.Type == "parameters"
	record.bytes += len(encoded)
	return nil
}

func (broker *Broker) desktopOperator(path string, raw []byte, admission operatorAdmission) (any, error) {
	state := broker.desktopState()
	if path == contract.DesktopRelayTicketPath {
		return broker.desktopRelayTicket(raw, "viewer", "", admission)
	}
	if path == "/v1/desktop-catalog" {
		var request struct {
			Schema string `json:"schema"`
		}
		if decodeFeature(raw, &request) != nil || request.Schema != contract.DesktopVersion {
			return nil, errors.New("invalid desktop discovery request")
		}
		catalogs := []contract.DesktopCatalog{}
		for _, catalog := range state.catalogs {
			if broker.desktopCatalogCurrent(catalog) && catalog.View {
				catalog.Epoch = state.epoch
				catalogs = append(catalogs, catalog)
			}
		}
		sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].Target.Node < catalogs[j].Target.Node })
		return catalogs, nil
	}
	if path == "/v1/desktop-open" {
		var request contract.DesktopOpen
		if decodeFeature(raw, &request) != nil || request.Validate() != nil || request.Epoch != state.epoch ||
			!request.Deadline.After(time.Now()) || request.Deadline.After(time.Now().Add(contract.DesktopLifetime)) {
			return nil, errors.New("invalid or expired desktop open intent")
		}
		if previous := state.records[request.ID]; previous != nil {
			if !desktopOwner(previous, admission) {
				return nil, errors.New("desktop opening belongs to a different OS admission")
			}
			return broker.desktopView(previous, uint64(len(previous.worker.signals)))
		}
		catalog, exists := state.catalogs[request.Target.Node]
		if !exists || catalog.Target != request.Target || catalog.Provider != request.Provider || !broker.desktopCatalogCurrent(catalog) ||
			!catalog.View || request.Mode == "control" && !catalog.Input {
			return nil, errors.New("desktop target, provider or independent grant is unavailable")
		}
		for id, previous := range state.records {
			if previous.state == "closed" && time.Now().After(previous.open.Deadline) {
				delete(state.records, id)
				continue
			}
			if previous.open.Target.Node == request.Target.Node && previous.state != "closed" {
				return nil, errors.New("existing or uncertain desktop session consumes this machine's capture/input reservation")
			}
		}
		if len(state.records) >= contract.MaxDesktopSessions {
			return nil, errors.New("desktop session capacity reached")
		}
		record := &desktopRecord{open: request, admission: admission, state: "queued"}
		state.records[request.ID] = record
		return broker.desktopView(record, 0)
	}
	var read contract.DesktopRead
	var signal contract.DesktopSignal
	if path == "/v1/desktop-signal" {
		if decodeFeature(raw, &signal) != nil || signal.Validate() != nil {
			return nil, errors.New("invalid desktop signaling")
		}
		read.Reference = signal.Reference
	} else if path == "/v1/desktop-close" {
		if decodeFeature(raw, &read.Reference) != nil {
			return nil, errors.New("invalid desktop closure reference")
		}
	} else if decodeFeature(raw, &read) != nil {
		return nil, errors.New("invalid desktop session lookup")
	}
	record := state.records[read.Reference.ID]
	if read.Reference.Validate() != nil || read.Reference.Epoch != state.epoch || record == nil || record.open.Reference() != read.Reference {
		return nil, errors.New("desktop lookup changed its exact session or OS owner")
	}
	metadataRecovery := path == "/v1/desktop-status" && read.MetadataOnly
	if read.MetadataOnly && (path != "/v1/desktop-status" || read.After != 0) {
		return nil, errors.New("desktop metadata status cannot renew or select signaling")
	}
	if metadataRecovery || path == "/v1/desktop-close" {
		enrollment, exists := broker.state.Nodes[record.open.Target.Node]
		if !exists || remoteidentity.Fingerprint(enrollment.MachineKey) != record.open.Target.Fingerprint || broker.security == nil || broker.security.CheckAdmission(admission) != nil {
			return nil, errors.New("desktop recovery requires the current OS account and original paired identity")
		}
	} else if !desktopOwner(record, admission) {
		return nil, errors.New("desktop signaling and renewal belong to the original OS admission")
	}
	if path == "/v1/desktop-close" {
		record.close = true
		if record.state == "queued" {
			record.state = "closed"
		} else if record.state != "closed" {
			record.state, record.reason, record.closingAt = "closing", "", time.Now()
		}
	}
	if path == "/v1/desktop-pulse" || path == "/v1/desktop-signal" {
		if !record.close {
			record.admission = admission
		}
	}
	if path == "/v1/desktop-signal" {
		if broker.desktopLease(record) <= 0 {
			return nil, errors.New("desktop signaling authority expired")
		}
		if err := record.addSignal(&record.viewer, signal, true); err != nil {
			return nil, err
		}
		read.After = uint64(len(record.worker.signals))
	}
	if broker.desktopLease(record) <= 0 && record.state != "closed" && (record.state != "closing" || time.Since(record.closingAt) >= contract.DesktopLease) {
		record.close, record.state, record.reason = true, "unknown", "authority-lost"
	}
	if metadataRecovery {
		view, err := record.view(uint64(len(record.worker.signals)))
		return view, err
	}
	return broker.desktopView(record, read.After)
}

func desktopOwner(record *desktopRecord, admission operatorAdmission) bool {
	return record.admission.session == admission.session && record.admission.generation == admission.generation
}

func (broker *Broker) desktopWorker(node string, raw []byte) (any, error) {
	state := broker.desktopState()
	var exchange contract.DesktopExchange
	enrollment := broker.state.Nodes[node]
	if decodeFeature(raw, &exchange) != nil || exchange.Schema != contract.DesktopVersion || exchange.Catalog.Validate() != nil ||
		exchange.Catalog.Target.Node != node || enrollment.Revoked || enrollment.MachineKey == "" ||
		exchange.Catalog.Target.Fingerprint != remoteidentity.Fingerprint(enrollment.MachineKey) || len(exchange.Reports) > 1 {
		return nil, errors.New("invalid authenticated desktop worker exchange")
	}
	response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: state.epoch}
	if exchange.Epoch != state.epoch {
		if exchange.Epoch != "" || len(exchange.Reports) != 0 {
			return nil, errors.New("desktop worker epoch changed; close old native channels before reconnecting")
		}
		return response, nil
	}
	exchange.Catalog.ReceivedAt = time.Now().UTC()
	exchange.Catalog.Epoch = ""
	state.catalogs[node] = exchange.Catalog
	for _, report := range exchange.Reports {
		record := state.records[report.Reference.ID]
		if report.Reference.Validate() != nil || report.Reference.Epoch != state.epoch || report.Reference.Target != exchange.Catalog.Target ||
			len(report.Signals) > contract.MaxDesktopSignals || report.State != "running" && report.State != "closed" && report.State != "unknown" && report.State != "opening" {
			return nil, errors.New("invalid desktop worker report identity or state")
		}
		if record == nil {
			response.Forget = append(response.Forget, report.Reference)
			continue
		}
		if record.open.Reference() != report.Reference || report.Ack < record.viewer.ack || report.Ack > uint64(len(record.viewer.signals)) {
			return nil, errors.New("desktop worker changed the signaling acknowledgment")
		}
		if record.state == "running" && report.State == "opening" {
			return nil, errors.New("running desktop session cannot repeat opening effects")
		}
		if record.state == "closed" && report.State != "closed" {
			return nil, errors.New("closed desktop session cannot be restarted")
		}
		if report.State == "closed" || report.State == "unknown" {
			record.close = true
		}
		if !record.close {
			for _, signal := range report.Signals {
				if err := record.addSignal(&record.worker, signal, false); err != nil {
					return nil, err
				}
			}
		}
		record.viewer.ack = report.Ack
		if !record.close || report.State == "closed" || report.State == "unknown" {
			record.state = report.State
		}
		if report.State == "closed" {
			record.reason = "closed"
			response.Forget = append(response.Forget, report.Reference)
			clear(record.viewer.signals)
			clear(record.worker.signals)
		}
	}
	for _, record := range state.records {
		if record.open.Target.Node != node || record.state == "closed" {
			continue
		}
		lease := broker.desktopLease(record)
		if record.open.Target != exchange.Catalog.Target || lease <= 0 {
			record.close = true
			if record.state != "closing" || time.Since(record.closingAt) >= contract.DesktopLease {
				record.state, record.reason = "unknown", "authority-lost"
			}
		}
		if record.open.Target != exchange.Catalog.Target {
			continue
		}
		if record.state == "queued" {
			record.state = "opening"
		}
		admission := record.admission
		directive := contract.DesktopDirective{Open: record.open, Close: record.close, LeaseMillis: lease,
			Operator: Hash(fmt.Sprintf("%s:%d", admission.session, admission.generation)), Ack: uint64(len(record.worker.signals))}
		if !record.close {
			directive.Signals = append([]contract.DesktopSignal(nil), record.viewer.signals[record.viewer.ack:]...)
		}
		response.Directives = append(response.Directives, directive)
	}
	return response, nil
}

// Desktop paths are an explicit allowlist. Unrecognized prefixed operations do
// not acquire a signing scope or arbitrary command authority.
func desktopPath(path string) bool {
	return desktopOperatorPath(path) || path == "/v1/desktop-worker"
}
