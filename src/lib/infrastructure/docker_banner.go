package infrastructure

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/term"
)

const banner = `
    ██████╗  ██████╗ ██████╗██╗  ██╗██████╗ ██╗██████╗ ███████╗
    ██╔══██╗██╔═══██╗██╔═══╝██║ ██╔╝██╔══██╗██║██╔══██╗██╔════╝
    ██║  ██║██║   ██║██║    █████╔╝ ██████╔╝██║██████╔╝█████╗
    ██║  ██║██║   ██║██║    ██╔═██╗ ██╔═══╝ ██║██╔═══╝ ██╔══╝
    ██████╔╝╚██████╔╝██████╗██║  ██╗██║     ██║██║     ███████╗
    ╚═════╝  ╚═════╝ ╚═════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝     ╚══════╝
                      Run  →  Isolate  →  Act

`

const compactBanner = "dockpipe — Run -> Isolate -> Act\n"

func terminalWidth(fd int) int {
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

// terminalWidthForBanner prefers a non-zero GetSize from stdout or stderr; on Windows if both
// are zero but we're on a TTY, assume a wide console so the full ASCII banner matches Linux.
func terminalWidthForBanner(outFd, errFd int) int {
	for _, fd := range []int{outFd, errFd} {
		if fd <= 0 {
			continue
		}
		if !isTerminalDockerFn(fd) {
			continue
		}
		if w := terminalWidth(fd); w > 0 {
			return w
		}
	}
	if runtime.GOOS == "windows" {
		return 120
	}
	return 0
}

// PrintLaunchBanner prints the ASCII banner to stderr when stdout or stderr is a TTY.
// The application calls this once at CLI launch (before host work). Spinners for long-running
// work run separately via StartLineSpinner.
func PrintLaunchBanner(stdout, stderr *os.File) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	outFd, outOK := fdInt(stdout)
	errFd, errOK := fdInt(stderr)
	stdoutTTY := outOK && isTerminalDockerFn(outFd)
	stderrTTY := errOK && isTerminalDockerFn(errFd)
	if !stdoutTTY && !stderrTTY {
		return
	}
	width := terminalWidthForBanner(outFd, errFd)
	fmt.Fprint(stderr, renderBannerForWidth(width))
}

func renderBannerForWidth(width int) string {
	// Use a conservative threshold to avoid wrapping/artifacts in split panes.
	const minBannerWidth = 70
	if width < minBannerWidth {
		return compactBanner
	}
	return banner
}

// RenderBannerForTerminal returns the launch/help banner variant for the current terminal width.
// Unlike PrintLaunchBanner, this always returns a string so callers can embed it in other output.
func RenderBannerForTerminal(stdout, stderr *os.File) string {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	outFd, outOK := fdInt(stdout)
	errFd, errOK := fdInt(stderr)
	width := 0
	if outOK || errOK {
		width = terminalWidthForBanner(outFd, errFd)
	}
	return renderBannerForWidth(width)
}

func shouldShowSpinner(width int) bool {
	// Spinner uses carriage-return updates; hide it in narrow terminals to avoid messy wraps.
	return width >= 60
}
