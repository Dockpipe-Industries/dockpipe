//go:build linux || darwin

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
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/nativekey"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type fixtureNativeHardware struct {
	devices []nativekey.Device
	calls   int
	pin     []byte
	err     error
}

func (f *fixtureNativeHardware) Devices(context.Context) ([]nativekey.Device, error) {
	return f.devices, nil
}

func (f *fixtureNativeHardware) Create(_ context.Context, _ nativekey.Device, _ []byte, pin []byte) ([]byte, error) {
	f.calls++
	f.pin = pin
	return []byte(`{"signed":"credential"}`), f.err
}

func (f *fixtureNativeHardware) Assert(ctx context.Context, device nativekey.Device, options, pin []byte) ([]byte, error) {
	return f.Create(ctx, device, options, pin)
}

type fixtureHardwareInteraction struct {
	selection int
	pin       string
	err       error
	touches   int
}

func (f *fixtureHardwareInteraction) Choose(context.Context, hardwareCeremonyOptions, []nativekey.Device) (int, []byte, error) {
	return f.selection, []byte(f.pin), f.err
}

func (f *fixtureHardwareInteraction) Touch() error {
	f.touches++
	return nil
}

func fixtureCeremonyOptions() (hardwareCeremonyOptions, error) {
	return hardwareCeremonyOptions{Title: "Approve", Details: "Verified code and identities", Kind: "get", Options: map[string]string{"challenge": "example"}}, nil
}

func TestNativeHardwareFailureNeverRetriesOrPublishes(t *testing.T) {
	client := &fixtureNativeHardware{devices: []nativekey.Device{{ID: "0", Name: "Fixture"}}, err: errors.New("incorrect PIN")}
	interaction := &fixtureHardwareInteraction{pin: "1234"}
	published := false
	err := performHardwareCeremony(context.Background(), fixtureCeremonyOptions, func([]byte) (bool, error) {
		published = true
		return false, nil
	}, client, interaction)
	if err == nil || client.calls != 1 || published || !bytes.Equal(client.pin, make([]byte, 4)) {
		t.Fatal("native failure retried, published approval, or retained the owned PIN buffer")
	}
}

func TestNativeHardwareRejectsInputBeforeKeyOperation(t *testing.T) {
	for _, interaction := range []*fixtureHardwareInteraction{
		{selection: -1, pin: "1234"},
		{selection: 1, pin: "1234"},
		{pin: "123"},
		{pin: strings.Repeat("1", 64)},
		{pin: "1234\x00"},
		{err: context.Canceled},
	} {
		client := &fixtureNativeHardware{devices: []nativekey.Device{{ID: "0", Name: "Fixture"}}}
		err := performHardwareCeremony(context.Background(), fixtureCeremonyOptions, func([]byte) (bool, error) {
			t.Fatal("invalid native input reached approval persistence")
			return false, nil
		}, client, interaction)
		if err == nil || client.calls != 0 || interaction.touches != 0 {
			t.Fatal("invalid native input reached the hardware key")
		}
	}
}

func TestNativeHardwareSpareHasTwoDistinctHumanPhases(t *testing.T) {
	client := &fixtureNativeHardware{devices: []nativekey.Device{{ID: "0", Name: "Fixture"}}}
	interaction := &fixtureHardwareInteraction{pin: "1234"}
	finishes := 0
	err := performHardwareCeremony(context.Background(), fixtureCeremonyOptions, func([]byte) (bool, error) {
		finishes++
		return finishes == 1, nil
	}, client, interaction)
	if err != nil || client.calls != 2 || finishes != 2 || interaction.touches != 2 {
		t.Fatal("spare registration failed to complete its two bounded phases")
	}
}

type hardwareResponseReader struct {
	output *bytes.Buffer
	mutate func(map[string]any)
	reader *bytes.Reader
}

