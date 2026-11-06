//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"sort"
	"syscall"
	"unsafe"

	"dockpipe/workflows/internal/synology/boundedcmd"
	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/hostidentity"
)

const schema = "dockpipe.synology_host_identity_qualification.v1"
const maximumResponseBytes = 16 << 10

type workerRequest struct {
	Environment []string `json:"environment"`
}

type probeRequest struct {
	Nonce     string `json:"nonce"`
	ParentPID int    `json:"parent_pid"`
}

type childReport struct {
	Status            hostidentity.Status `json:"status"`
	Nonce             string              `json:"nonce"`
	ParentPID         int                 `json:"parent_pid"`
	ParentDeathSignal int32               `json:"parent_death_signal"`
}

type identityReport struct {
	Schema              string              `json:"schema"`
	AuthenticatedUser   string              `json:"authenticated_user"`
	Helper              hostidentity.Status `json:"helper"`
	Delegated           hostidentity.Status `json:"delegated"`
	CapabilityAdmission bool                `json:"capability_admission"`
}

type workerResponse struct {
	Report *identityReport `json:"report,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func decodeRequest(reader io.Reader, maximumBytes int, result any) error {
	data, err := io.ReadAll(io.LimitReader(reader, int64(maximumBytes)+1))
	if err != nil || len(data) > maximumBytes {
		return errors.New("request exceeds qualification budget")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return errors.New("invalid qualification request")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("unexpected qualification request data")
	}
	return nil
}

func writeResponse(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data)+1 > maximumResponseBytes {
		return errors.New("qualification response exceeds budget")
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}

func runProbe() error {
	image, err := inheritExecutable()
	if err != nil {
		return err
	}
	defer image.Close()
	var request probeRequest
	if err := decodeRequest(os.Stdin, 1024, &request); err != nil {
		return err
	}
	nonce, err := hex.DecodeString(request.Nonce)
	if err != nil || len(nonce) != 32 {
		return errors.New("invalid qualification challenge")
	}
	var parentDeathSignal int32
	_, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, 2, uintptr(unsafe.Pointer(&parentDeathSignal)), 0, 0, 0, 0)
	if errno != 0 || parentDeathSignal != int32(syscall.SIGKILL) || request.ParentPID <= 1 || os.Getppid() != request.ParentPID {
		return errors.New("delegated child supervision admission failed")
	}
	metadata, err := hostidentity.Read(schema)
	if err != nil || !unprivileged(metadata.Identity) {
		return errors.New("delegated child did not enter an unprivileged state")
	}
	return writeResponse(os.Stdout, childReport{Status: metadata, Nonce: request.Nonce, ParentPID: os.Getppid(), ParentDeathSignal: parentDeathSignal})
}

func verifyChild(report childReport, account localAccount, nonce, hostname string) bool {
	identity := report.Status.Identity
	if report.Nonce != nonce || report.ParentPID != os.Getpid() || report.ParentDeathSignal != int32(syscall.SIGKILL) || report.Status.Schema != schema || report.Status.Hostname != hostname || !unprivileged(identity) {
		return false
	}
	for _, identifier := range []int{identity.UID, identity.EUID, identity.SavedUID, identity.FSUID} {
		if uint32(identifier) != account.UID {
			return false
		}
	}
	for _, identifier := range []int{identity.GID, identity.EGID, identity.SavedGID, identity.FSGID} {
		if uint32(identifier) != account.GID {
			return false
		}
	}
	if len(identity.Groups) != len(account.Groups) {
		return false
	}
	groups := append([]int(nil), identity.Groups...)
	sort.Ints(groups)
	for index, identifier := range groups {
		if uint32(identifier) != account.Groups[index] {
			return false
		}
	}
	return true
}

func delegateIdentity(account localAccount, helper hostidentity.Status, image *sealedImage) (hostidentity.Status, error) {
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return hostidentity.Status{}, errors.New("cannot create qualification challenge")
	}
	nonce := hex.EncodeToString(challenge)
	request, _ := json.Marshal(probeRequest{Nonce: nonce, ParentPID: os.Getpid()})
	// A per-invocation package-UID supervisor owns this process. Its caller can
	// kill the supervisor, and Pdeathsig then kills the different-UID child.
	// Go's pinned Linux fork/exec sets Pdeathsig after Credential changes and
	// checks getppid before exec. Keep that spawning OS thread alive through Wait.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	command, err := image.command(context.Background(), "identity-probe")
	if err != nil {
		return hostidentity.Status{}, err
	}
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	command.Stdin = bytes.NewReader(request)
	command.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: account.UID, Gid: account.GID, Groups: account.Groups},
		Pdeathsig:  syscall.SIGKILL,
	}
	output, failure := boundedcmd.Run(command, maximumResponseBytes)
	if failure != nil {
		return hostidentity.Status{}, errors.New("delegated identity child failed")
	}
	var report childReport
	if err := decodeRequest(bytes.NewBufferString(output), maximumResponseBytes, &report); err != nil || !verifyChild(report, account, nonce, helper.Hostname) {
		return hostidentity.Status{}, errors.New("delegated identity evidence did not match the authorized account")
	}
	return report.Status, nil
}

func runWorker() error {
	image, err := inheritExecutable()
	if err != nil {
		return err
	}
	defer image.Close()
	helper, err := prepareCapabilities(true)
	if err != nil {
		return err
	}
	var request workerRequest
	// JSON escaping can expand the already bounded 32 KiB CGI envelope.
	if err := decodeRequest(os.Stdin, 128<<10, &request); err != nil {
		return err
	}
	environment, err := dsmauth.Environment(request.Environment)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), dsmauth.Timeout)
	defer cancel()
	username, err := dsmauth.Authenticate(ctx, environment)
	if err != nil {
		if errors.Is(err, dsmauth.ErrUnauthenticated) {
			return writeResponse(os.Stdout, workerResponse{Error: "unauthenticated"})
		}
		return writeResponse(os.Stdout, workerResponse{Error: "authentication_unavailable"})
	}
	account, err := resolveAccount(username)
	if err != nil {
		return writeResponse(os.Stdout, workerResponse{Error: "local_account_ineligible"})
	}
	delegated, err := delegateIdentity(account, helper, image)
	if err != nil {
		return writeResponse(os.Stdout, workerResponse{Error: "delegation_failed"})
	}
	return writeResponse(os.Stdout, workerResponse{Report: &identityReport{
		Schema: schema, AuthenticatedUser: username, Helper: helper, Delegated: delegated, CapabilityAdmission: true,
	}})
}

func invokeWorker(ctx context.Context, environment []string, image *sealedImage) (workerResponse, error) {
	request, err := json.Marshal(workerRequest{Environment: environment})
	if err != nil || len(request) > 128<<10 {
		return workerResponse{}, errors.New("authentication envelope exceeds worker budget")
	}
	command, err := image.command(ctx, "identity-worker")
	if err != nil {
		return workerResponse{}, err
	}
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	command.Stdin = bytes.NewReader(request)
	command.SysProcAttr = &syscall.SysProcAttr{AmbientCaps: []uintptr{6, 7}, Pdeathsig: syscall.SIGKILL}
	output, failure := boundedcmd.Run(command, maximumResponseBytes)
	if failure != nil {
		return workerResponse{}, errors.New("identity qualification worker failed")
	}
	var response workerResponse
	if err := decodeRequest(bytes.NewBufferString(output), maximumResponseBytes, &response); err != nil {
		return workerResponse{}, err
	}
	return response, nil
}
