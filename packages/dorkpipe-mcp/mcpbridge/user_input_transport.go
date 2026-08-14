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
	"sync"
	"unicode"
	"unicode/utf8"

	"dorkpipe.orchestrator/providersession"
)

const (
	privateUserInputFramePrefix = "DORKPIPE_PRIVATE_USER_INPUT_V1 "
	privateUserInputFrameLimit  = 64 * 1024
	privateInteractiveStdioFlag = "--_private-mcp-interactive-stdio-v1"
)

type providerPoolChatInteractiveStreamingRunner func(context.Context, []string, *transientApprovalController, *transientUserInputController, *transientCancellationController) (stdout, stderr string, exitCode int, err error)

type privateUserInputPromptFrame struct {
	Type   string                          `json:"type"`
	Prompt providersession.UserInputPrompt `json:"prompt"`
}

type privateUserInputResponseFrame struct {
	Type     string                            `json:"type"`
	Response providersession.UserInputResponse `json:"response"`
}

type pendingUserInput struct {
	prompt      providersession.UserInputPrompt
	responses   chan *userInputDelivery
	submitted   bool
	failure     chan struct{}
	failureErr  error
	failureOnce sync.Once
}

func (p *pendingUserInput) fail(err error) {
	if err == nil {
		err = errors.New("pending user-input prompt is no longer available")
	}
	p.failureOnce.Do(func() {
		p.failureErr = err
		close(p.failure)
	})
}

type userInputDelivery struct {
	response   providersession.UserInputResponse
	controller *transientUserInputController
	pending    *pendingUserInput
	ack        chan error
	once       sync.Once
}

func (d *userInputDelivery) complete(err error) {
	d.once.Do(func() {
		if err != nil {
			d.controller.failPending(d.pending, err)
		} else {
			d.controller.completePending(d.pending)
		}
		d.ack <- err
	})
}

type transientUserInputController struct {
	mu       sync.Mutex
	pending  *pendingUserInput
	closed   bool
	closedCh chan struct{}
}

func newTransientUserInputController() *transientUserInputController {
	return &transientUserInputController{closedCh: make(chan struct{})}
}

func cloneUserInputPrompt(prompt providersession.UserInputPrompt) providersession.UserInputPrompt {
	prompt.Options = append([]providersession.UserInputOption(nil), prompt.Options...)
	return prompt
}

func cloneUserInputResponse(response providersession.UserInputResponse) providersession.UserInputResponse {
	response.SelectedOptionRefs = append([]string(nil), response.SelectedOptionRefs...)
	return response
}

func (c *transientUserInputController) pendingPrompt() (providersession.UserInputPrompt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pending == nil || c.pending.submitted {
		return providersession.UserInputPrompt{}, errors.New("no exact provider-pool user-input prompt is pending")
	}
	return cloneUserInputPrompt(c.pending.prompt), nil
}

func (c *transientUserInputController) awaitResponse(ctx context.Context, prompt providersession.UserInputPrompt) (*userInputDelivery, error) {
	prompt = cloneUserInputPrompt(prompt)
	request := providersession.UserInputRequest{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef}
	if err := prompt.ValidateFor(request); err != nil {
		return nil, fmt.Errorf("invalid neutral user-input prompt: %w", err)
	}
	pending := &pendingUserInput{
		prompt:    prompt,
		responses: make(chan *userInputDelivery),
		failure:   make(chan struct{}),
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("provider-pool user-input transport is closed")
	}
	if c.pending != nil {
		c.mu.Unlock()
		return nil, errors.New("a provider-pool user-input prompt is already pending")
	}
	c.pending = pending
	c.mu.Unlock()

	select {
	case delivery := <-pending.responses:
		return delivery, nil
	case <-pending.failure:
		return nil, pending.failureErr
	case <-c.closedCh:
		return nil, errors.New("provider-pool user-input transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return nil, ctx.Err()
	}
}

func (c *transientUserInputController) submit(ctx context.Context, response providersession.UserInputResponse) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	response = cloneUserInputResponse(response)
	c.mu.Lock()
	pending := c.pending
	if c.closed || pending == nil || pending.submitted {
		c.mu.Unlock()
		return errors.New("no exact provider-pool user-input prompt is pending")
	}
	prompt := cloneUserInputPrompt(pending.prompt)
	if err := validateTransientUserInputResponse(response, prompt); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("user-input response rejected: %w", err)
	}
	pending.submitted = true
	c.mu.Unlock()

	delivery := &userInputDelivery{response: response, controller: c, pending: pending, ack: make(chan error, 1)}
	select {
	case pending.responses <- delivery:
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool user-input transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return ctx.Err()
	}
	select {
	case err := <-delivery.ack:
		return err
	case <-pending.failure:
		return pending.failureErr
	case <-c.closedCh:
		return errors.New("provider-pool user-input transport is closed")
	case <-ctx.Done():
		c.failPending(pending, ctx.Err())
		return ctx.Err()
	}
}