func (r *hardwareResponseReader) Read(buffer []byte) (int, error) {
	if r.reader == nil {
		var event map[string]any
		if err := json.Unmarshal(r.output.Bytes(), &event); err != nil {
			return 0, err
		}
		answer := map[string]any{"id": event["id"], "device": "0", "pin": "private-test-pin", "cancel": false}
		if r.mutate != nil {
			r.mutate(answer)
		}
		raw, _ := json.Marshal(answer)
		r.reader = bytes.NewReader(append(raw, '\n'))
	}
	return r.reader.Read(buffer)
}

func TestNativeHardwareGUIBindsPromptAndKeepsPINOutOfOutput(t *testing.T) {
	var output bytes.Buffer
	interaction := &guiHardwareInteraction{output: &output}
	interaction.input = &hardwareResponseReader{output: &output}
	options, _ := fixtureCeremonyOptions()
	index, pin, err := interaction.Choose(context.Background(), options, []nativekey.Device{{ID: "0", Name: "Fixture"}})
	defer clear(pin)
	if err != nil || index != 0 || string(pin) != "private-test-pin" || bytes.Contains(output.Bytes(), pin) {
		t.Fatal("native response routing failed or leaked its PIN into output")
	}
	if err := interaction.Touch(); err != nil || !bytes.Contains(output.Bytes(), []byte("hardware-key-status")) {
		t.Fatal("native touch status was not delivered")
	}
	for _, mutate := range []func(map[string]any){
		func(answer map[string]any) { answer["id"] = strings.Repeat("0", 32) },
		func(answer map[string]any) { answer["device"] = "unknown" },
		func(answer map[string]any) { answer["cancel"] = true },
		func(answer map[string]any) { answer["unexpected"] = true },
	} {
		output.Reset()
		interaction.input = &hardwareResponseReader{output: &output, mutate: mutate}
		if _, pin, err := interaction.Choose(context.Background(), options, []nativekey.Device{{ID: "0", Name: "Fixture"}}); err == nil {
			clear(pin)
			t.Fatal("unbound or invalid native prompt response accepted")
		}
	}
}

func TestNativeHardwareInputBoundsAndCancellation(t *testing.T) {
	for _, input := range []string{"", "{}", strings.Repeat("x", 8192) + "\n"} {
		if _, err := readHardwareInteraction(context.Background(), strings.NewReader(input)); err == nil {
			t.Fatal("incomplete or oversized native frame accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readHardwareInteraction(ctx, strings.NewReader("{}\n")); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled native prompt accepted input")
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = readHardwareInteraction(ctx, &contextTerminalIO{ctx: ctx, file: read})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("idle native input did not honor cancellation: %v", err)
	}
}

func TestNativeHardwareInteractionFlagRejectsUnrelatedCommands(t *testing.T) {
	for _, arguments := range [][]string{{"nodes", "--interaction", "gui"}, {"approve", "--interaction", "unknown"}} {
		if err := Run(arguments, nil); err == nil {
			t.Fatal("invalid native interaction selection accepted")
		}
	}
	if safeTerminalHardwareText("\x1b[31mname\r\u202e\ncode") != "[31mname\ncode" {
		t.Fatal("untrusted control characters reached terminal consent")
	}
}

var _ io.Reader = (*hardwareResponseReader)(nil)

func TestPairingKeyRegistrationRequiresStoppedLocalBroker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "broker")
	if _, err := initialize(root, "127.0.0.1:47831"); err != nil {
		t.Fatal(err)
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = registerPairingKey(context.Background(), root, "")
	if err == nil || !strings.Contains(err.Error(), "stop the broker") {
		t.Fatalf("registration bypassed live broker lock: %v", err)
	}
	var state remoteio.BrokerState
	if err := remoteio.ReadPrivate(filepath.Join(root, "broker.json"), &state); err != nil {
		t.Fatal(err)
	}
	if state.PairingKeys != nil {
		t.Fatal("rejected registration changed hardware trust")
	}
}
