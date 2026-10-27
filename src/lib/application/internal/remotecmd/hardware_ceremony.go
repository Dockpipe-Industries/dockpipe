package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure/hardwarekey"
	"dockpipe/src/lib/infrastructure/nativekey"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type hardwareCeremonyOptions struct {
	Title   string
	Details string
	Kind    string
	Options any
}

type hardwareInteractionModeKey struct{}

type nativeHardwareClient interface {
	Devices(context.Context) ([]nativekey.Device, error)
	Create(context.Context, nativekey.Device, []byte, []byte) ([]byte, error)
	Assert(context.Context, nativekey.Device, []byte, []byte) ([]byte, error)
}

type systemHardwareClient struct{}

func (systemHardwareClient) Devices(ctx context.Context) ([]nativekey.Device, error) {
	return nativekey.Devices(ctx)
}

func (systemHardwareClient) Create(ctx context.Context, device nativekey.Device, options, pin []byte) ([]byte, error) {
	return nativekey.Create(ctx, device, options, pin)
}

func (systemHardwareClient) Assert(ctx context.Context, device nativekey.Device, options, pin []byte) ([]byte, error) {
	return nativekey.Assert(ctx, device, options, pin)
}

type hardwareInteraction interface {
	Choose(context.Context, hardwareCeremonyOptions, []nativekey.Device) (int, []byte, error)
	Touch() error
}

func runHardwareCeremony(ctx context.Context, begin func() (hardwareCeremonyOptions, error), finish func([]byte) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, hardwarekey.CeremonyLifetime*2)
	defer cancel()
	var interaction hardwareInteraction
	if ctx.Value(hardwareInteractionModeKey{}) == "gui" {
		interaction = &guiHardwareInteraction{input: hardwareGUIInput(ctx), output: os.Stdout}
	} else {
		terminal, err := newTerminalHardwareInteraction()
		if err != nil {
			return err
		}
		defer terminal.Close()
		interaction = terminal
	}
	return performHardwareCeremony(ctx, begin, finish, systemHardwareClient{}, interaction)
}

func performHardwareCeremony(ctx context.Context, begin func() (hardwareCeremonyOptions, error), finish func([]byte) (bool, error), client nativeHardwareClient, interaction hardwareInteraction) error {
	// Adding a spare has two deliberate phases: verify an existing key, then
	// register the new key. Never retry a PIN or a failed authenticator operation.
	for phase := 0; phase < 2; phase++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		options, err := begin()
		if err != nil {
			return err
		}
		if options.Kind != "create" && options.Kind != "get" {
			return errors.New("unsupported native hardware-key ceremony")
		}
		raw, err := json.Marshal(options.Options)
		if err != nil || len(raw) > hardwarekey.MaxResponseBytes {
			return errors.New("invalid native hardware-key options")
		}
		devices, err := client.Devices(ctx)
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			return errors.New("no accessible FIDO2 security key found; connect your key and check OS device permissions")
		}
		if len(devices) > 8 {
			return errors.New("too many FIDO2 keys connected; connect at most eight keys")
		}
		selected, pin, err := interaction.Choose(ctx, options, devices)
		if err != nil {
			clear(pin)
			return err
		}
		if selected < 0 || selected >= len(devices) || !utf8.Valid(pin) || utf8.RuneCount(pin) < 4 || len(pin) > 63 || bytes.IndexByte(pin, 0) >= 0 {
			clear(pin)
			return errors.New("invalid hardware-key selection or PIN; no key operation attempted")
		}
		if err := interaction.Touch(); err != nil {
			clear(pin)
			return err
		}
		var response []byte
		if options.Kind == "create" {
			response, err = client.Create(ctx, devices[selected], raw, pin)
		} else {
			response, err = client.Assert(ctx, devices[selected], raw, pin)
		}
		clear(pin)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(response) == 0 || len(response) > hardwarekey.MaxResponseBytes {
			return errors.New("native hardware-key response exceeds its bounds")
		}
		next, err := finish(response)
		if err != nil || !next {
			return err
		}
	}
	return errors.New("native hardware-key ceremony exceeded its phase limit")
}

type guiHardwareInteraction struct {
	input  io.Reader
	output io.Writer
}

func (g *guiHardwareInteraction) Choose(ctx context.Context, options hardwareCeremonyOptions, devices []nativekey.Device) (int, []byte, error) {
	secret, err := remoteio.Secret()
	if err != nil {
		return 0, nil, err
	}
	id := secret[:32]
	event := struct {
		Event       string             `json:"event"`
		ID          string             `json:"id"`
		Title       string             `json:"title"`
		Details     string             `json:"details"`
		Devices     []nativekey.Device `json:"devices"`
		PINRequired bool               `json:"pin_required"`
	}{"hardware-key-prompt", id, options.Title, options.Details, devices, true}
	if err := json.NewEncoder(g.output).Encode(event); err != nil {
		return 0, nil, errors.New("cannot present the native hardware-key prompt")
	}
	response, err := readHardwareInteraction(ctx, g.input)
	if err != nil {
		return 0, nil, err
	}
	defer clear(response)
	var answer struct {
		ID     string `json:"id"`
		Device string `json:"device"`
		PIN    string `json:"pin"`
		Cancel bool   `json:"cancel"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&answer); err != nil || answer.ID != id {
		return 0, nil, errors.New("invalid native hardware-key prompt response")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return 0, nil, errors.New("unexpected native hardware-key prompt data")
	}
	if answer.Cancel {
		return 0, nil, context.Canceled
	}
	for index, device := range devices {
		if answer.Device == device.ID {
			return index, []byte(answer.PIN), nil
		}
	}
	return 0, nil, errors.New("native hardware-key response selected an unknown device")
}

func (g *guiHardwareInteraction) Touch() error {
	return json.NewEncoder(g.output).Encode(struct {
		Event   string `json:"event"`
		Message string `json:"message"`
	}{"hardware-key-status", "Touch your physical security key."})
}

func readHardwareInteraction(ctx context.Context, input io.Reader) ([]byte, error) {
	// Read only one frame without buffering a later phase's secret response.
	response := make([]byte, 0, 256)
	for len(response) < 8192 {
		if err := ctx.Err(); err != nil {
			clear(response)
			return nil, err
		}
		var character [1]byte
		n, err := input.Read(character[:])
		if n == 1 {
			if character[0] == '\n' {
				return response, nil
			}
			response = append(response, character[0])
		}
		if err != nil {
			clear(response)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("native hardware-key input closed before a complete response")
		}
	}
	clear(response)
	return nil, errors.New("native hardware-key input exceeds its bounds")
}