func (c *transientUserInputController) completePending(pending *pendingUserInput) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
	}
	c.mu.Unlock()
}

func (c *transientUserInputController) failPending(pending *pendingUserInput, err error) {
	c.mu.Lock()
	if c.pending == pending {
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientUserInputController) invalidate(err error) {
	c.mu.Lock()
	if c.pending != nil {
		pending := c.pending
		c.pending = nil
		pending.fail(err)
	}
	c.mu.Unlock()
}

func (c *transientUserInputController) close(err error) {
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

func validateTransientUserInputResponse(response providersession.UserInputResponse, prompt providersession.UserInputPrompt) error {
	if err := response.ValidateFor(prompt); err != nil {
		return err
	}
	if !utf8.ValidString(response.Text) {
		return errors.New("user-input text must be valid UTF-8")
	}
	for _, character := range response.Text {
		if unicode.IsControl(character) {
			return errors.New("user-input text must not contain control characters")
		}
	}
	return nil
}

func runDorkpipeWithInteractiveTransport(ctx context.Context, args []string, approvals *transientApprovalController, inputs *transientUserInputController, cancellations *transientCancellationController) (stdout, stderr string, exitCode int, err error) {
	if approvals == nil || inputs == nil || cancellations == nil {
		return "", "", -1, errors.New("provider-pool interactive controllers are required")
	}
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmdArgs := append(append([]string(nil), args...), privateInteractiveStdioFlag)
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
		text, relayErr := relayInteractiveFrames(runCtx, stderrPipe, stdin, approvals, inputs, cancellations)
		relayDone <- relayResult{stderr: text, err: relayErr}
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	var waitErr error
	var relay relayResult
	select {
	case waitErr = <-waitDone:
		exitErr := errors.New("provider-pool child exited while interactive input was pending")
		approvals.invalidate(exitErr)
		inputs.invalidate(exitErr)
		cancellations.invalidate(exitErr)
		relay = <-relayDone
	case relay = <-relayDone:
		if relay.err != nil {
			approvals.invalidate(relay.err)
			inputs.invalidate(relay.err)
			cancellations.invalidate(relay.err)
			cancel()
			_ = stdin.Close()
		}
		waitErr = <-waitDone
	case <-ctx.Done():
		cancel()
		approvals.invalidate(ctx.Err())
		inputs.invalidate(ctx.Err())
		cancellations.invalidate(ctx.Err())
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

type serializedPrivateInteractiveWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *serializedPrivateInteractiveWriter) write(prefix string, limit int, encoded []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writePrivateInteractiveFrame(w.writer, prefix, limit, encoded)
}

type cancellationRelayWorker struct {
	done chan struct{}
	err  error
}

func relayInteractiveFrames(ctx context.Context, fromChild io.Reader, toChild io.Writer, approvals *transientApprovalController, inputs *transientUserInputController, cancellations *transientCancellationController) (string, error) {
	if approvals == nil || inputs == nil || cancellations == nil {
		return "", errors.New("provider-pool interactive controllers are required")
	}
	relayCtx, cancelRelay := context.WithCancel(ctx)
	defer cancelRelay()
	writer := &serializedPrivateInteractiveWriter{writer: toChild}
	scanner := bufio.NewScanner(fromChild)
	scanner.Buffer(make([]byte, 4096), privateUserInputFrameLimit)
	var ordinary bytes.Buffer
	var cancellationWorker *cancellationRelayWorker
	defer func() {
		cancelRelay()
		if cancellationWorker != nil {
			<-cancellationWorker.done
		}
	}()
	cancellationSeen := false
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		switch {
		case bytes.HasPrefix(line, []byte(privateApprovalFramePrefix)):
			if err := relayOneApprovalFrame(relayCtx, line[len(privateApprovalFramePrefix):], writer, approvals); err != nil {
				return ordinary.String(), err
			}
		case bytes.HasPrefix(line, []byte(privateUserInputFramePrefix)):
			if err := relayOneUserInputFrame(relayCtx, line[len(privateUserInputFramePrefix):], writer, inputs); err != nil {
				return ordinary.String(), err
			}
		case bytes.HasPrefix(line, []byte(privateCancellationFramePrefix)):
			if cancellationSeen {
				return ordinary.String(), errors.New("duplicate private cancellation scope frame")
			}
			worker, err := startCancellationRelay(relayCtx, cancelRelay, fromChild, line[len(privateCancellationFramePrefix):], writer, cancellations)
			if err != nil {
				return ordinary.String(), err
			}
			cancellationSeen = true
			cancellationWorker = worker
		default:
			ordinary.Write(line)
			ordinary.WriteByte('\n')
		}
	}
	cancelRelay()
	var cancellationErr error
	if cancellationWorker != nil {
		<-cancellationWorker.done
		cancellationErr = cancellationWorker.err
	}
	if err := scanner.Err(); err != nil {
		return ordinary.String(), fmt.Errorf("read private interactive frame: %w", err)
	}
	if cancellationErr != nil && !errors.Is(cancellationErr, context.Canceled) {
		return ordinary.String(), cancellationErr
	}
	return ordinary.String(), nil
}

func startCancellationRelay(ctx context.Context, cancel context.CancelFunc, fromChild io.Reader, raw []byte, writer *serializedPrivateInteractiveWriter, cancellations *transientCancellationController) (*cancellationRelayWorker, error) {
	var frame privateCancellationScopeFrame
	if !utf8.Valid(raw) || decodeClosedJSON(raw, &frame) != nil || frame.Type != "cancellation_scope" || validateCancellationScope(frame.Scope) != nil {
		return nil, errors.New("invalid private cancellation scope frame")
	}
	pending, err := cancellations.registerScope(frame.Scope)
	if err != nil {
		return nil, err
	}
	worker := &cancellationRelayWorker{done: make(chan struct{})}
	go func() {
		defer close(worker.done)
		delivery, waitErr := cancellations.awaitRegisteredIntent(ctx, pending)
		if waitErr == nil {
			encoded, encodeErr := json.Marshal(privateCancellationIntentFrame{Type: "cancellation_intent", Intent: cloneCancellationIntent(delivery.intent)})
			waitErr = encodeErr
			if waitErr == nil {
				waitErr = writer.write(privateCancellationFramePrefix, privateCancellationFrameLimit, encoded)
			}
			delivery.complete(waitErr)
		}
		if waitErr != nil {
			worker.err = fmt.Errorf("private cancellation relay failed: %w", waitErr)
			cancel()
			if closer, ok := fromChild.(io.Closer); ok {
				_ = closer.Close()
			}
		}
	}()
	return worker, nil
}

func relayOneApprovalFrame(ctx context.Context, raw []byte, writer *serializedPrivateInteractiveWriter, approvals *transientApprovalController) error {
	var frame privateApprovalRequestFrame
	if !utf8.Valid(raw) || decodeClosedJSON(raw, &frame) != nil || frame.Type != "approval_request" || frame.Request.Validate() != nil {
		return errors.New("invalid private approval request frame")
	}
	delivery, err := approvals.awaitDecision(ctx, frame.Request)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(privateApprovalDecisionFrame{Type: "approval_decision", Decision: delivery.decision})
	if err == nil {
		err = writer.write(privateApprovalFramePrefix, privateApprovalFrameLimit, encoded)
	}
	delivery.complete(err)
	if err != nil {
		return fmt.Errorf("write private approval decision frame: %w", err)
	}
	return nil
}

func relayOneUserInputFrame(ctx context.Context, raw []byte, writer *serializedPrivateInteractiveWriter, inputs *transientUserInputController) error {
	var frame privateUserInputPromptFrame
	if !utf8.Valid(raw) || decodeClosedJSON(raw, &frame) != nil || frame.Type != "user_input_prompt" {
		return errors.New("invalid private user-input prompt frame")
	}
	frame.Prompt = cloneUserInputPrompt(frame.Prompt)
	request := providersession.UserInputRequest{Correlation: frame.Prompt.Correlation, PromptRef: frame.Prompt.PromptRef}
	if frame.Prompt.ValidateFor(request) != nil {
		return errors.New("invalid private user-input prompt frame")
	}
	delivery, err := inputs.awaitResponse(ctx, frame.Prompt)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(privateUserInputResponseFrame{Type: "user_input_response", Response: delivery.response})
	if err == nil {
		err = writer.write(privateUserInputFramePrefix, privateUserInputFrameLimit, encoded)
	}
	delivery.complete(err)
	if err != nil {
		return fmt.Errorf("write private user-input response frame: %w", err)
	}
	return nil
}

func writePrivateInteractiveFrame(writer io.Writer, prefix string, limit int, encoded []byte) error {
	line := append([]byte(prefix), encoded...)
	line = append(line, '\n')
	if len(line) > limit {
		return errors.New("private interactive frame exceeds its bound")
	}
	written, err := writer.Write(line)
	if err != nil {
		return err
	}
	if written != len(line) {
		return io.ErrShortWrite
	}
	return nil
}
