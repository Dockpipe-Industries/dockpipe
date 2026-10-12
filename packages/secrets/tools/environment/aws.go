package environment

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	contract "dockpipe/src/lib/domain/secretenv"
)

func aws(ctx context.Context, request contract.Request, run Runner) (map[string]string, error) {
	if err := parameters(request, []string{"secret_id"}, "region", "profile", "version_stage"); err != nil {
		return nil, err
	}
	arguments := []string{"aws", "secretsmanager", "get-secret-value", "--output=json", "--no-cli-pager", "--secret-id=" + request.Parameters["secret_id"]}
	for _, option := range []struct{ parameter, flag string }{{"region", "region"}, {"profile", "profile"}, {"version_stage", "version-stage"}} {
		if value := request.Parameters[option.parameter]; value != "" {
			arguments = append(arguments, "--"+option.flag+"="+value)
		}
	}
	output, err := run(ctx, arguments, nil, os.Environ())
	if err != nil {
		return nil, err
	}
	defer clear(output)
	var secret struct{ SecretString *string }
	if json.Unmarshal(output, &secret) != nil || secret.SecretString == nil {
		return nil, errors.New("AWS environment requires a JSON SecretString; binary secrets are unsupported")
	}
	var values map[string]string
	if json.Unmarshal([]byte(*secret.SecretString), &values) != nil {
		return nil, errors.New("AWS SecretString must be an object of string values (output withheld)")
	}
	response, err := Capture(request, func(name string) (string, bool) { value, ok := values[name]; return value, ok })
	return response.Values, err
}
