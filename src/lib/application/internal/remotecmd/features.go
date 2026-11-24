package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

const featureCommandTimeout = 25 * time.Second

func prepareFeature(ctx context.Context, input string, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	data, err := readFeatureInput(ctx, input)
	if err != nil {
		return err
	}
	request, err := decodeFeatureRequest(data, time.Now())
	if err != nil {
		return err
	}
	// Preparation never resolves operator state or contacts a broker. Emit only
	// metadata so a caller can retain this canonical identity before dispatch.
	metadata := struct {
		Schema   string                 `json:"schema"`
		ID       string                 `json:"id"`
		Target   contract.FeatureTarget `json:"target"`
		Root     string                 `json:"root,omitempty"`
		Action   string                 `json:"action"`
		Deadline time.Time              `json:"deadline"`
	}{Schema: request.Schema, ID: request.ID, Target: request.Target, Root: request.Root, Action: request.Action, Deadline: request.Deadline}
	return json.NewEncoder(output).Encode(metadata)
}

// The UI supplies the fixed nonce and deadline before dispatch. Omitting only
// ID delegates Go's canonical encoding to the CLI without creating a new intent.
func decodeFeatureRequest(data []byte, now time.Time) (contract.FeatureRequest, error) {
	var request contract.FeatureRequest
	if err := decodeFeatureInput(data, &request); err != nil {
		return request, err
	}
	if request.ID == "" {
		var err error
		request.ID, err = request.Digest()
		if err != nil {
			return request, err
		}
	}
	if err := request.Validate(); err != nil {
		return request, err
	}
	if err := remoteio.ValidateFeaturePayload(request); err != nil {
		return request, err
	}
	if !request.Deadline.After(now) || request.Deadline.After(now.Add(contract.FeatureLifetime)) {
		return request, errors.New("feature deadline must be in the next two minutes")
	}
	return request, nil
}

func featureCatalog(ctx context.Context, root string) error {
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	catalogs, err := client.FeatureCatalog(ctx)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(catalogs)
}

func callFeature(ctx context.Context, root, input string) error {
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	data, err := readFeatureInput(ctx, input)
	if err != nil {
		return err
	}
	request, err := decodeFeatureRequest(data, time.Now())
	if err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	operation, callErr := awaitFeature(ctx, client, request)
	if operation.Request.ID != "" {
		if err := json.NewEncoder(os.Stdout).Encode(operation); err != nil {
			return err
		}
	}
	return callErr
}

func decodeFeatureReceipt(data []byte) (contract.FeatureRequest, error) {
	var metadata struct {
		Schema   string                 `json:"schema"`
		ID       string                 `json:"id"`
		Target   contract.FeatureTarget `json:"target"`
		Root     string                 `json:"root,omitempty"`
		Action   string                 `json:"action"`
		Deadline time.Time              `json:"deadline"`
		Payload  json.RawMessage        `json:"payload,omitempty"`
		Nonce    string                 `json:"nonce,omitempty"`
	}
	if err := decodeFeatureInput(data, &metadata); err != nil {
		return contract.FeatureRequest{}, err
	}
	if metadata.Nonce != "" || len(metadata.Payload) != 0 && !bytes.Equal(bytes.TrimSpace(metadata.Payload), []byte("null")) {
		return contract.FeatureRequest{}, errors.New("feature receipt queries accept routing metadata only")
	}
	request := contract.FeatureRequest{Schema: metadata.Schema, ID: metadata.ID, Target: metadata.Target,
		Root: metadata.Root, Action: metadata.Action, Deadline: metadata.Deadline}
	if request.Reference().Validate() != nil || contract.FeatureAccess(request.Action) == "" || request.Deadline.IsZero() {
		return request, errors.New("invalid feature operation receipt")
	}
	rootRequired := contract.FeatureUsesRoot(request.Action)
	if rootRequired && !contract.ValidID(request.Root) || !rootRequired && request.Root != "" {
		return request, errors.New("feature receipt root does not match its action")
	}
	return request, nil
}

func inspectFeature(ctx context.Context, root, input string, cancelOperation bool) error {
	ctx, stop := context.WithTimeout(ctx, featureCommandTimeout)
	defer stop()
	data, err := readFeatureInput(ctx, input)
	if err != nil {
		return err
	}
	request, err := decodeFeatureReceipt(data)
	if err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	var operation contract.FeatureOperation
	if cancelOperation {
		ctx = context.WithValue(ctx, remoteAuthenticationKey{}, false)
		operation, err = cancelFeatureReceipt(ctx, client, request)
	} else {
		operation, err = client.FeatureStatusReference(ctx, request.Reference(), request.Action)
		if err == nil {
			err = validateFeatureOperation(operation, request)
		}
		if err != nil {
			operation = unknownFeatureReceipt(request, false)
		}
	}
	if encodeErr := json.NewEncoder(os.Stdout).Encode(operation); encodeErr != nil {
		return encodeErr
	}
	if errors.Is(err, remoteio.ErrOperatorLocked) {
		return err
	}
	if err != nil {
		return errors.New("feature receipt query could not be confirmed; no operation was submitted or replayed")
	}
	return nil
}

