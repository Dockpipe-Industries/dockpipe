//go:build !linux && !darwin

package remotecmd

import (
	"context"
	"errors"
	"io"
	"os"

	"dockpipe/src/lib/infrastructure/nativekey"
)

type terminalHardwareInteraction struct{}

func newTerminalHardwareInteraction() (*terminalHardwareInteraction, error) {
	return nil, errors.New("native terminal hardware-key interaction is not available on this platform")
}

func (*terminalHardwareInteraction) Close() error { return nil }

func (*terminalHardwareInteraction) Choose(context.Context, hardwareCeremonyOptions, []nativekey.Device) (int, []byte, error) {
	return 0, nil, errors.New("native terminal hardware-key interaction is not available on this platform")
}

func (*terminalHardwareInteraction) Touch() error {
	return errors.New("native terminal hardware-key interaction is not available on this platform")
}

func hardwareGUIInput(context.Context) io.Reader { return os.Stdin }
