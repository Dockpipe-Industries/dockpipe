package remote

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/hardwarekey"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type Enrollment struct {
	SecretHash string    `json:"secret_hash"`
	ExpiresAt  time.Time `json:"expires_at"`
	TokenHash  string    `json:"token_hash,omitempty"`
	Revoked    bool      `json:"revoked"`
	MachineKey string    `json:"machine_key,omitempty"`
	GuestGrant string    `json:"guest_grant,omitempty"`
}

type BrokerState struct {
	GuestGrants      map[string]guestGrant     `json:"guest_grants,omitempty"`
	OperatorSecurity string                    `json:"operator_security,omitempty"`
	Identity         *remoteidentity.Identity  `json:"machine_identity,omitempty"`
	PairingKeys      *hardwarekey.Registry     `json:"pairing_keys,omitempty"`
	TerminalArchive  bool                      `json:"terminal_archive,omitempty"`
	Pairings         map[string]PairingSession `json:"pairings,omitempty"`
	PairingUntil     time.Time                 `json:"pairing_until,omitempty"`
	Schema           string                    `json:"schema"`
	AdminHash        string                    `json:"admin_hash"`
	Nodes            map[string]Enrollment     `json:"nodes"`
	Jobs             map[string]contract.Job   `json:"jobs"`
}

type Broker struct {
	networks          *networkBroker
	terminals         *terminalBroker
	desktops          *desktopBroker
	features          *featureBroker
	edge              *EdgeSupervisor
	operatorTokenHash string
	security          *SecurityAuthority
	pairingChallenges map[string]hardwarePairingChallenge
	machineChallenges map[string]machineChallenge
	monitoring        map[string]MachineSnapshot
	liveLogs          map[string]contract.LiveLog
	mutex             sync.Mutex
	path              string
	state             BrokerState
	poisoned          bool
	pairingRateStart  time.Time
	pairingRateCount  int
}

func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func matches(secret, hash string) bool {
	return len(secret) == 64 && subtle.ConstantTimeCompare([]byte(Hash(secret)), []byte(hash)) == 1
}

