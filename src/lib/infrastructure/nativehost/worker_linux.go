package nativehost

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure"
	"golang.org/x/sys/unix"
)

func workerIdentity() (Status, error) {
	real, effective, saved := unix.Getresuid()
	if real == 0 || real != effective || real != saved {
		return Status{}, errors.New("native host worker requires an ordinary user")
	}
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	capabilities := [2]unix.CapUserData{}
	if err := unix.Capget(&header, &capabilities[0]); err != nil {
		return Status{}, err
	}
	for _, capability := range capabilities {
		if capability.Effective != 0 || capability.Permitted != 0 || capability.Inheritable != 0 {
			return Status{}, errors.New("native host worker must have no capabilities")
		}
	}
	account, err := user.LookupId(strconv.Itoa(real))
	if err != nil {
		return Status{}, errors.New("native host worker account unavailable")
	}
	hostname, err := os.Hostname()
	if err != nil {
		return Status{}, err
	}
	groups, err := os.Getgroups()
	if err != nil {
		return Status{}, err
	}
	status := Status{Schema: StatusSchema, Node: hostname, Username: account.Username, Home: account.HomeDir, UID: uint32(real), GID: uint32(os.Getgid())}
	for _, group := range groups {
		status.Groups = append(status.Groups, uint32(group))
	}
	if !validWorkerStatus(status, status.UID, status.GID) {
		return Status{}, errors.New("invalid native host worker identity")
	}
	return status, nil
}

func RunWorker(ctx context.Context, options WorkerOptions) error {
	if !validSocketPath(options.Socket) || !validToken(options.Nonce) {
		return errors.New("invalid native host worker options")
	}
	identity, err := workerIdentity()
	if err != nil {
		return err
	}
	startup, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	var connection net.Conn
	for {
		connection, err = (&net.Dialer{Timeout: time.Second}).DialContext(startup, "unix", options.Socket)
		if err == nil {
			break
		}
		select {
		case <-startup.Done():
			return ErrUnavailable
		case <-time.After(100 * time.Millisecond):
		}
	}
	transport := &wire{ReadWriteCloser: connection}
	defer transport.Close()
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	if transport.send(frame{Kind: "worker", Nonce: options.Nonce, Status: &identity}) != nil {
		return ErrUnavailable
	}
	accepted, err := transport.receive()
	if err != nil || accepted.Kind != "accepted" {
		return ErrUnavailable
	}
	connection.SetDeadline(time.Time{})
	if options.Ready != nil {
		if err := options.Ready(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { transport.Close() })
	defer stop()
	var mu sync.Mutex
	jobs := make(map[uint64]context.CancelFunc)
	terminals := make(map[uint64]*ptyProcess)
	var wait sync.WaitGroup
	defer func() {
		cancel()
		mu.Lock()
		for _, cancelJob := range jobs {
			cancelJob()
		}
		for _, terminal := range terminals {
			terminal.Close()
		}
		mu.Unlock()
		wait.Wait()
	}()
	for {
		request, err := transport.receive()
		if err != nil {
			return err
		}
		mu.Lock()
		switch request.Kind {
		case "snapshot", "docker", "docker-query":
			if request.ID == 0 || len(jobs) >= 8 || jobs[request.ID] != nil {
				mu.Unlock()
				return errors.New("invalid native host worker request")
			}
			jobContext, cancelJob := boundedContext(ctx)
			jobs[request.ID] = cancelJob
			wait.Add(1)
			go func(request frame) {
				defer wait.Done()
				defer cancelJob()
				defer func() {
					mu.Lock()
					delete(jobs, request.ID)
					mu.Unlock()
				}()
				if request.Kind == "docker-query" {
					result := runDockerQuery(jobContext, request.ResourceKind, request.ResourceID)
					transport.send(frame{Kind: "docker-query", ID: request.ID, DockerQuery: &result})
					return
				}
				if request.Kind == "docker" {
					result := runDocker(jobContext, request.Action, request.Container)
					transport.send(frame{Kind: "docker", ID: request.ID, DockerResult: &result})
					return
				}
				snapshot, err := infrastructure.ReadLocalMachineSnapshot(jobContext, request.System, request.Docker)
				if err != nil {
					transport.send(frame{Kind: "error", ID: request.ID, Error: "snapshot_unavailable"})
					return
				}
				data, err := json.Marshal(snapshot)
				if err != nil || len(data) > 1<<20 {
					transport.send(frame{Kind: "error", ID: request.ID, Error: "snapshot_unavailable"})
					return
				}
				transport.send(frame{Kind: "snapshot", ID: request.ID, Snapshot: data})
			}(request)
		case "terminal":
			if request.ID == 0 || len(terminals) >= 8 || terminals[request.ID] != nil {
				mu.Unlock()
				return errors.New("native host terminal limit")
			}
			terminal, err := startPTY(identity, request)
			if err != nil {
				mu.Unlock()
				transport.send(frame{Kind: "error", ID: request.ID, Error: "terminal_unavailable"})
				continue
			}
			terminals[request.ID] = terminal
			wait.Add(1)
			go func(id uint64) {
				defer wait.Done()
				defer terminal.Close()
				transport.send(frame{Kind: "opened", ID: id})
				buffer := make([]byte, 32<<10)
				for {
					n, err := terminal.master.Read(buffer)
					if n > 0 && transport.send(frame{Kind: "output", ID: id, Data: buffer[:n]}) != nil {
						break
					}
					if err != nil {
						break
					}
				}
				exit := terminal.Wait()
				transport.send(frame{Kind: "exit", ID: id, Exit: exit})
				mu.Lock()
				delete(terminals, id)
				mu.Unlock()
			}(request.ID)
		case "input":
			terminal := terminals[request.ID]
			if terminal == nil {
				mu.Unlock()
				return errors.New("unknown native host terminal")
			}
			// PTY writes have a finite deadline so a stopped job cannot hold
			// the shared reverse transport indefinitely.
			terminal.master.SetWriteDeadline(time.Now().Add(time.Second))
			_, err := terminal.master.Write(request.Data)
			if err != nil {
				terminal.Close()
			}
		case "resize":
			if terminal := terminals[request.ID]; terminal != nil {
				terminal.Resize(request.Rows, request.Columns)
			}
		case "close":
			if cancelJob := jobs[request.ID]; cancelJob != nil {
				cancelJob()
			}
			if terminal := terminals[request.ID]; terminal != nil {
				terminal.Close()
			}
		default:
			mu.Unlock()
			return errors.New("invalid native host worker operation")
		}
		mu.Unlock()
	}
}
