package environment

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/secretenv"
)

func TestProvidersReturnOnlyBoundValues(t *testing.T) {
	tests := []struct {
		provider   string
		parameters map[string]string
		output     string
		prefix     []string
	}{
		{"onepassword", map[string]string{"environment_id": "prod-id"}, `{"schema":"dockpipe.secret-environment/v1","values":{"TOKEN":"line1\nline2"}}`, []string{"op", "run"}},
		{"infisical", map[string]string{"project_id": "project-id", "environment": "prod"}, `{"schema":"dockpipe.secret-environment/v1","values":{"TOKEN":"line1\nline2"}}`, []string{"infisical", "run"}},
		{"aws-secretsmanager", map[string]string{"secret_id": "prod/packages"}, `{"SecretString":"{\"remote-key\":\"line1\\nline2\",\"UNRELATED\":\"hidden\"}"}`, []string{"aws", "secretsmanager", "get-secret-value"}},
		{"azure-keyvault", map[string]string{"vault_name": "packages-prod"}, `{"value":"line1\nline2"}`, []string{"az", "keyvault", "secret", "show"}},
	}
	for _, test := range tests {
		t.Run(test.provider, func(t *testing.T) {
			request := contract.Request{Schema: contract.Schema, Parameters: test.parameters, Bindings: map[string]string{"TOKEN": "remote-key"}}
			runner := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
				if !reflect.DeepEqual(args[:len(test.prefix)], test.prefix) {
					t.Fatalf("unexpected provider command: %v", args)
				}
				return []byte(test.output), nil
			}
			response, err := Resolve(context.Background(), test.provider, request, runner, "/helper")
			if err != nil || len(response.Values) != 1 || response.Values["TOKEN"] != "line1\nline2" {
				t.Fatalf("incorrect response, error=%v", err)
			}
		})
	}
}

func TestCaptureDoesNotFallBackToParentOrReturnUnrequestedValues(t *testing.T) {
	t.Setenv("REMOTE_KEY", "parent-must-not-be-used")
	t.Setenv("TOKEN", "old-value")
	request := contract.Request{Schema: contract.Schema, Parameters: map[string]string{"environment_id": "prod"}, Bindings: map[string]string{"TOKEN": "REMOTE_KEY"}}
	runner := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
		for _, entry := range env {
			if strings.HasPrefix(entry, "REMOTE_KEY=") || strings.HasPrefix(entry, "TOKEN=") {
				t.Fatal("binding inherited from parent")
			}
		}
		var captured contract.Request
		if json.Unmarshal(input, &captured) != nil || captured.Bindings["TOKEN"] != "REMOTE_KEY" {
			t.Fatal("capture did not receive explicit binding")
		}
		_, err := Capture(captured, func(string) (string, bool) { return "", false })
		return nil, err
	}
	if _, err := Resolve(context.Background(), "onepassword", request, runner, "/helper"); err == nil {
		t.Fatal("missing production variable accepted")
	}
}

func TestAzureFailureDoesNotReturnPartialEnvironment(t *testing.T) {
	request := contract.Request{Schema: contract.Schema, Parameters: map[string]string{"vault_name": "prod"}, Bindings: map[string]string{"FIRST": "first", "SECOND": "second"}}
	count := 0
	runner := func(context.Context, []string, []byte, []string) ([]byte, error) {
		count++
		if count == 2 {
			return nil, errors.New("provider unavailable")
		}
		return []byte(`{"value":"secret"}`), nil
	}
	response, err := Resolve(context.Background(), "azure-keyvault", request, runner, "")
	if err == nil || len(response.Values) != 0 {
		t.Fatal("partial environment escaped")
	}
}

func TestUnknownParametersAndAWSBinaryFail(t *testing.T) {
	request := contract.Request{Schema: contract.Schema, Parameters: map[string]string{"secret_id": "prod", "secret_value": "must-not-be-configured"}, Bindings: map[string]string{"TOKEN": "key"}}
	called := false
	runner := func(context.Context, []string, []byte, []string) ([]byte, error) {
		called = true
		return []byte(`{"SecretBinary":"c2VjcmV0"}`), nil
	}
	if _, err := Resolve(context.Background(), "aws-secretsmanager", request, runner, ""); err == nil || called {
		t.Fatal("unknown config accepted")
	}
	delete(request.Parameters, "secret_value")
	if _, err := Resolve(context.Background(), "aws-secretsmanager", request, runner, ""); err == nil {
		t.Fatal("binary secret accepted as text")
	}
}
