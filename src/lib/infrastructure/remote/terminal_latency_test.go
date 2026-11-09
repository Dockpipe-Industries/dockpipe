//go:build linux || darwin

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type terminalLatencyTransport struct {
	http.RoundTripper
	mutex      sync.Mutex
	challenges int
	exchanges  int
	roundTrip  time.Duration
}

func (transport *terminalLatencyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mutex.Lock()
	switch request.URL.Path {
	case "/v1/machine-challenge":
		transport.challenges++
	case "/v1/terminal-worker":
		transport.exchanges++
	}
	transport.mutex.Unlock()
	if transport.roundTrip != 0 && !pause(request.Context(), transport.roundTrip/2) {
		return nil, request.Context().Err()
	}
	response, err := transport.RoundTripper.RoundTrip(request)
	if err == nil && transport.roundTrip != 0 && !pause(request.Context(), transport.roundTrip/2) {
		response.Body.Close()
		return nil, request.Context().Err()
	}
	return response, err
}

func TestTerminalInputEchoLatencyMeasurement(t *testing.T) {
	if os.Getenv("DOCKPIPE_TEST_TERMINAL_LATENCY") != "1" {
		t.Skip("explicit bounded latency measurement")
	}
	roundTrip, _ := strconv.Atoi(os.Getenv("DOCKPIPE_TEST_TERMINAL_LATENCY_RTT_MS"))
	if roundTrip < 0 || roundTrip > 100 {
		t.Fatal("fixture RTT must be between 0 and 100 ms")
	}
	t.Setenv("PS1", "")
	t.Setenv("ENV", "")
	t.Setenv("HISTFILE", "/dev/null")
	_, admin, worker, config, root, permissions, _ := featureFixture(t)
	permissions.Roots[0].Terminal = true
	permissions.TerminalProfile = &TerminalProfile{Executable: "/bin/sh", Args: []string{"-i"}}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	transport := &terminalLatencyTransport{RoundTripper: worker.HTTP.Transport, roundTrip: time.Duration(roundTrip) * time.Millisecond}
	worker.HTTP.Transport = transport
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan error, 1)
	instance, _ := Secret()
	go func() { done <- runTerminalWorker(ctx, root, config, worker, instance) }()
	defer func() { cancel(); <-done }()
	var catalog contract.FeatureCatalog
	for ctx.Err() == nil {
		catalogs, err := admin.FeatureCatalog(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalogs) == 1 && catalogs[0].TerminalEpoch != "" && terminalRoot(catalogs[0], "project") {
			catalog = catalogs[0]
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	intent := terminalIntent(t, catalog)
	view, err := admin.OpenTerminal(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	reference := intent.Reference()
	defer admin.CloseTerminal(context.Background(), reference)
	var cursor uint64
	for view.State != "running" && ctx.Err() == nil {
		view, err = admin.PollTerminal(ctx, reference, cursor)
		if err != nil {
			t.Fatal(err)
		}
		cursor = view.Cursor
		time.Sleep(5 * time.Millisecond)
	}
	transport.mutex.Lock()
	transport.challenges, transport.exchanges = 0, 0
	transport.mutex.Unlock()
	var echoMillis, ackMillis []float64
	started := time.Now()
	for index := uint64(1); index <= 12; index++ {
		// Printable bytes without Enter exercise native TTY echo, not shell
		// commands. Unique short batches cannot match an earlier prompt/echo.
		marker := []byte("Q" + strconv.FormatUint(index+100, 10))
		at := time.Now()
		view, err = admin.InputTerminal(ctx, contract.TerminalInput{Reference: reference, Sequence: index, Data: marker})
		if err != nil {
			t.Fatal(err)
		}
		var ack, echo time.Duration
		if view.AckSequence == index {
			ack = time.Since(at)
		}
		var received []byte
		for ack == 0 || echo == 0 {
			view, err = admin.PollTerminal(ctx, reference, cursor)
			if err != nil {
				t.Fatal(err)
			}
			cursor = view.Cursor
			received = append(received, view.Data...)
			if ack == 0 && view.AckSequence == index {
				ack = time.Since(at)
			}
			if echo == 0 && bytes.Contains(received, marker) {
				echo = time.Since(at)
			}
			if ack == 0 || echo == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		ackMillis = append(ackMillis, float64(ack.Microseconds())/1000)
		echoMillis = append(echoMillis, float64(echo.Microseconds())/1000)
	}
	percentile := func(values []float64, percentile int) float64 {
		sorted := append([]float64(nil), values...)
		sort.Float64s(sorted)
		return sorted[(len(sorted)-1)*percentile/100]
	}
	transport.mutex.Lock()
	challenges, exchanges := transport.challenges, transport.exchanges
	transport.mutex.Unlock()
	elapsed := time.Since(started)
	if !pause(ctx, time.Second) {
		t.Fatal(ctx.Err())
	}
	transport.mutex.Lock()
	tailExchanges := transport.exchanges - exchanges
	transport.mutex.Unlock()
	measurement := map[string]any{"samples": len(echoMillis), "simulated_worker_rtt_ms": roundTrip, "elapsed_ms": elapsed.Milliseconds(),
		"echo_p50_ms": percentile(echoMillis, 50), "echo_p95_ms": percentile(echoMillis, 95), "ack_p50_ms": percentile(ackMillis, 50),
		"worker_challenges": challenges, "worker_exchanges": exchanges, "tail_worker_exchanges_1s": tailExchanges, "echo_ms": echoMillis}
	encoded, _ := json.Marshal(measurement)
	t.Log(string(encoded))
}
