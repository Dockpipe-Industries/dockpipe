package environment

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/secretenv"
)

func infisicalRequest() contract.Request {
	return contract.Request{
		Schema: contract.Schema,
		Parameters: map[string]string{
			"project_id": "project", "environment": "dev", "domain": "https://vault.example/api",
			"profile": "local-dev", "organization_id": "organization",
		},
		Bindings: map[string]string{"TOKEN": "remote"},
	}
}

func TestInfisicalLoginState(t *testing.T) {
	t.Setenv("INFISICAL_TOKEN", "")
	for _, test := range []struct {
		name, output string
		failure      error
		authRequired bool
	}{
		{"missing", `{"sessions":[]}`, errors.New("exit 1"), true},
		{"missing-profile", "", errProfileMissing, true},
		{"expired", `{"sessions":[{"status":"expired"}]}`, errors.New("exit 1"), true},
		{"revoked", `{"sessions":[{"status":"rejected"}]}`, errors.New("exit 1"), true},
		{"keyring-error", "private-diagnostics", errors.New("keyring unavailable"), false},
		{"malformed", `{}`, nil, false},
		{"ambiguous", `{"sessions":[{"status":"authenticated"},{"status":"expired"}]}`, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			runner := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
				calls++
				return []byte(test.output), test.failure
			}
			_, err := Resolve(context.Background(), "infisical", infisicalRequest(), runner, "/helper")
			if err == nil || errors.Is(err, contract.ErrAuthenticationRequired) != test.authRequired || calls != 1 || strings.Contains(err.Error(), "private-diagnostics") {
				t.Fatalf("incorrect login classification: %v, calls=%d", err, calls)
			}
		})
	}
}

func TestInfisicalBrowserLoginPreservesTargetAndDoesNotInheritCredentials(t *testing.T) {
	t.Setenv("INFISICAL_TOKEN", "")
	t.Setenv("INFISICAL_PROFILE", "unrelated-profile")
	t.Setenv("INFISICAL_PASSWORD", "private-fixture")
	t.Setenv("INFISICAL_CLIENT_SECRET", "private-fixture")
	for _, state := range []string{"missing", "expired", "authenticated"} {
		t.Run(state, func(t *testing.T) {
			status := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
				for _, wanted := range []string{"--profile=local-dev", "--org=organization", "--domain=https://vault.example/api"} {
					if !containsArgument(args, wanted) {
						t.Fatalf("scope missing: %s", wanted)
					}
				}
				if state == "missing" {
					return nil, errProfileMissing
				}
				return []byte(`{"sessions":[{"status":"` + state + `"}]}`), nil
			}
			calls := 0
			login := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
				calls++
				flag := "--profile=local-dev"
				if state == "missing" {
					flag = "--save-as=local-dev"
				}
				if !containsArgument(args, flag) || !containsArgument(args, "--method=user") || containsArgument(args, "--organization-id=organization") {
					t.Fatalf("wrong login target: %v", args)
				}
				for _, entry := range env {
					if strings.HasPrefix(entry, "INFISICAL_") {
						t.Fatal("inherited login overrides leaked")
					}
				}
				return nil, nil
			}
			if err := AuthenticateInfisical(context.Background(), infisicalRequest(), status, login); err != nil {
				t.Fatal(err)
			}
			wanted := 1
			if state == "authenticated" {
				wanted = 0
			}
			if calls != wanted {
				t.Fatalf("unexpected login count: %d", calls)
			}
		})
	}
}

func containsArgument(arguments []string, wanted string) bool {
	for _, argument := range arguments {
		if argument == wanted {
			return true
		}
	}
	return false
}

func TestInfisicalMachineTokenNeverFallsBackToBrowser(t *testing.T) {
	t.Setenv("INFISICAL_TOKEN", "private-token")
	calls := 0
	runner := func(ctx context.Context, args []string, input []byte, env []string) ([]byte, error) {
		calls++
		if !reflect.DeepEqual(args[:2], []string{"infisical", "run"}) {
			t.Fatal("machine token invoked user login")
		}
		return nil, errors.New("provider failure")
	}
	_, err := Resolve(context.Background(), "infisical", infisicalRequest(), runner, "/helper")
	if err == nil || errors.Is(err, contract.ErrAuthenticationRequired) || calls != 1 {
		t.Fatal("machine authentication failure was not preserved")
	}
	if AuthenticateInfisical(context.Background(), infisicalRequest(), runner, runner) == nil || calls != 1 {
		t.Fatal("browser fallback accepted a machine token")
	}
}

func TestLoginProgressOnlyRevealsValidatedChallenge(t *testing.T) {
	domain, _ := url.Parse("https://vault.example/api")
	var output bytes.Buffer
	var opened []string
	progress := &loginProgress{domain: domain, output: &output, open: func(value string) { opened = append(opened, value) }}
	for _, chunk := range []string{
		"private-secret\nhttps://evil.example/login?callback_port=1234\n",
		"https://vault.example/login?callback_port=1234&token=private-secret\n",
		"https://vault.example/login?callback_port=0\n",
		"To complete login: https://vault.example/login?callback_",
		"port=1234 \nprivate-secret\nhttps://vault.example/login?callback_port=5678\n",
	} {
		if _, err := progress.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if len(opened) != 1 || opened[0] != "https://vault.example/login?callback_port=1234" || strings.Contains(output.String(), "private-secret") || strings.Contains(output.String(), "evil") {
		t.Fatal("unsafe or repeated browser challenge")
	}
}
