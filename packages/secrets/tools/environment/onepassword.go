package environment

import (
	"context"

	contract "dockpipe/src/lib/domain/secretenv"
)

func onepassword(ctx context.Context, request contract.Request, run Runner, executable string) (map[string]string, error) {
	if err := parameters(request, []string{"environment_id"}, "account"); err != nil {
		return nil, err
	}
	arguments := []string{"op", "run", "--no-masking", "--environment=" + request.Parameters["environment_id"]}
	if account := request.Parameters["account"]; account != "" {
		arguments = append(arguments, "--account="+account)
	}
	// Unmasked output is captured by the resolver pipe and is never attached to a terminal.
	return captureWith(ctx, request, run, arguments, executable)
}
