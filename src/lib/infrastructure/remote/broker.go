package remote

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type Enrollment struct {
	SecretHash string    `json:"secret_hash"`
	ExpiresAt  time.Time `json:"expires_at"`
	TokenHash  string    `json:"token_hash,omitempty"`
	Revoked    bool      `json:"revoked"`
}

type BrokerState struct {
	Pairings     map[string]PairingSession `json:"pairings,omitempty"`
	PairingUntil time.Time                 `json:"pairing_until,omitempty"`
	Schema       string                    `json:"schema"`
	AdminHash    string                    `json:"admin_hash"`
	Nodes        map[string]Enrollment     `json:"nodes"`
	Jobs         map[string]contract.Job   `json:"jobs"`
}

type Broker struct {
	mutex            sync.Mutex
	path             string
	state            BrokerState
	poisoned         bool
	pairingRateStart time.Time
	pairingRateCount int
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
	if err := ReadPrivate(b.path, &b.state); err != nil {
		return nil, err
	}
	if b.state.Schema != contract.Version || len(b.state.AdminHash) != 64 || b.state.Nodes == nil || b.state.Jobs == nil {
		return nil, errors.New("invalid broker state")
	}
	if b.state.Pairings == nil {
		b.state.Pairings = map[string]PairingSession{}
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
		b.state.Jobs[id] = job.Recover()
	}
	if err := b.save(); err != nil {
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
	if r.Method != http.MethodPost || r.URL.RawQuery != "" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	limit := int64(contract.MaxBody)
	if r.URL.Path == "/v1/pairing-request" {
		limit = 1024
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.poisoned {
		http.Error(w, "broker storage requires restart and inspection", http.StatusServiceUnavailable)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.URL.Path == "/v1/pairing-request" {
		b.respond(w, func() (any, error) { return b.requestPairing(body) })
		return
	}
	if r.URL.Path == "/v1/pair" {
		b.respond(w, func() (any, error) { return b.pair(body) })
		return
	}
	if matches(token, b.state.AdminHash) {
		b.respond(w, func() (any, error) { return b.operator(r.URL.Path, body) })
		return
	}
	for node, enrollment := range b.state.Nodes {
		if !enrollment.Revoked && matches(token, enrollment.TokenHash) {
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
	if time.Now().After(enrollment.ExpiresAt) {
		return nil, errors.New("invitation expired")
	}
	enrollment.TokenHash = Hash(request.Token)
	b.state.Nodes[request.Node] = enrollment
	return map[string]string{"node": request.Node}, b.save()
}

func (b *Broker) operator(path string, raw []byte) (any, error) {
	switch path {
	case "/v1/pairing-open", "/v1/pairing-close", "/v1/pairings", "/v1/approve", "/v1/deny":
		return b.pairingOperator(path, raw)
	case "/v1/nodes":
		nodes := []contract.NodeStatus{}
		for name, enrollment := range b.state.Nodes {
			status := "invited"
			if enrollment.TokenHash != "" {
				status = "paired"
			}
			if enrollment.Revoked {
				status = "revoked"
			}
			nodes = append(nodes, contract.NodeStatus{Node: name, Status: status})
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Node < nodes[j].Node })
		return nodes, nil
	case "/v1/health":
		return map[string]string{"schema": contract.Version}, nil
	case "/v1/invite":
		var request struct {
			Node string `json:"node"`
		}
		if err := Decode(raw, &request); err != nil || !contract.ValidID(request.Node) {
			return nil, errors.New("invalid node")
		}
		if _, exists := b.state.Nodes[request.Node]; exists || len(b.state.Nodes) >= 64 {
			return nil, errors.New("node already exists or node capacity reached")
		}
		secret, err := Secret()
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(15 * time.Minute)
		b.state.Nodes[request.Node] = Enrollment{SecretHash: Hash(secret), ExpiresAt: expires}
		return contract.Invitation{Schema: contract.Version, Node: request.Node, Secret: secret, ExpiresAt: expires}, b.save()
	case "/v1/submit":
		var request contract.SubmitRequest
		if err := Decode(raw, &request); err != nil {
			return nil, err
		}
		node, exists := b.state.Nodes[request.Node]
		enrolled := exists && !node.Revoked && node.TokenHash != ""
		job, changed, err := contract.Queue(b.state.Jobs).Submit(request.Submission, enrolled, time.Now())
		if err != nil {
			return nil, err
		}
		if err := b.storeBundle(request); err != nil {
			return nil, err
		}
		if !changed {
			return b.withResult(job)
		}
		b.state.Jobs[job.ID] = job
		return job, b.save()
	case "/v1/jobs":
		jobs := make([]contract.Job, 0, len(b.state.Jobs))
		for _, job := range b.state.Jobs {
			job.Result = nil
			jobs = append(jobs, job)
		}
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.Before(jobs[j].CreatedAt) })
		return jobs, nil
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
			return map[string]string{"revoked": request.ID}, b.save()
		}
		job, ok := b.state.Jobs[request.ID]
		if !ok {
			return nil, errors.New("job not found")
		}
		if path == "/v1/cancel" {
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
	switch path {
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
		job, err := contract.Queue(b.state.Jobs).Assigned(request.ID, node, request.Session)
		if err != nil {
			return nil, err
		}
		if path == "/v1/bundle" {
			return b.loadBundle(job)
		}
		if path == "/v1/pulse" {
			return map[string]bool{"cancel": job.CancelRequested}, nil
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
		return map[string]string{"id": job.ID}, b.save()
	}
	return nil, errors.New("unknown worker operation")
}

// SafeRelative retains the infrastructure entrypoint for existing callers.
func SafeRelative(path string) bool {
	return contract.SafeRelative(path)
}