type featureCancellationClient interface {
	CancelFeatureResult(context.Context, contract.FeatureReference) (contract.FeatureOperation, error)
}

func cancelFeatureReceipt(ctx context.Context, client featureCancellationClient, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	operation, err := client.CancelFeatureResult(ctx, request.Reference())
	if err == nil {
		err = validateFeatureOperation(operation, request)
	}
	if err == nil && !operation.CancelRequested {
		err = errors.New("feature cancellation response did not acknowledge the request")
	}
	if err != nil {
		return unknownFeatureReceipt(request, false), err
	}
	switch operation.Status {
	case "success", "failure", "cancelled":
		operation.Request.Payload = nil
		operation.Request.Nonce = ""
		operation.ReceiptOnly = true
		operation.Result = &contract.FeatureResult{Status: operation.Status,
			Error: "The original operation's terminal status is confirmed. Cancellation does not prove prior native effects are absent; refresh the affected state before new work."}
		return operation, nil
	default:
		unknown := unknownFeatureReceipt(request, true)
		unknown.Result.Error = "Cancellation requested; inspect the captured operation to learn its outcome."
		return unknown, nil
	}
}

type featureClient interface {
	SubmitFeature(context.Context, contract.FeatureRequest) (contract.FeatureOperation, error)
	FeatureStatus(context.Context, contract.FeatureRequest) (contract.FeatureOperation, error)
	CancelFeature(context.Context, contract.FeatureReference) error
}

// Submission is never replayed here. A lost acknowledgement is an unknown
// outcome, even if the subsequent cancellation request is acknowledged.
func awaitFeature(parent context.Context, client featureClient, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	ctx, stop := context.WithDeadline(parent, request.Deadline)
	defer stop()
	operation, err := client.SubmitFeature(ctx, request)
	submitted := err == nil
	for err == nil {
		if err = validateFeatureOperation(operation, request); err != nil {
			break
		}
		switch operation.Status {
		case "success", "failure", "cancelled", "unknown":
			return operation, nil
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
		case <-timer.C:
			operation, err = client.FeatureStatus(ctx, request)
		}
	}
	if !submitted && errors.Is(err, remoteio.ErrOperatorLocked) {
		return contract.FeatureOperation{}, err
	}
	// Detach cleanup from the interrupted operation, preserving values but
	// disabling any interactive authentication in this short cleanup attempt.
	cleanup := context.WithValue(context.WithoutCancel(parent), remoteAuthenticationKey{}, false)
	cleanup, cancel := context.WithTimeout(cleanup, 2*time.Second)
	defer cancel()
	cancelErr := client.CancelFeature(cleanup, request.Reference())
	unknown := unknownFeatureReceipt(request, cancelErr == nil)
	return unknown, errors.New("remote feature outcome is unknown; cancellation was requested where reachable and remaining authority expires with its lease")
}

func unknownFeatureReceipt(request contract.FeatureRequest, cancelRequested bool) contract.FeatureOperation {
	metadata := request
	metadata.Payload = nil
	metadata.Nonce = ""
	return contract.FeatureOperation{
		Request: metadata, Status: "unknown", ReceiptOnly: true,
		CancelRequested: cancelRequested,
		Result:          &contract.FeatureResult{Status: "unknown", Error: "Remote outcome is unknown; inspect this operation before creating another intent."},
	}
}

func validateFeatureOperation(operation contract.FeatureOperation, request contract.FeatureRequest) error {
	if proof := operation.HostRecovery; proof != nil {
		if request.Action != "terminal.host-recover" || operation.Status != "success" || proof.Validate() != nil || proof.Root != request.Root ||
			proof.Reference.Target.Node != request.Target.Node || proof.Reference.Target.Fingerprint != request.Target.Fingerprint {
			return errors.New("host terminal recovery metadata changed identity or root")
		}
	}
	if operation.Reconciled && (!operation.RecoveryRequested || !operation.CancelRequested || !operation.ReceiptOnly ||
		operation.Status != "unknown" || contract.FeatureAccess(request.Action) == "read") {
		return errors.New("feature response has an invalid reconciliation acknowledgment")
	}
	if operation.ReceiptOnly && (len(operation.Request.Payload) != 0 || operation.Request.Nonce != "" || operation.Result != nil && len(operation.Result.Payload) != 0) {
		return errors.New("feature receipt response contains replayable input or result content")
	}
	if operation.Request.Schema != contract.FeatureVersion || operation.Request.ID != request.ID || operation.Request.Target != request.Target {
		return errors.New("feature response changed the captured operation identity")
	}
	if operation.Request.Action != request.Action || operation.Request.Root != request.Root || !operation.Request.Deadline.Equal(request.Deadline) {
		return errors.New("feature response changed the captured operation metadata")
	}
	if !operation.ReceiptOnly && operation.Request.Validate() != nil {
		return errors.New("feature response changed the captured intent")
	}
	switch operation.Status {
	case "queued", "running":
		if operation.Result != nil {
			return errors.New("unfinished feature response contains a result")
		}
	case "success", "failure", "cancelled", "unknown":
		if operation.Result == nil || operation.Result.Status != operation.Status {
			return errors.New("feature response has no matching terminal result")
		}
		return operation.Result.Validate()
	default:
		return errors.New("feature response has an invalid status")
	}
	return nil
}

