//go:build windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Run attaches the suspended command to a private job before allowing it to
// launch descendants. Closing the job also cleans up children after normal exit.
// https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
func Run(command *exec.Cmd) error {
	if command.Cancel == nil {
		return errors.New("process tree requires a context-bound command")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create process job: %w", err)
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fmt.Errorf("configure process job: %w", err)
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	command.Cancel = func() error {
		if err := windows.TerminateJobObject(job, 1); err != nil {
			return err
		}
		// Cancellation can race assignment to the job; stop the suspended root too.
		return command.Process.Kill()
	}
	if err := command.Start(); err != nil {
		return err
	}
	if err := attachAndResume(job, uint32(command.Process.Pid)); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return fmt.Errorf("contain process tree: %w", err)
	}
	return command.Wait()
}

func attachAndResume(job windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return err
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, resumeErr := windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		return resumeErr
	}
	return errors.New("suspended process thread unavailable")
}

func inheritedTree() bool { return false }