func NewBroker(root string) (*Broker, error) {
	b := &Broker{path: filepath.Join(root, "broker.json")}
	state, err := readBrokerStateForStartup(b.path)
	if err != nil {
		return nil, err
	}
	b.state = state
	if len(b.state.Nodes) > 64 || len(b.state.Jobs) > contract.MaxJobs || b.state.Schema != contract.Version || len(b.state.AdminHash) != 64 || b.state.Nodes == nil || b.state.Jobs == nil {
		return nil, errors.New("invalid broker state")
	}
	if b.state.Identity == nil {
		identity, err := remoteidentity.New()
		if err != nil {
			return nil, err
		}
		b.state.Identity = &identity
	}
	if b.state.OperatorSecurity != "" && b.state.OperatorSecurity != contract.OperatorSecurityVersion {
		return nil, errors.New("unsupported broker operator security protocol")
	}
	b.state.OperatorSecurity = contract.OperatorSecurityVersion
	b.operatorTokenHash = b.state.AdminHash
	if err := b.state.Identity.Validate(); err != nil {
		return nil, err
	}
	if b.state.PairingKeys != nil {
		if err := b.state.PairingKeys.Validate(); err != nil {
			return nil, err
		}
	}
	if b.state.Pairings == nil {
		b.state.Pairings = map[string]PairingSession{}
	}
	if err := b.validateGuestGrants(); err != nil {
		return nil, err
	}
	if err := b.loadDesktopAdministratorServices(root); err != nil {
		return nil, err
	}
	// Claimed jobs are never automatically requeued after a crash. The worker
	// journal may deliver a completed result, otherwise the operator sees unknown.
	for id, job := range b.state.Jobs {
		if id != job.ID || job.Validate() != nil {
			return nil, errors.New("invalid stored job identity")
		}
		if job.Result != nil {
			if err := b.storeResult(&job, job.Result); err != nil {
				return nil, err
			}
		}
		if err := b.restoreJobInterruptionAcknowledgement(&job); err != nil {
			return nil, err
		}
		b.state.Jobs[id] = job
		if _, _, err := b.lookupJob(id); err != nil {
			return nil, err
		}
		b.state.Jobs[id] = job.Recover()
	}
	if err := b.save(); err != nil {
		return nil, err
	}
	if err := b.loadFeatures(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Broker) save() error {
	data, err := json.Marshal(b.state)
	if err == nil && len(data) > 32<<20 {
		err = errors.New("broker metadata exceeds its 32 MiB bound")
	}
	if err == nil {
		err = WritePrivate(b.path, b.state)
	}
	if err != nil {
		// A failed durable publication can have unknown outcome. No later request
		// may acknowledge or execute against uncertain in-memory state.
		b.poisoned = true
	}
	return err
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == contract.NetworkRelayPath {
		b.serveNetworkRelay(w, r)
		return
	}
	if r.URL.Path == contract.DesktopRelayPath || r.URL.Path == contract.DesktopAdministratorRelayPath {
		b.serveDesktopRelay(w, r)
		return
	}
	if r.Method != http.MethodPost || r.URL.RawQuery != "" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	limit := int64(contract.MaxBody)
	if strings.HasPrefix(r.URL.Path, "/v1/network-") {
		limit = 4096
	}
	if r.URL.Path == "/v1/network-trust-publish" {
		limit = contract.MaxNetworkTrustBody
	}
	if desktopPath(r.URL.Path) {
		limit = contract.MaxDesktopBody
	}
	if terminalPath(r.URL.Path) {
		limit = contract.MaxTerminalBody
	}
	if r.URL.Path == "/v1/monitor-publish" {
		limit = MaxMonitoringBytes
	}
	switch r.URL.Path {
	case "/v1/health", "/v1/edge-health", "/v1/job-log", contract.JobInterruptionInspectPath, contract.JobInterruptionAcknowledgePath:
		limit = 1024
	case "/v1/job-log-publish":
		// The payload is a base64-encoded preview plus bounded job and session IDs.
		limit = (contract.MaxLiveLog+2)/3*4 + 1024
	case contract.DesktopAdministratorGrantPath, contract.DesktopAdministratorApprovalPath,
		contract.DesktopAdministratorRelayTicketPath, contract.DesktopAdministratorServicePath, "/v1/pairing-request", "/v1/guest-grant", "/v1/guest-grant-revoke", "/v1/guest-enroll", "/v1/guest-release":
		limit = 4096
	case "/v1/machine-challenge", "/v1/approve-begin":
		limit = 1024
	case "/v1/approve":
		limit = hardwarekey.MaxResponseBytes + 1024
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var admission operatorAdmission
	var authorizationError error
	if matches(token, b.operatorTokenHash) && contract.OperatorScope(r.URL.Path) != "" {
		if b.security == nil || len(r.Header.Values(operatorProofHeader)) != 1 {
			authorizationError = errors.New("OS-authenticated remote access is locked or unavailable")
		} else {
			admission, authorizationError = b.security.Verify(r.Context(), r.Header.Get(operatorProofHeader), r.URL.Path, body)
		}
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.poisoned {
		http.Error(w, "broker storage requires restart and inspection", http.StatusServiceUnavailable)
		return
	}
	if r.URL.Path == "/v1/edge-health" {
		b.respond(w, func() (any, error) { return b.proveEdgeHealth(body) })
		return
	}
	if r.URL.Path == "/v1/health" {
		b.respond(w, func() (any, error) {
			return struct {
				Schema string `json:"schema"`
			}{Schema: contract.Version}, nil
		})
		return
	}
	if r.URL.Path == "/v1/pairing-request" {
		b.respond(w, func() (any, error) { return b.requestPairing(body) })
		return
	}
	if r.URL.Path == "/v1/pair" {
		b.respond(w, func() (any, error) { return b.pair(body) })
		return
	}
	if r.URL.Path == contract.DesktopAdministratorServicePath ||
		r.URL.Path == contract.DesktopAdministratorRelayTicketPath && !matches(token, b.state.AdminHash) {
		b.serveDesktopAdministratorService(w, r.URL.Path, body, token)
		return
	}

	if matches(token, b.state.AdminHash) {
		b.serveOperator(w, r, body, admission, authorizationError)
		return
	}
	for node, enrollment := range b.state.Nodes {
		if matches(token, enrollment.TokenHash) && b.enrollmentActive(enrollment, time.Now()) {
			if enrollment.MachineKey != "" {
				b.serveMachine(w, r, node, enrollment, body)
				return
			}
			b.respond(w, func() (any, error) { return b.worker(node, r.URL.Path, body) })
			return
		}
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func (b *Broker) respond(w http.ResponseWriter, action func() (any, error)) {
	value, err := action()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func (b *Broker) pair(raw []byte) (any, error) {
	var request struct {
		Node   string `json:"node"`
		Secret string `json:"secret"`
		Token  string `json:"token"`
	}
	if err := Decode(raw, &request); err != nil {
		return nil, errors.New("invalid enrollment request")
	}
	enrollment, ok := b.state.Nodes[request.Node]
	if !ok || enrollment.Revoked || !matches(request.Secret, enrollment.SecretHash) || len(request.Token) != 64 {
		return nil, errors.New("enrollment rejected")
	}
	if _, err := hex.DecodeString(request.Token); err != nil {
		return nil, errors.New("invalid worker token")
	}
	// An exact retry can recover a lost response; the invitation cannot mint a
	// second token. The worker persists its token before the first network call.
	if enrollment.TokenHash != "" {
		if !matches(request.Token, enrollment.TokenHash) {
			return nil, errors.New("invitation already consumed")
		}
		return map[string]string{"node": request.Node}, nil
	}
	return nil, errors.New("unused file invitations are disabled; request hardware-key pairing")
}

func (b *Broker) operator(path string, raw []byte) (any, error) {
	if path == "/v1/submit" && b.edge != nil && b.edge.maintenanceActive() {
		return nil, errors.New("connector maintenance pauses new workflow submissions")
	}
	switch path {
	case "/v1/guest-grant", "/v1/guest-grant-revoke":
		return b.manageGuestGrant(path, raw)
	case "/v1/edge-plan", "/v1/edge-activate":
		if b.edge == nil {
			return nil, errors.New("edge supervision is unavailable")
		}
		var request contract.EdgeActivationRequest
		if err := Decode(raw, &request); err != nil {
			return nil, err
		}
		for _, job := range b.state.Jobs {
			if !job.Resolved() {
				return nil, errors.New("active jobs block connector replacement")
			}
		}
		return b.edge.accept(request, path == "/v1/edge-plan")
	case "/v1/edge-status":
		if b.edge == nil {
			return nil, errors.New("edge supervision is unavailable")
		}
		var request struct {
			ID string `json:"id"`
		}
		if err := Decode(raw, &request); err != nil || !contract.ValidEdgeDigest(request.ID) {
			return nil, errors.New("invalid activation identity")
		}
		return b.edge.status(request.ID)
	case "/v1/observe":
		return b.observeMachine(raw)
	case "/v1/remove":
		return b.removeNode(raw)
	case "/v1/pairing-open", "/v1/pairing-close", "/v1/pairings", "/v1/approve", "/v1/deny":
		return b.pairingOperator(path, raw)
	case "/v1/nodes":
		nodes := []contract.NodeStatus{}
		for name, enrollment := range b.state.Nodes {
			status := "invited"
			if enrollment.TokenHash != "" {
				status = "paired"
			}
			if !b.enrollmentActive(enrollment, time.Now()) {
				status = "revoked"
			}
			nodes = append(nodes, contract.NodeStatus{Node: name, Status: status, Fingerprint: remoteidentity.Fingerprint(enrollment.MachineKey)})
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Node < nodes[j].Node })
		return nodes, nil
	case "/v1/health":
		return map[string]string{"schema": contract.Version}, nil
	case "/v1/invite":
		return nil, errors.New("file invitations are disabled; use hardware-key pairing")
	case "/v1/approve-begin":
		return b.beginHardwarePairing(raw)
	case "/v1/submit":
		var request contract.SubmitRequest
		if err := Decode(raw, &request); err != nil {
			return nil, err
		}
		if err := request.Submission.Validate(); err != nil {
			return nil, err
		}
		previous, _, err := b.lookupJob(request.ID)
		if err == nil {
			if previous.Submission != request.Submission {
				return nil, errors.New("job ID already binds different work")
			}
			if err := b.storeBundle(request); err != nil {
				return nil, err
			}
			return b.withResult(previous)
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		if _, err := b.readJobInterruptionAcknowledgement(request.ID); !os.IsNotExist(err) {
			return nil, errors.New("job interruption acknowledgement requires matching historical job metadata")
		}
		node, exists := b.state.Nodes[request.Node]
		if !exists || !b.enrollmentActive(node, time.Now()) || node.TokenHash == "" {
			return nil, errors.New("node is not enrolled")
		}
		if node.GuestGrant != "" && request.BundleHash == "" {
			return nil, errors.New("disposable guests only accept delivered workflows")
		}
		if err := b.validateDeliveryTarget(request.Submission); err != nil {
			return nil, err
		}
		jobs, retired, err := b.jobsForAdmission()
		if err != nil {
			return nil, err
		}
		if err := b.storeBundle(request); err != nil {
			return nil, err
		}
		job, _, err := contract.Queue(jobs).Submit(request.Submission, true, time.Now())
		if err != nil {
			return nil, err
		}
		jobs[job.ID] = job
		b.state.Jobs = jobs
		// This new field makes older strict decoders reject migrated state. Restoring
		// an old broker must never forget archived IDs and execute them again.
		b.state.TerminalArchive = b.state.TerminalArchive || retired
		return job, b.save()
	case "/v1/jobs":
		jobs := make([]contract.Job, 0, len(b.state.Jobs))
		for _, job := range b.state.Jobs {
			job.Result = nil
			jobs = append(jobs, job)
		}
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
		return jobs, nil
	case "/v1/job-log":
		return b.readLiveLog(raw)
	case contract.JobInterruptionInspectPath, contract.JobInterruptionAcknowledgePath:
		return b.jobInterruption(raw, path == contract.JobInterruptionAcknowledgePath)
	case "/v1/job", "/v1/cancel", "/v1/revoke":
		var request struct {
			ID string `json:"id"`
		}
		if err := Decode(raw, &request); err != nil {
			return nil, err
		}
		if path == "/v1/revoke" {
			node, ok := b.state.Nodes[request.ID]
			if !ok {
				return nil, errors.New("node not found")
			}
			node.Revoked = true
			b.state.Nodes[request.ID] = node
			delete(b.monitoring, request.ID)
			return map[string]string{"revoked": request.ID}, b.save()
		}
		job, archived, err := b.lookupJob(request.ID)
		if err != nil {
			return nil, err
		}
		if path == "/v1/cancel" && !archived {
			if cancelled, changed := job.Cancel(); changed {
				b.state.Jobs[job.ID] = cancelled
				return cancelled, b.save()
			}
		}
		return b.withResult(job)
	}
	return nil, errors.New("unknown operator operation")
}

func (b *Broker) worker(node, path string, raw []byte) (any, error) {
	if path == "/v1/network-trust-publish" {
		return b.publishNetworkTrust(node, raw)
	}
	if path == "/v1/network-worker" {
		return b.networkWorker(node, raw)
	}
	if path == contract.DesktopRelayTicketPath {
		return b.desktopRelayTicket(raw, "worker", node, operatorAdmission{})
	}
	if path == "/v1/desktop-worker" {
		return b.desktopWorker(node, raw)
	}
	if path == "/v1/terminal-worker" {
		return b.terminalWorker(node, raw)
	}
	if featureOperatorPath(path) {
		return b.featureWorker(node, path, raw)
	}
	switch path {
	case "/v1/monitor-publish":
		return b.publishMonitoring(node, raw)
	case "/v1/next":
		var request struct {
			Session string `json:"session"`
		}
		if err := Decode(raw, &request); err != nil {
			return nil, errors.New("worker session identity required")
		}
		selected, changed, err := contract.Queue(b.state.Jobs).Next(node, request.Session)
		if err != nil || selected == nil {
			return nil, err
		}
		if !changed {
			return b.withResult(*selected)
		}
		if targetErr := b.validateDeliveryTarget(selected.Submission); targetErr != nil {
			// No assignment has left the broker. Record the refusal as a terminal
			// failure so this obsolete queued target cannot stop the worker loop.
			result := &contract.Result{Status: "failure", ExitCode: -1, Log: targetErr.Error()}
			digest, err := resultHash(result)
			if err != nil {
				return nil, err
			}
			completed, _, err := selected.Complete(result, digest)
			if err != nil {
				return nil, err
			}
			if err := b.storeResult(&completed, result); err != nil {
				b.poisoned = true
				return nil, err
			}
			b.state.Jobs[completed.ID] = completed
			return nil, b.save()
		}
		b.state.Jobs[selected.ID] = *selected
		return selected, b.save()
	case "/v1/pulse", "/v1/result", "/v1/bundle":
		var request struct {
			ID      string           `json:"id"`
			Session string           `json:"session"`
			Result  *contract.Result `json:"result,omitempty"`
		}
		if err := Decode(raw, &request); err != nil {
			return nil, errors.New("invalid worker message")
		}
		job, _, err := b.lookupJob(request.ID)
		if err != nil {
			return nil, err
		}
		job, err = contract.Queue{job.ID: job}.Assigned(request.ID, node, request.Session)
		if err != nil {
			return nil, err
		}
		if path == "/v1/bundle" {
			if job.HasAcknowledgedInterruption() {
				return nil, errors.New("interrupted job no longer permits bundle delivery")
			}
			if err := b.validateDeliveryTarget(job.Submission); err != nil {
				return nil, err
			}
			return b.loadBundle(job)
		}
		if path == "/v1/pulse" {
			return map[string]bool{"cancel": job.CancelRequested || job.HasAcknowledgedInterruption() || b.validateDeliveryTarget(job.Submission) != nil}, nil
		}
		if request.Result == nil {
			return nil, errors.New("result required")
		}
		digest, err := resultHash(request.Result)
		if err != nil {
			return nil, err
		}
		completed, changed, err := job.Complete(request.Result, digest)
		if err != nil {
			return nil, err
		}
		if !changed {
			return map[string]string{"id": job.ID}, nil
		}
		if err := b.storeResult(&completed, request.Result); err != nil {
			b.poisoned = true
			return nil, err
		}
		job = completed
		b.state.Jobs[job.ID] = job
		delete(b.liveLogs, job.ID)
		return map[string]string{"id": job.ID}, b.save()
	}
	return nil, errors.New("unknown worker operation")
}

// SafeRelative retains the infrastructure entrypoint for existing callers.
func SafeRelative(path string) bool {
	return contract.SafeRelative(path)
}
