//go:build windows

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Run attaches the suspended command to a private job before allowing it to
// launch descendants. Closing the job also cleans up children after normal exit.
// https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
func run(command *exec.Cmd, started func()) error {
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
	started()
	if err := attachAndResume(job, uint32(command.Process.Pid)); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return fmt.Errorf("contain process tree: %w", err)
	}
	return command.Wait()
}

// RunHostObserved preserves host scripts which launch a redirected GUI/helper
// and return early. The job remains armed through root wait and completion/EOF;
// only an uncancelled completion releases surviving descendants. RunObserved's
// bounded-operation policy above deliberately remains unchanged.
func RunHostObserved(ctx context.Context, command *exec.Cmd, observer HostStartedObserver) (outcome error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var finish func(error) error
	finished := false
	complete := func(runErr error) error {
		finished = true
		if finish == nil && observer != nil {
			finish = observer(command)
		}
		if finish != nil {
			return finish(runErr)
		}
		return nil
	}
	defer func() {
		if !finished {
			outcome = errors.Join(outcome, complete(outcome))
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if command.Cancel == nil {
		return errors.New("host process requires a context-bound command")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create host process job: %w", err)
	}
	custody := &hostProcessCustody{job: job, ctx: ctx}
	defer func() { outcome = errors.Join(outcome, custody.close()) }()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fmt.Errorf("configure host process job: %w", err)
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	command.Cancel = custody.cancel

	// exec stops its own context watcher after root Wait. This owner remains
	// cancellable while a descendant still holds an output writer open.
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			_ = custody.cancel()
		case <-stop:
		}
	}()
	defer func() {
		close(stop)
		<-joined
	}()

	if err := command.Start(); err != nil {
		return err
	}
	custody.setRoot(command.Process)
	if observer != nil {
		finish = observer(command)
	}
	if err := custody.attachAndResume(); err != nil {
		_ = custody.cancel()
		waitErr := command.Wait()
		return errors.Join(fmt.Errorf("contain host process tree: %w", err), waitErr)
	}

	runErr := command.Wait()
	completionErr := complete(runErr)
	if completionErr != nil {
		return errors.Join(runErr, completionErr)
	}
	var exitError *exec.ExitError
	if runErr != nil && (!errors.As(runErr, &exitError) || command.ProcessState == nil || !command.ProcessState.Exited()) {
		return runErr
	}
	if err := custody.release(); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

// The mutex serializes context cancellation, suspended-root registration,
// assignment/resume and the normal release commit. A cancelled/closed handle
// is never reused in a later job operation.
type hostProcessCustody struct {
	mutex     sync.Mutex
	job       windows.Handle
	ctx       context.Context
	root      *os.Process
	cancelled bool
	released  bool
	closed    bool
	cancelErr error
}

func (custody *hostProcessCustody) setRoot(root *os.Process) {
	custody.mutex.Lock()
	defer custody.mutex.Unlock()
	custody.root = root
	if custody.cancelled {
		// Cancellation may have closed the empty job before Start returned.
		// The actual suspended root is still our direct child and must retire.
		custody.cancelErr = errors.Join(custody.cancelErr, ignoreFinishedHostProcess(root.Kill()))
	}
}

func (custody *hostProcessCustody) attachAndResume() error {
	custody.mutex.Lock()
	defer custody.mutex.Unlock()
	if custody.cancelled || custody.closed {
		return errors.Join(errors.New("host process custody cancelled"), custody.ctx.Err(), custody.cancelErr)
	}
	return attachAndResume(custody.job, uint32(custody.root.Pid))
}

func ignoreFinishedHostProcess(err error) error {
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func (custody *hostProcessCustody) cancel() error {
	custody.mutex.Lock()
	defer custody.mutex.Unlock()
	if custody.released {
		return os.ErrProcessDone
	}
	custody.cancelled = true
	if !custody.closed {
		jobErr := windows.TerminateJobObject(custody.job, 1)
		// Closing the armed job is also the fallback if termination failed;
		// output completion cannot wait forever on its inherited writer handles.
		closeErr := windows.CloseHandle(custody.job)
		custody.closed = true
		custody.cancelErr = errors.Join(custody.cancelErr, jobErr, closeErr)
	}
	if custody.root != nil {
		custody.cancelErr = errors.Join(custody.cancelErr, ignoreFinishedHostProcess(custody.root.Kill()))
	}
	return custody.cancelErr
}

func (custody *hostProcessCustody) release() error {
	custody.mutex.Lock()
	defer custody.mutex.Unlock()
	if custody.cancelled || custody.closed || custody.ctx.Err() != nil {
		return errors.Join(errors.New("host process cancelled before release"), custody.ctx.Err(), custody.cancelErr)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	if _, err := windows.SetInformationJobObject(custody.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fmt.Errorf("release completed host job: %w", err)
	}
	// This is the normal completion commit. A later cancellation request has
	// no authority over children deliberately released from this host call.
	custody.released = true
	return nil
}

func (custody *hostProcessCustody) close() error {
	custody.mutex.Lock()
	defer custody.mutex.Unlock()
	var closeErr error
	if !custody.closed {
		closeErr = windows.CloseHandle(custody.job)
		custody.closed = true
	}
	return errors.Join(custody.cancelErr, closeErr)
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