func configureFeatures(ctx context.Context, root, input string) error {
	return configureWorkerFeatures(ctx, root, input, false)
}

func configureWorkerFeatures(ctx context.Context, root, input string, terminalHome bool) error {
	ctx, cancel := context.WithTimeout(ctx, featureCommandTimeout)
	defer cancel()
	var permissions remoteio.FeaturePermissions
	if input != "" {
		data, err := readFeatureInput(ctx, input)
		if err != nil {
			return err
		}
		if err := decodeFeatureInput(data, &permissions); err != nil {
			return err
		}
		if err := permissions.Validate(); err != nil {
			return err
		}
	}
	var worker contract.WorkerConfig
	workerPath := filepath.Join(root, "worker.json")
	if err := remoteio.ReadPrivate(workerPath, &worker); err != nil {
		return errors.New("configure features on an already paired worker")
	}
	identity := remoteidentity.Identity{PublicKey: worker.MachineKey, PrivateKey: worker.MachinePrivateKey}
	if worker.Schema != contract.Version || !contract.ValidID(worker.Node) || len(worker.Token) != 64 ||
		contract.Endpoint(worker.Endpoint) != nil || identity.Validate() != nil || remoteidentity.Fingerprint(worker.BrokerKey) == "" {
		return errors.New("feature configuration requires valid paired machine keys")
	}
	if input != "" || terminalHome {
		unlock, err := remoteio.LockFeatureConfiguration(root)
		if err != nil {
			return err
		}
		defer unlock()
	}
	if terminalHome {
		var err error
		permissions, err = terminalHomePermissions(root)
		if err != nil {
			return err
		}
	}
	if input == "" && !terminalHome {
		var err error
		permissions, err = remoteio.LoadFeaturePermissions(root)
		if err != nil {
			return err
		}
	} else {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := remoteio.SaveFeaturePermissions(root, permissions); err != nil {
			return err
		}
		worker.Features = true
		if err := remoteio.WritePrivate(workerPath, worker); err != nil {
			return errors.New("feature permissions were saved but worker opt-in could not be persisted; inspect local worker configuration before retrying")
		}
	}
	return json.NewEncoder(os.Stdout).Encode(permissions)
}

func readFeatureInput(ctx context.Context, path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("--request must name a JSON file or - for stdin")
	}
	file := os.Stdin
	if path != "-" {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > contract.MaxFeaturePayload {
			return nil, errors.New("feature request must be a regular file of at most 2 MiB")
		}
		file, err = os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
		if err != nil {
			return nil, errors.New("cannot open feature request file")
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() || opened.Size() > contract.MaxFeaturePayload || !os.SameFile(info, opened) {
			return nil, errors.New("feature request must be a regular file of at most 2 MiB")
		}
	}
	type readResult struct {
		data []byte
		err  error
	}
	// Exactly one reader exists for this CLI invocation. The byte limit bounds
	// allocation; the process deadline also bounds an unattended stdin producer.
	finished := make(chan readResult, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(file, contract.MaxFeaturePayload+1))
		finished <- readResult{data: data, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-finished:
		if result.err != nil {
			return nil, errors.New("cannot read feature request")
		}
		if len(result.data) > contract.MaxFeaturePayload {
			return nil, errors.New("feature request exceeds 2 MiB")
		}
		return result.data, nil
	}
}

func decodeFeatureInput(data []byte, target any) error {
	if len(data) > contract.MaxFeaturePayload || !utf8.Valid(data) {
		return errors.New("feature request must be UTF-8 JSON of at most 2 MiB")
	}
	if err := uniqueFeatureFields(data); err != nil {
		return err
	}
	if err := remoteio.Decode(data, target); err != nil {
		// Decoder diagnostics may contain supplied field names or values.
		return errors.New("feature request contains invalid, unknown or trailing JSON fields")
	}
	return nil
}

func uniqueFeatureFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	type container struct {
		keys        map[string]bool
		keyExpected bool
	}
	var stack []container
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.New("invalid feature request JSON")
		}
		if len(stack) != 0 {
			current := &stack[len(stack)-1]
			if key, ok := token.(string); ok && current.keys != nil && current.keyExpected {
				// encoding/json also matches struct fields without regard to case.
				key = strings.ToLower(key)
				if current.keys[key] {
					return errors.New("duplicate feature request JSON field")
				}
				current.keys[key] = true
				current.keyExpected = false
				continue
			}
			if current.keys != nil {
				current.keyExpected = true
			}
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{', '[':
				// Feature payloads are fixed typed records, never recursive trees.
				if len(stack) >= 64 {
					return errors.New("feature request JSON exceeds 64 container levels")
				}
				next := container{}
				if delimiter == '{' {
					next.keys = map[string]bool{}
					next.keyExpected = true
				}
				stack = append(stack, next)
			case '}', ']':
				if len(stack) == 0 {
					return errors.New("invalid feature request JSON container")
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
}
