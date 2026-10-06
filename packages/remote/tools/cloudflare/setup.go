// Package cloudflare owns browser authentication and locally managed tunnel
// provisioning. The engine sees only the generic remote edge process contract.
package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

var hostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
var tunnelPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Runner func(context.Context, string, []string, bool) error

type Setup struct {
	State      string
	Output     string
	Hostname   string
	Origin     string
	Executable string
	Home       string
	Progress   io.Writer
	Run        Runner
}

type setupState struct {
	Hostname      string `json:"hostname"`
	Origin        string `json:"origin"`
	Name          string `json:"name"`
	CreateStarted bool   `json:"create_started"`
	Routed        bool   `json:"routed"`
}

func (setup Setup) Execute(ctx context.Context) error {
	if setup.Progress == nil {
		setup.Progress = os.Stderr
	}
	if len(setup.Hostname) > 253 || !hostnamePattern.MatchString(setup.Hostname) {
		return errors.New("choose a lowercase hostname in a domain already managed by Cloudflare")
	}
	if !strings.HasPrefix(setup.Origin, "http://") || contract.Endpoint(setup.Origin) != nil {
		return errors.New("origin for Cloudflare must be an explicit loopback HTTP endpoint")
	}
	if !filepath.IsAbs(setup.Executable) || !filepath.IsAbs(setup.Home) || !filepath.IsAbs(setup.Output) {
		return errors.New("setup requires absolute executable, home, and output paths")
	}
	if err := remoteio.PrivateDirectory(setup.State); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(setup.State, "setup.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	if setup.Run == nil {
		setup.Run = runCloudflared
	}
	statePath := filepath.Join(setup.State, "setup.json")
	state := setupState{}
	if err := remoteio.ReadPrivate(statePath, &state); os.IsNotExist(err) {
		name, err := remoteio.Secret()
		if err != nil {
			return err
		}
		state = setupState{Hostname: setup.Hostname, Origin: setup.Origin, Name: "dockpipe-" + name[:20]}
		if err := remoteio.WritePrivate(statePath, state); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if state.Hostname != setup.Hostname || state.Origin != setup.Origin {
		return errors.New("setup state belongs to a different hostname or origin")
	}
	certificate := filepath.Join(setup.Home, ".cloudflared", "cert.pem")
	if _, err := os.Lstat(certificate); os.IsNotExist(err) {
		fmt.Fprintln(setup.Progress, "1/3 Open Cloudflare in your browser and select the domain. If no browser appears, open the login URL printed below; keep this terminal running.")
		// cloudflared owns its browser login and standard certificate location.
		// Never copy this account-management certificate to workers.
		if err := setup.Run(ctx, setup.Executable, []string{"tunnel", "login"}, true); err != nil {
			return errors.New("browser login to Cloudflare did not complete")
		}
	} else if err != nil {
		return err
	} else {
		fmt.Fprintln(setup.Progress, "1/3 Reusing existing Cloudflare authorization; browser login is not needed.")
	}
	if err := privateCredential(certificate); err != nil {
		return err
	}
	fmt.Fprintln(setup.Progress, "2/3 Preparing your named tunnel. Existing setup is reused.")
	credentials := filepath.Join(setup.State, "tunnel.json")
	if _, err := os.Lstat(credentials); os.IsNotExist(err) {
		if state.CreateStarted {
			return errors.New("tunnel creation outcome is unknown; inspect the recorded tunnel name before retrying, do not create another tunnel")
		}
		state.CreateStarted = true
		if err := remoteio.WritePrivate(statePath, state); err != nil {
			return err
		}
		arguments := []string{"tunnel", "--origincert", certificate, "create", "--credentials-file", credentials, state.Name}
		if err := setup.Run(ctx, setup.Executable, arguments, false); err != nil {
			return errors.New("tunnel creation failed or has unknown outcome; setup state was preserved")
		}
	} else if err != nil {
		return err
	}
	if err := privateCredential(credentials); err != nil {
		return err
	}
	raw, err := os.ReadFile(credentials)
	if err != nil {
		return err
	}
	var credential struct {
		TunnelID string `json:"TunnelID"`
	}
	// Provider-owned credential fields remain opaque; only its documented ID is
	// used to bind routing and the runtime configuration.
	if err := json.Unmarshal(raw, &credential); err != nil || !tunnelPattern.MatchString(credential.TunnelID) {
		return errors.New("invalid tunnel credentials written by Cloudflare")
	}
	fmt.Fprintf(setup.Progress, "3/3 Connecting https://%s to the local broker.\n", setup.Hostname)
	if !state.Routed {
		arguments := []string{"tunnel", "--origincert", certificate, "route", "dns", credential.TunnelID, setup.Hostname}
		if err := setup.Run(ctx, setup.Executable, arguments, false); err != nil {
			return errors.New("DNS routing failed; existing DNS was not overwritten, rerun setup after resolving the conflict")
		}
		state.Routed = true
		if err := remoteio.WritePrivate(statePath, state); err != nil {
			return err
		}
	}
	configPath := filepath.Join(setup.State, "tunnel-config.json")
	// JSON is a YAML subset accepted by cloudflared. Only a credential path is
	// serialized; the secret stays in its private provider-owned file.
	config := map[string]any{
		"tunnel":           credential.TunnelID,
		"credentials-file": credentials,
		"ingress": []map[string]string{
			{"hostname": setup.Hostname, "service": setup.Origin},
			{"service": "http_status:404"},
		},
	}
	if err := remoteio.WritePrivate(configPath, config); err != nil {
		return err
	}
	edge := contract.EdgeConfig{
		Schema: contract.Version, Endpoint: "https://" + setup.Hostname, Executable: setup.Executable,
		Arguments: []string{"tunnel", "--config", configPath, "--no-autoupdate", "run", credential.TunnelID},
	}
	return remoteio.WritePrivate(setup.Output, edge)
}

func privateCredential(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 64<<10 {
		return errors.New("credential for Cloudflare must be a private regular file")
	}
	return infrastructure.ValidatePrivatePath(path, false)
}

func runCloudflared(ctx context.Context, executable string, arguments []string, interactive bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, arguments...)
	for _, name := range []string{"PATH", "HOME", "USER", "TMPDIR", "LANG", "DISPLAY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS", "XAUTHORITY", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE", "XDG_CONFIG_HOME", "BROWSER"} {
		if value, exists := os.LookupEnv(name); exists {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	if interactive {
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stderr, os.Stderr
	}
	command.WaitDelay = 2 * time.Second
	if err := process.Run(command); err != nil {
		return fmt.Errorf("cloudflared operation failed")
	}
	return nil
}
