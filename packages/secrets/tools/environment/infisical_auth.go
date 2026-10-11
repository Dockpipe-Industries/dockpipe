package environment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"

	contract "dockpipe/src/lib/domain/secretenv"
)

type sessionState int

const (
	sessionReady sessionState = iota
	sessionMissing
	sessionExpired
)

var errProfileMissing = errors.New("requested Infisical profile does not exist")

func validateInfisical(request contract.Request) error {
	return parameters(request, []string{"project_id", "environment"}, "path", "domain", "profile", "organization_id")
}

func hasInfisicalToken() bool {
	return os.Getenv("INFISICAL_TOKEN") != ""
}

func infisicalScope(request contract.Request) []string {
	var arguments []string
	for _, field := range []struct{ parameter, flag string }{
		{"domain", "domain"}, {"profile", "profile"}, {"organization_id", "org"},
	} {
		if value := request.Parameters[field.parameter]; value != "" {
			arguments = append(arguments, "--"+field.flag+"="+value)
		}
	}
	return arguments
}

func infisicalSession(ctx context.Context, request contract.Request, run Runner) (sessionState, error) {
	arguments := append([]string{"infisical", "login", "status", "--json", "--silent"}, infisicalScope(request)...)
	output, runErr := run(ctx, arguments, nil, authenticationEnvironment(request))
	defer clear(output)
	if ctx.Err() != nil {
		return sessionMissing, ctx.Err()
	}
	if errors.Is(runErr, errProfileMissing) {
		return sessionMissing, nil
	}
	var report struct {
		Sessions *[]struct {
			Status string `json:"status"`
		} `json:"sessions"`
	}
	if json.Unmarshal(output, &report) != nil || report.Sessions == nil {
		return sessionMissing, errors.New("cannot determine Infisical login status; check CLI version, keyring and connection")
	}
	if len(*report.Sessions) == 0 {
		return sessionMissing, nil
	}
	if len(*report.Sessions) != 1 {
		return sessionMissing, errors.New("ambiguous Infisical login status")
	}
	switch (*report.Sessions)[0].Status {
	case "authenticated":
		if runErr == nil {
			return sessionReady, nil
		}
	case "expired", "rejected":
		return sessionExpired, nil
	}
	return sessionMissing, errors.New("unrecognized Infisical login status")
}

// AuthenticateInfisical owns browser login. Token-based automation never falls
// back to a user session, and explicit profiles leave the global default alone.
func AuthenticateInfisical(ctx context.Context, request contract.Request, run Runner, login Runner) error {
	if request.Schema != contract.Schema {
		return errors.New("invalid authentication request")
	}
	if err := contract.ValidateBindings(request.Bindings); err != nil {
		return err
	}
	if err := validateInfisical(request); err != nil {
		return err
	}
	if hasInfisicalToken() {
		return errors.New("token authentication cannot use browser login")
	}
	state, err := infisicalSession(ctx, request, run)
	if err != nil || state == sessionReady {
		return err
	}
	profile := request.Parameters["profile"]
	if profile == "" {
		profile = os.Getenv("INFISICAL_PROFILE")
	}
	if profile == "" {
		return errors.New("automatic login requires an explicit Infisical profile")
	}
	domain := request.Parameters["domain"]
	if domain == "" {
		domain = os.Getenv("INFISICAL_DOMAIN")
	}
	if domain == "" {
		domain = "https://app.infisical.com/api"
	}
	parsed, err := url.Parse(domain)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("automatic login requires an HTTPS Infisical domain")
	}
	arguments := []string{"infisical", "login", "--method=user", "--plain=false", "--silent", "--domain=" + domain}
	if state == sessionMissing {
		arguments = append(arguments, "--save-as="+profile)
	} else {
		arguments = append(arguments, "--profile="+profile)
	}
	// --organization-id opts the CLI into email/password login. Browser login
	// selects its organization in the browser; reads use the explicit --org scope.
	// Prevent inherited password/machine-login variables from changing the
	// explicitly requested browser flow. Preserve desktop/keyring environment.
	var environment []string
	for _, entry := range authenticationEnvironment(request) {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "INFISICAL_") {
			environment = append(environment, entry)
		}
	}
	output, err := login(ctx, arguments, nil, environment)
	clear(output)
	if err != nil {
		return err
	}
	return nil
}

// BrowserLogin keeps provider output private except for a validated browser URL.
func BrowserLogin(progress io.Writer) Runner {
	return func(ctx context.Context, arguments []string, input []byte, environment []string) ([]byte, error) {
		return runBrowserLogin(ctx, arguments, environment, progress)
	}
}
