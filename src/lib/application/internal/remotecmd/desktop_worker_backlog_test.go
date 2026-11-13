package remotecmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// Model bytes already accepted by a native pipe, including the final verified
// closed report. Reading one frame must not require another native write.
type desktopBacklogInput struct {
	queued chan []byte
	closed chan struct{}
	once   sync.Once
	data   []byte
}

func (input *desktopBacklogInput) Read(data []byte) (int, error) {
	if len(input.data) == 0 {
		select {
		case input.data = <-input.queued:
		case <-input.closed:
			return 0, io.EOF
		}
	}
	count := copy(data, input.data)
	input.data = input.data[count:]
	return count, nil
}

func (input *desktopBacklogInput) Close() error {
	input.once.Do(func() { close(input.closed) })
	return nil
}

type desktopBacklogOutput struct {
	desktopBuffer
	startOnce sync.Once
	input     *desktopBacklogInput
	request   contract.DesktopOpen
	ordinary  int
}

func (output *desktopBacklogOutput) Write(data []byte) (int, error) {
	var event desktopEvent
	if json.Unmarshal(data, &event) == nil && event.Type == "directive" {
		output.startOnce.Do(func() {
			var frames bytes.Buffer
			encoder := json.NewEncoder(&frames)
			for count := 0; count < output.ordinary; count++ {
				_ = encoder.Encode(desktopFrame{Type: "report", Report: &contract.DesktopReport{
					Reference: output.request.Reference(), State: "running", Ack: 1}})
			}
			_ = encoder.Encode(desktopFrame{Type: "report", Report: &contract.DesktopReport{
				Reference: output.request.Reference(), State: "closed", Ack: 1}})
			output.input.queued <- frames.Bytes()
		})
	}
	if event.Type == "forget" {
		_ = output.input.Close()
	}
	return output.desktopBuffer.Write(data)
}

type desktopBacklogBroker struct {
	t               *testing.T
	request         contract.DesktopOpen
	calls           int
	closing         bool
	postCloseCalls  int
	closedPublished int
}

func (broker *desktopBacklogBroker) ExchangeDesktop(_ context.Context, exchange contract.DesktopExchange) (contract.DesktopExchangeResult, error) {
	broker.calls++
	response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: broker.request.Epoch}
	if broker.calls == 1 {
		return response, nil
	}
	if broker.closing {
		broker.postCloseCalls++
		if broker.postCloseCalls > 2 {
			return response, errors.New("native Forget budget exhausted behind repeated ordinary reports")
		}
		if len(exchange.Reports) == 1 && exchange.Reports[0].State == "closed" {
			if exchange.Reports[0].Reference != broker.request.Reference() || exchange.Reports[0].Ack != 1 || len(exchange.Reports[0].Signals) != 0 {
				broker.t.Error("queued native closure changed exact identity/ACK or forwarded signaling")
			}
			broker.closedPublished++
			response.Forget = []contract.DesktopReference{broker.request.Reference()}
			return response, nil
		}
	}
	directive := contract.DesktopDirective{Open: broker.request, Operator: strings.Repeat("a", 64), LeaseMillis: 5000}
	if broker.calls >= 3 {
		broker.closing = true
		directive.Close, directive.LeaseMillis = true, 0
	}
	response.Directives = []contract.DesktopDirective{directive}
	return response, nil
}

func TestDesktopWorkerNormalCloseDoesNotStarveQueuedNativeProof(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		ordinary int
	}{{"unchanged-report-backlog", 32}, {"deduplicated-native-reports", 1}} {
		t.Run(scenario.name, func(t *testing.T) {
			request := desktopTestIntent()
			permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: request.Provider, View: true, Input: true}
			root := privateConnectionDirectory(t)
			if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
			input := &desktopBacklogInput{queued: make(chan []byte, 1), closed: make(chan struct{})}
			defer input.Close()
			output := desktopBacklogOutput{input: input, request: request, ordinary: scenario.ordinary}
			broker := desktopBacklogBroker{t: t, request: request}
			catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Target: request.Target, Provider: request.Provider, View: true, Input: true}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := exchangeDesktopWorker(ctx, root, permissions, &broker, catalog, bufio.NewScanner(input), input, &output,
				remoteio.NewDesktopSessionFence(desktopUnlockedFixture{}))
			if err != nil && err != io.EOF || broker.closedPublished != 1 || !strings.Contains(output.String(), `"type":"forget"`) {
				t.Fatalf("native closure was stranded behind %d ordinary reports: error=%v, accepted=%d, post-close exchanges=%d",
					scenario.ordinary, err, broker.closedPublished, broker.postCloseCalls)
			}
		})
	}
}
