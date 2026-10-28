package remote

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

const securityIPCBytes = 8192

// OperatorAuthority exposes the already configured local authority to the
// owning composition root. Its methods still enforce native authentication.
func (broker *Broker) OperatorAuthority() *SecurityAuthority {
	return broker.security
}

type SecurityRequest struct {
	Action   string                         `json:"action"`
	Audience string                         `json:"audience,omitempty"`
	Path     string                         `json:"path,omitempty"`
	Digest   string                         `json:"digest,omitempty"`
	Policy   *contract.RemoteSecurityPolicy `json:"policy,omitempty"`
}

type SecurityResponse struct {
	ErrorCode string          `json:"error_code,omitempty"`
	Status    *SecurityStatus `json:"status,omitempty"`
	Proof     *SecurityProof  `json:"proof,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// StartOperatorSecurity uses one canonical OS-user socket for every CLI/UI
// client. A second local broker fails closed rather than creating another lock
// owner. The connection root and environment cannot select a weaker authority.
func (broker *Broker) StartOperatorSecurity(ctx context.Context, platform osauth.Platform) (func(), error) {
	directory, err := platform.SecurityDirectory()
	if err != nil {
		return nil, err
	}
	return broker.startOperatorSecurityAt(ctx, platform, directory)
}

func (broker *Broker) startOperatorSecurityAt(ctx context.Context, platform osauth.Platform, directory string) (func(), error) {
	authority, err := NewSecurityAuthority(platform, directory, broker.state.Identity.PublicKey, os.Getuid())
	if err != nil {
		return nil, err
	}
	unlock, err := Lock(filepath.Join(directory, "authority.lock"))
	if err != nil {
		return nil, errors.New("another local broker owns the global remote security authority; secondary brokers stay unavailable")
	}
	path := filepath.Join(directory, "authority.sock")
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			unlock()
			return nil, errors.New("remote security IPC path is not a socket; preserve it and inspect")
		}
		if err := os.Remove(path); err != nil {
			unlock()
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		unlock()
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		unlock()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		// Permission setup failed; closing the unused listener is best-effort.
		_ = listener.Close()
		unlock()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	broker.security = authority
	authority.Sample(ctx)
	var connections sync.Map
	slots := make(chan struct{}, 8)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				authority.Sample(ctx)
			}
		}
	}()
	go func() {
		defer workers.Done()
		for {
			connection, err := listener.AcceptUnix()
			if err != nil {
				return
			}
			select {
			case slots <- struct{}{}:
				connections.Store(connection, true)
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-slots }()
					defer connections.Delete(connection)
					defer connection.Close()
					authority.serveSecurityConnection(ctx, connection)
				}()
			default:
				// Admission was refused, so this connection has no result to retain.
				_ = connection.Close()
			}
		}
	}()
	var closeOnce sync.Once
	return func() {
		closeOnce.Do(func() {
			cancel()
			authority.Lock()
			// Shutdown is idempotent; already-closed descriptors need no retry.
			_ = listener.Close()
			connections.Range(func(key, _ any) bool {
				_ = key.(*net.UnixConn).Close()
				return true
			})
			workers.Wait()
			unlock()
		})
	}, nil
}

func (authority *SecurityAuthority) serveSecurityConnection(parent context.Context, connection *net.UnixConn) {
	stopClose := context.AfterFunc(parent, func() {
		// Cancellation discards the exchange; closing is best-effort cleanup.
		_ = connection.Close()
	})
	defer stopClose()
	uid, pid, err := securityPeer(connection)
	if err != nil || uid != os.Getuid() || pid <= 0 {
		return
	}
	if err := connection.SetDeadline(time.Now().Add(125 * time.Second)); err != nil {
		return
	}
	reader := bufio.NewReaderSize(connection, securityIPCBytes)
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) > securityIPCBytes {
		return
	}
	var request SecurityRequest
	if err := Decode(line, &request); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 120*time.Second)
	defer cancel()
	// A closed client or extra request bytes cancel an in-progress OS dialog.
	go func() { _, _ = reader.ReadByte(); cancel() }()
	response := SecurityResponse{}
	if request.Action != "sign" && (request.Audience != "" || request.Path != "" || request.Digest != "") {
		err = errors.New("unexpected fields in remote security request")
	} else if request.Action != "policy" && request.Policy != nil {
		err = errors.New("unexpected security policy")
	} else {
		switch request.Action {
		case "status":
		case "unlock":
			err = authority.Unlock(ctx, pid)
		case "lock":
			authority.Lock()
		case "policy":
			if request.Policy == nil {
				err = errors.New("security policy is required")
			} else {
				err = authority.UpdatePolicy(ctx, pid, *request.Policy)
			}
		case "sign":
			var proof SecurityProof
			proof, err = authority.Sign(ctx, request.Audience, request.Path, request.Digest)
			if err == nil {
				response.Proof = &proof
			}
		default:
			err = errors.New("unknown remote security operation")
		}
	}
	if err != nil {
		response.Error = err.Error()
		if errors.Is(err, ErrOperatorLocked) {
			response.ErrorCode = "operator_locked"
		}
		if len(response.Error) > 1024 {
			response.Error = "operating-system security operation failed"
		}
	} else if response.Proof == nil {
		status := authority.Status()
		response.Status = &status
	}
	_ = json.NewEncoder(connection).Encode(response)
}

func CallSecurity(ctx context.Context, request SecurityRequest) (SecurityResponse, error) {
	platform, err := osauth.New()
	if err != nil {
		return SecurityResponse{}, err
	}
	directory, err := platform.SecurityDirectory()
	if err != nil {
		return SecurityResponse{}, err
	}
	return callSecurityAt(ctx, directory, request)
}

func callSecurityAt(ctx context.Context, directory string, request SecurityRequest) (SecurityResponse, error) {
	var response SecurityResponse
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded)+1 > securityIPCBytes {
		return response, errors.New("remote security request is too large")
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "unix", filepath.Join(directory, "authority.sock"))
	if err != nil {
		return response, errors.New("OS remote authority is unavailable; start the updated local broker service before unlocking")
	}
	connection := raw.(*net.UnixConn)
	defer connection.Close()
	uid, _, err := securityPeer(connection)
	if err != nil || uid != os.Getuid() {
		return response, errors.New("remote security authority has an invalid OS identity")
	}
	if err := connection.SetDeadline(time.Now().Add(125 * time.Second)); err != nil {
		return response, err
	}
	stop := context.AfterFunc(ctx, func() {
		// Cancellation already owns the error; closing only interrupts I/O.
		_ = connection.Close()
	})
	defer stop()
	if _, err := connection.Write(append(encoded, '\n')); err != nil {
		return response, err
	}
	data, err := io.ReadAll(io.LimitReader(connection, securityIPCBytes+1))
	if ctx.Err() != nil {
		return response, ctx.Err()
	}
	if err != nil {
		return response, err
	}
	if len(data) > securityIPCBytes {
		return response, errors.New("remote security response is too large")
	}
	if err := Decode(data, &response); err != nil {
		return response, err
	}
	if response.Error != "" {
		if response.ErrorCode == "operator_locked" {
			return response, ErrOperatorLocked
		}
		return response, errors.New(response.Error)
	}
	return response, nil
}
