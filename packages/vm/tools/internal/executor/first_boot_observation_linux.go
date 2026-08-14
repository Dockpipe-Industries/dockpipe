//go:build linux

package executor

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"dockpipe.vm/tools/internal/provisioning"
)

var errFirstBootObservationOverflow = errors.New("first-boot console exceeded the exact 4 MiB prefix cap")

type observationFile interface {
	io.Writer
	Sync() error
	Close() error
}

type boundedConsoleCapture struct {
	reader   io.ReadCloser
	file     observationFile
	maxBytes int64
	done     chan struct{}
	result   error
	stopping atomic.Bool
}

func startBoundedConsoleCapture(reader io.ReadCloser, file observationFile, maxBytes int64) (*boundedConsoleCapture, error) {
	if reader == nil || file == nil || maxBytes != provisioning.FirstBootObservationMaxBytes {
		return nil, fmt.Errorf("exact bounded first-boot capture inputs are required")
	}
	capture := &boundedConsoleCapture{reader: reader, file: file, maxBytes: maxBytes, done: make(chan struct{})}
	go func() {
		capture.result = capture.copyPrefix()
		close(capture.done)
	}()
	return capture, nil
}

func (c *boundedConsoleCapture) copyPrefix() error {
	buffer := make([]byte, 32*1024)
	written := int64(0)
	for {
		n, err := c.reader.Read(buffer)
		if n > 0 {
			remaining := c.maxBytes - written
			retain := int64(n)
			if retain > remaining {
				retain = remaining
			}
			if retain > 0 {
				if writeErr := writeAll(c.file, buffer[:int(retain)]); writeErr != nil {
					return fmt.Errorf("write first-boot console prefix: %w", writeErr)
				}
				written += retain
			}
			if int64(n) > remaining {
				return errFirstBootObservationOverflow
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || c.stopping.Load() {
				return nil
			}
			return fmt.Errorf("read first-boot console: %w", err)
		}
	}
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

type observationSession struct {
	policy   provisioning.FirstBootObservationPlan
	listener consoleListener
	conn     net.Conn
	file     observationFile
	capture  *boundedConsoleCapture
	syncDir  func(string) error
}

type consoleListener interface {
	Accept() (net.Conn, error)
	Close() error
	SetDeadline(time.Time) error
}

func prepareObservationSession(policy provisioning.FirstBootObservationPlan) (*observationSession, error) {
	return prepareObservationSessionWithListener(policy, func(path string) (consoleListener, error) {
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			return nil, err
		}
		listener.SetUnlinkOnClose(false)
		if err := os.Chmod(path, os.FileMode(policy.SocketMode)); err != nil {
			return nil, errors.Join(err, listener.Close())
		}
		return listener, nil
	})
}

func prepareObservationSessionWithListener(policy provisioning.FirstBootObservationPlan, listen func(string) (consoleListener, error)) (*observationSession, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if listen == nil {
		return nil, fmt.Errorf("controller-owned first-boot listener factory is required")
	}
	if _, err := os.Lstat(policy.EvidencePath); !os.IsNotExist(err) {
		return nil, fmt.Errorf("first-boot evidence path is not fresh")
	}
	if _, err := os.Lstat(policy.SocketPath); !os.IsNotExist(err) {
		return nil, fmt.Errorf("first-boot console socket path is not fresh")
	}
	file, err := os.OpenFile(policy.EvidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(policy.EvidenceMode))
	if err != nil {
		return nil, err
	}
	listener, err := listen(policy.SocketPath)
	if err != nil {
		return nil, errors.Join(err, finishFailedObservationSetup(listener, file, filepath.Dir(policy.EvidencePath), syncDirectory))
	}
	return &observationSession{policy: policy, listener: listener, file: file, syncDir: syncDirectory}, nil
}

func finishFailedObservationSetup(listener consoleListener, file observationFile, evidenceDir string, syncDir func(string) error) error {
	var result error
	if listener != nil {
		result = errors.Join(result, listener.Close())
	}
	if file != nil {
		result = errors.Join(result, file.Sync())
		result = errors.Join(result, file.Close())
	}
	if syncDir != nil {
		result = errors.Join(result, syncDir(evidenceDir))
	}
	return result
}

func (s *observationSession) startCapture() error {
	if s == nil || s.listener != nil || s.conn == nil || s.capture != nil {
		return fmt.Errorf("first-boot console transport is not ready for capture")
	}
	capture, err := startBoundedConsoleCapture(s.conn, s.file, s.policy.MaxBytes)
	if err != nil {
		return err
	}
	s.capture = capture
	return nil
}

func (s *observationSession) stopAndSync() error {
	if s == nil {
		return nil
	}
	var result error
	if s.listener != nil {
		result = errors.Join(result, s.listener.Close())
		s.listener = nil
	}
	if s.capture != nil {
		s.capture.stopping.Store(true)
		result = errors.Join(result, s.capture.reader.Close())
		<-s.capture.done
		result = errors.Join(result, s.capture.result)
		s.capture = nil
	} else if s.conn != nil {
		result = errors.Join(result, s.conn.Close())
	}
	s.conn = nil
	if s.file != nil {
		result = errors.Join(result, s.file.Sync())
		result = errors.Join(result, s.file.Close())
		s.file = nil
	}
	if s.syncDir != nil {
		result = errors.Join(result, s.syncDir(filepath.Dir(s.policy.EvidencePath)))
	}
	return result
}
