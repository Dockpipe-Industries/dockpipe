package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"dorkpipe.orchestrator/providersession"
)

const (
	privateApprovalFramePrefix = "DORKPIPE_PRIVATE_APPROVAL_V1 "
	privateApprovalFrameLimit  = 64 * 1024
	privateApprovalStdioFlag   = "--_private-mcp-approval-stdio-v1"
)

type providerPoolChatStreamingRunner func(context.Context, []string, *transientApprovalController) (stdout, stderr string, exitCode int, err error)

type privateApprovalRequestFrame struct {
	Type    string                          `json:"type"`
	Request providersession.ApprovalRequest `json:"request"`
}

type privateApprovalDecisionFrame struct {
	Type     string                           `json:"type"`
	Decision providersession.ApprovalDecision `json:"decision"`
}

type pendingApproval struct {
	request     providersession.ApprovalRequest
	decisions   chan *approvalDelivery
	submitted   bool
	failure     chan struct{}
	failureErr  error
	failureOnce sync.Once
}

func (p *pendingApproval) fail(err error) {
	if err == nil {
		err = errors.New("pending approval is no longer available")
	}
	p.failureOnce.Do(func() {
		p.failureErr = err
		close(p.failure)
	})
}

type approvalDelivery struct {
	decision   providersession.ApprovalDecision
	controller *transientApprovalController
	pending    *pendingApproval
	ack        chan error
	once       sync.Once
}

func (d *approvalDelivery) complete(err error) {
	d.once.Do(func() {
		if err != nil {
			d.controller.failPending(d.pending, err)
		} else {
			d.controller.completePending(d.pending)
		}
		d.ack <- err
	})
}

type transientApprovalController struct {
	mu       sync.Mutex
	pending  *pendingApproval
	closed   bool
	closedCh chan struct{}
}

func newTransientApprovalController() *transientApprovalController {
	return &transientApprovalController{closedCh: make(chan struct{})}
}

func cloneApprovalRequest(request providersession.ApprovalRequest) providersession.ApprovalRequest {
	request.AllowedDecisions = append([]string(nil), request.AllowedDecisions...)
	return request
}

func (c *transientApprovalController) pendingRequest() (providersession.ApprovalRequest, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pending == nil || c.pending.submitted {
		return providersession.ApprovalRequest{}, errors.New("no exact provider-pool approval request is pending")
	}
	return cloneApprovalRequest(c.pending.request), nil
}

func (c *transientApprovalController) awaitDecision(ctx context.Context, request providersession.ApprovalRequest) (*approvalDelivery, error) {
	request = cloneApprovalRequest(request)
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid neutral approval request: %w", err)
	}
	pending := &pendingApproval{
		request:   request,
		decisions: make(chan *approvalDelivery),
		failure:   make(chan struct{}),
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("provider-pool approval transport is closed")
	}
	if c.pending != nil {
		c.mu.Unlock()
		return nil, errors.New("a provider-pool approval request is already pending")
	}
	c.pending = pending
	c.mu.Unlock()

	select {
	case delivery := <-pending.decisions:
		return delivery, nil
	case <-pending.failure:
		return nil, pending.failureErr
	case <-c.closedCh:
		return nil, errors.New("provider-pool approval transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return nil, ctx.Err()
	}
}

func (c *transientApprovalController) submit(ctx context.Context, decision providersession.ApprovalDecision) error {
	c.mu.Lock()
	pending := c.pending
	if c.closed || pending == nil || pending.submitted {
		c.mu.Unlock()
		return errors.New("no exact provider-pool approval request is pending")
	}
	request := cloneApprovalRequest(pending.request)
	if err := decision.ValidateFor(request); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("approval decision rejected: %w", err)
	}
	pending.submitted = true
	c.mu.Unlock()

	delivery := &approvalDelivery{decision: decision, controller: c, pending: pending, ack: make(chan error, 1)}
	select {
	case pending.decisions <- delivery:
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool approval transport is closed")
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-delivery.ack:
		return err
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool approval transport is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *transientApprovalController) completePending(pending *pendingApproval) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
	}
	c.mu.Unlock()
}

