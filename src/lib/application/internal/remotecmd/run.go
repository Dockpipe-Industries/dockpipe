package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

const usage = `dockpipe remote <command> [options]
  setup   --resolver <name> --hostname <hostname>  Browser login and edge setup
  init    [--listen 127.0.0.1:47831]               Initialize a private broker
  serve                                          Run broker and configured edge
  invite  --node <name> --out <private-file>       Issue a 15-minute pairing file
  pair    --invite <file> --profiles <json-file>   Approve workflows on this worker
  worker                                         Run the outbound worker
  submit  --node <name> --profile <name> --id <id> Submit once; same ID is idempotent
  jobs                                           List job states
  result  --id <id> [--out <new-private-directory>] Inspect/download a result
  cancel  --id <id>                               Request cancellation
  revoke  --id <node>                             Revoke a worker
  service --role broker|worker                    Install a user service
All commands accept --state <directory>. Remote nodes currently target Linux/macOS.
Profiles are local JSON: {"bench":{"workdir":"/absolute/checkout","workflow":"bench",
"timeout_seconds":3600,"artifacts":["results/bench.json"]}}.
Secrets stay in private state/pairing files. Do not paste pairing files into chat or logs.
`

func Run(args []string, checkDependencies func(string) error) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(usage)
		return nil
	}
	command := args[0]
	flags := flag.NewFlagSet("remote "+command, flag.ContinueOnError)
	state := flags.String("state", "", "private state directory")
	listen := flags.String("listen", "127.0.0.1:47831", "loopback origin address")
	resolver := flags.String("resolver", "", "edge resolver profile")
	hostname := flags.String("hostname", "", "public edge hostname")
	node := flags.String("node", "", "worker name")
	profile := flags.String("profile", "", "worker-approved workflow profile")
	id := flags.String("id", "", "stable job ID or node for revoke")
	out := flags.String("out", "", "private output destination")
	invite := flags.String("invite", "", "pairing file")
	profiles := flags.String("profiles", "", "locally approved profiles file")
	role := flags.String("role", "", "broker or worker service")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected remote positional arguments")
	}
	root, err := stateRoot(*state)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return infrastructure.RunOperation(os.Stderr, "remote."+command, "Remote "+command, nil, func() error {
		switch command {
		case "init":
			_, err := initialize(root, *listen)
			return err
		case "setup":
			return setup(ctx, root, *listen, *resolver, *hostname, checkDependencies)
		case "serve":
			return serve(ctx, root)
		case "pair":
			return pair(ctx, root, *invite, *profiles)
		case "worker":
			var config contract.WorkerConfig
			if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
				return err
			}
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			return remoteio.RunWorker(ctx, root, executable, config)
		case "service":
			return installService(ctx, root, *role)
		case "invite", "submit", "jobs", "result", "cancel", "revoke":
			return operator(ctx, root, command, *node, *profile, *id, *out)
		default:
			return fmt.Errorf("unknown remote command %q\n%s", command, usage)
		}
	})
}

func operator(ctx context.Context, root, command, node, profile, id, output string) error {
	var config OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &config); err != nil {
		return err
	}
	// Administration stays on loopback. Worker pairing uses the published edge.
	client, err := remoteio.NewClient("http://"+config.Listen, config.Token)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	switch command {
	case "invite":
		if output == "" {
			return errors.New("--out is required; pairing secrets are never printed")
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			return errors.New("invitation output already exists or is inaccessible")
		}
		if err := remoteio.PrivateDirectory(filepath.Dir(output)); err != nil {
			return err
		}
		var invitation contract.Invitation
		if err := client.Call(ctx, "/v1/invite", map[string]string{"node": node}, &invitation); err != nil {
			return err
		}
		invitation.Endpoint = config.Endpoint
		return remoteio.WritePrivate(output, invitation)
	case "submit":
		request := contract.Submission{ID: id, Node: node, Profile: profile}
		if err := request.Validate(); err != nil {
			return err
		}
		var job contract.Job
		if err := client.Call(ctx, "/v1/submit", request, &job); err != nil {
			return err
		}
		job.Result = nil
		return json.NewEncoder(os.Stdout).Encode(job)
	case "result":
		var job contract.Job
		if err := client.Call(ctx, "/v1/job", map[string]string{"id": id}, &job); err != nil {
			return err
		}
		if output != "" {
			return saveResult(output, job)
		}
		job.Result = nil
		return json.NewEncoder(os.Stdout).Encode(job)
	case "cancel":
		var job contract.Job
		if err := client.Call(ctx, "/v1/cancel", map[string]string{"id": id}, &job); err != nil {
			return err
		}
		job.Result = nil
		return json.NewEncoder(os.Stdout).Encode(job)
	default:
		var response json.RawMessage
		if err := client.Call(ctx, "/v1/"+command, map[string]string{"id": id}, &response); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(response)
	}
}

func pair(ctx context.Context, root, invitePath, profilesPath string) error {
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "worker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	var invitation contract.Invitation
	if err := remoteio.ReadPrivate(invitePath, &invitation); err != nil {
		return err
	}
	if invitation.Schema != contract.Version || !contract.ValidID(invitation.Node) {
		return errors.New("invalid invitation")
	}
	if err := contract.Endpoint(invitation.Endpoint); err != nil {
		return err
	}
	profiles := map[string]contract.Profile{}
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		return err
	}
	if len(data) > 64<<10 {
		return errors.New("profiles file exceeds 64 KiB")
	}
	if err := remoteio.Decode(data, &profiles); err != nil {
		return err
	}
	if err := remoteio.ValidateProfiles(profiles); err != nil {
		return err
	}
	configPath := filepath.Join(root, "worker.json")
	config := contract.WorkerConfig{}
	if err := remoteio.ReadPrivate(configPath, &config); os.IsNotExist(err) {
		token, err := remoteio.Secret()
		if err != nil {
			return err
		}
		config = contract.WorkerConfig{Schema: contract.Version, Endpoint: invitation.Endpoint, Node: invitation.Node, Token: token, Profiles: profiles}
		if err := remoteio.WritePrivate(configPath, config); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if config.Endpoint != invitation.Endpoint || config.Node != invitation.Node {
		return errors.New("worker is already bound to another invitation")
	}
	if !reflect.DeepEqual(config.Profiles, profiles) {
		return errors.New("worker profiles differ from the saved pairing; inspect the local worker configuration before changing authority")
	}
	client, err := remoteio.NewClient(config.Endpoint, "")
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	return client.Call(ctx, "/v1/pair", map[string]string{"node": config.Node, "secret": invitation.Secret, "token": config.Token}, nil)
}

func saveResult(output string, job contract.Job) error {
	if job.Result == nil {
		return errors.New("job has no result yet")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return errors.New("result output must be a new directory")
	}
	if err := remoteio.PrivateDirectory(output); err != nil {
		return err
	}
	if err := remoteio.WritePrivate(filepath.Join(output, "result.json"), job); err != nil {
		return err
	}
	for name, data := range job.Result.Artifacts {
		if !remoteio.SafeRelative(name) {
			return errors.New("invalid result artifact path")
		}
		path := filepath.Join(output, "artifacts", filepath.FromSlash(name))
		if err := remoteio.PrivateDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(output, "workflow.log"), []byte(job.Result.Log), 0o600)
}
