package environment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"

	contract "dockpipe/src/lib/domain/secretenv"
)

func azure(ctx context.Context, request contract.Request, run Runner) (map[string]string, error) {
	if err := parameters(request, []string{"vault_name"}, "subscription"); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(request.Bindings))
	for name := range request.Bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	values := map[string]string{}
	for _, name := range names {
		arguments := []string{"az", "keyvault", "secret", "show", "--only-show-errors", "--output=json",
			"--vault-name=" + request.Parameters["vault_name"], "--name=" + request.Bindings[name]}
		if subscription := request.Parameters["subscription"]; subscription != "" {
			arguments = append(arguments, "--subscription="+subscription)
		}
		output, err := run(ctx, arguments, nil, os.Environ())
		if err != nil {
			return nil, err
		}
		var secret struct{ Value *string }
		err = json.Unmarshal(output, &secret)
		clear(output)
		if err != nil || secret.Value == nil {
			return nil, errors.New("Azure Key Vault returned no string secret (output withheld)")
		}
		values[name] = *secret.Value
	}
	return values, nil
}
