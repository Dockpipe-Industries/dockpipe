//go:build windows

package localterminal

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureProviderProcess(command *exec.Cmd) {
	// The QtCore provider uses private pipes and owns a separate ConPTY. It must
	// not allocate a visible console when started from the desktop application.
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
