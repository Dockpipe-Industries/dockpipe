//go:build linux || darwin

package remotecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"dockpipe/src/lib/infrastructure/nativekey"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type terminalHardwareInteraction struct {
	file *os.File
}

func newTerminalHardwareInteraction() (*terminalHardwareInteraction, error) {
	file, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(int(file.Fd())) {
		if file != nil {
			file.Close()
		}
		return nil, errors.New("native hardware-key interaction requires a terminal; use Dockpipe's GUI or run this command in a terminal")
	}
	return &terminalHardwareInteraction{file: file}, nil
}

func (t *terminalHardwareInteraction) Close() error {
	return t.file.Close()
}

func (t *terminalHardwareInteraction) Choose(ctx context.Context, options hardwareCeremonyOptions, devices []nativekey.Device) (int, []byte, error) {
	state, err := term.MakeRaw(int(t.file.Fd()))
	if err != nil {
		return 0, nil, err
	}
	defer term.Restore(int(t.file.Fd()), state)
	input := term.NewTerminal(&contextTerminalIO{ctx: ctx, file: t.file}, "")
	fmt.Fprintf(input, "%s\n%s\n", safeTerminalHardwareText(options.Title), safeTerminalHardwareText(options.Details))
	for index, device := range devices {
		fmt.Fprintf(input, "%d. %s\n", index+1, safeTerminalHardwareText(device.Name))
	}
	input.SetPrompt("Choose key number (or Ctrl+C to cancel): ")
	selection, err := input.ReadLine()
	if err != nil {
		return 0, nil, err
	}
	index, err := strconv.Atoi(strings.TrimSpace(selection))
	if err != nil || index < 1 || index > len(devices) {
		return 0, nil, errors.New("invalid hardware-key selection; no key operation attempted")
	}
	// ReadPassword suppresses echo and does not add the PIN to input history.
	pin, err := input.ReadPassword("Existing FIDO2 security-key PIN (hidden; unchanged): ")
	if err != nil {
		return 0, nil, err
	}
	return index - 1, []byte(pin), nil
}

func (t *terminalHardwareInteraction) Touch() error {
	_, err := fmt.Fprintln(t.file, "Touch your physical security key.")
	return err
}

type contextTerminalIO struct {
	ctx  context.Context
	file *os.File
}

func (c *contextTerminalIO) Read(buffer []byte) (int, error) {
	for {
		if err := c.ctx.Err(); err != nil {
			return 0, err
		}
		poll := []unix.PollFd{{Fd: int32(c.file.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		if poll[0].Revents&unix.POLLIN == 0 {
			return 0, io.EOF
		}
		return unix.Read(int(c.file.Fd()), buffer)
	}
}

func (c *contextTerminalIO) Write(buffer []byte) (int, error) {
	return c.file.Write(buffer)
}

func hardwareGUIInput(ctx context.Context) io.Reader {
	return &contextTerminalIO{ctx: ctx, file: os.Stdin}
}

func safeTerminalHardwareText(value string) string {
	return strings.Map(func(character rune) rune {
		if character != '\n' && (unicode.IsControl(character) || unicode.Is(unicode.Cf, character)) {
			return -1
		}
		return character
	}, value)
}
