package environment

import (
	"context"

	contract "dockpipe/src/lib/domain/secretenv"
)

func infisical(ctx context.Context, request contract.Request, run Runner, executable string) (map[string]string, error) {
	if err := parameters(request, []string{"project_id", "environment"}, "path", "domain"); err != nil {
		return nil, err
	}
	arguments := []string{"infisical", "run", "--silent", "--log-destination=stderr", "--expand=false", "--secret-overriding=false",
		"--projectId=" + request.Parameters["project_id"], "--env=" + request.Parameters["environment"]}
	for _, name := range []string{"path", "domain"} {
		if value := request.Parameters[name]; value != "" {
			arguments = append(arguments, "--"+name+"="+value)
		}
	}
	return captureWith(ctx, request, run, arguments, executable)
}
