package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func remoteSecurity(ctx context.Context, command string, changes map[string]bool, idle int, screenLock, sleep bool) error {
	action := "status"
	switch command {
	case "unlock":
		action = "unlock"
	case "lock":
		action = "lock"
	case "security-policy":
		action = "policy"
	}
	request := remoteio.SecurityRequest{Action: action}
	if action == "policy" {
		current, err := remoteio.CallSecurity(ctx, remoteio.SecurityRequest{Action: "status"})
		if err != nil {
			return err
		}
		if current.Status == nil {
			return errors.New("remote security authority returned no policy")
		}
		policy := current.Status.RemoteSecurityPolicy
		if changes["idle-seconds"] {
			policy.IdleSeconds = idle
		}
		if changes["lock-on-screen-lock"] {
			policy.LockOnScreenLock = screenLock
		}
		if changes["lock-on-sleep"] {
			policy.LockOnSleep = sleep
		}
		if err := policy.Validate(); err != nil {
			return err
		}
		request.Policy = &policy
	}
	if action == "unlock" || action == "policy" {
		closeAgent, err := startTerminalSecurityAgent(ctx)
		if err != nil {
			return err
		}
		defer closeAgent()
	}
	response, err := remoteio.CallSecurity(ctx, request)
	if err != nil {
		if command != "security-status" {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(remoteio.SecurityStatus{
			RemoteSecurityPolicy: contract.DefaultRemoteSecurityPolicy(), Locked: true,
			AbsoluteSeconds: 3600, Detail: err.Error(),
		})
	}
	if response.Status == nil {
		return errors.New("remote security authority returned no status")
	}
	return json.NewEncoder(os.Stdout).Encode(response.Status)
}
