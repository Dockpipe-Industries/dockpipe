package environment

import (
	"context"

	contract "dockpipe/src/lib/domain/secretenv"
)

func infisical(ctx context.Context, request contract.Request, run Runner, executable string) (map[string]string, error) {
	if err := validateInfisical(request); err != nil {
		return nil, err
	}
	if !hasInfisicalToken() {
		state, err := infisicalSession(ctx, request, run)
		if err != nil {
			return nil, err
		}
		if state != sessionReady {
			return nil, contract.ErrAuthenticationRequired
		}
	}
	arguments := []string{"infisical", "run", "--silent", "--log-destination=stderr", "--expand=false", "--secret-overriding=false",
		"--projectId=" + request.Parameters["project_id"], "--env=" + request.Parameters["environment"]}
	if path := request.Parameters["path"]; path != "" {
		arguments = append(arguments, "--path="+path)
	}
	arguments = append(arguments, infisicalScope(request)...)
	return captureWith(ctx, request, run, arguments, executable)
}