func (c *transientApprovalController) failPending(pending *pendingApproval, err error) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientApprovalController) invalidate(err error) {
	c.mu.Lock()
	if c.pending != nil {
		pending := c.pending
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientApprovalController) close(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if c.pending != nil {
		pending := c.pending
		c.pending = nil
		pending.fail(err)
	}
	close(c.closedCh)
	c.mu.Unlock()
}

func runDorkpipeWithApprovalTransport(ctx context.Context, args []string, approvals *transientApprovalController) (stdout, stderr string, exitCode int, err error) {
	if approvals == nil {
		return "", "", -1, errors.New("provider-pool approval controller is required")
	}
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmdArgs := append(append([]string(nil), args...), privateApprovalStdioFlag)
	cmd := exec.CommandContext(runCtx, dorkpipePath(), cmdArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", -1, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, err
	}
	var stdoutBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	if err := cmd.Start(); err != nil {
		return "", "", -1, err
	}

	type relayResult struct {
		stderr string
		err    error
	}
	relayDone := make(chan relayResult, 1)
	go func() {
		text, relayErr := relayApprovalFrames(runCtx, stderrPipe, stdin, approvals)
		relayDone <- relayResult{stderr: text, err: relayErr}
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	var waitErr error
	var relay relayResult
	select {
	case waitErr = <-waitDone:
		approvals.invalidate(errors.New("provider-pool child exited while approval was pending"))
		relay = <-relayDone
	case relay = <-relayDone:
		if relay.err != nil {
			cancel()
			_ = stdin.Close()
		}
		waitErr = <-waitDone
	case <-ctx.Done():
		cancel()
		approvals.invalidate(ctx.Err())
		_ = stdin.Close()
		waitErr = <-waitDone
		relay = <-relayDone
	}
	_ = stdin.Close()
	stdout = stdoutBuffer.String()
	stderr = relay.stderr
	if relay.err != nil && ctx.Err() == nil {
		return stdout, stderr, -1, relay.err
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		if ctx.Err() != nil {
			return stdout, stderr, -1, ctx.Err()
		}
		return stdout, stderr, -1, waitErr
	}
	return stdout, stderr, 0, nil
}

func relayApprovalFrames(ctx context.Context, fromChild io.Reader, toChild io.Writer, approvals *transientApprovalController) (string, error) {
	scanner := bufio.NewScanner(fromChild)
	scanner.Buffer(make([]byte, 4096), privateApprovalFrameLimit)
	var ordinary strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, privateApprovalFramePrefix) {
			ordinary.WriteString(line)
			ordinary.WriteByte('\n')
			continue
		}
		var frame privateApprovalRequestFrame
		if err := decodeClosedJSON([]byte(strings.TrimPrefix(line, privateApprovalFramePrefix)), &frame); err != nil {
			return ordinary.String(), fmt.Errorf("invalid private approval request frame: %w", err)
		}
		if frame.Type != "approval_request" || frame.Request.Validate() != nil {
			return ordinary.String(), errors.New("invalid private approval request frame")
		}
		delivery, err := approvals.awaitDecision(ctx, frame.Request)
		if err != nil {
			return ordinary.String(), err
		}
		decisionFrame := privateApprovalDecisionFrame{Type: "approval_decision", Decision: delivery.decision}
		encoded, err := json.Marshal(decisionFrame)
		if err == nil && len(encoded)+len(privateApprovalFramePrefix)+1 > privateApprovalFrameLimit {
			err = errors.New("private approval decision frame exceeds its bound")
		}
		if err == nil {
			line := append([]byte(privateApprovalFramePrefix), encoded...)
			line = append(line, '\n')
			var written int
			written, err = toChild.Write(line)
			if err == nil && written != len(line) {
				err = io.ErrShortWrite
			}
		}
		delivery.complete(err)
		if err != nil {
			return ordinary.String(), fmt.Errorf("write private approval decision frame: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return ordinary.String(), fmt.Errorf("read private approval request frame: %w", err)
	}
	return ordinary.String(), nil
}

func decodeClosedJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}
