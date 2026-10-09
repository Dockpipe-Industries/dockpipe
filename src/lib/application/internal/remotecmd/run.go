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
  setup   --resolver <name> [--hostname <hostname>]  Package-owned remote setup
  info                                           Show configuration without credentials
  init    [--listen 127.0.0.1:47831]               Initialize a private broker
  serve                                          Run broker and configured edge
  pairing-open                                   Accept pairing requests for 15 minutes
  pairings                                       Show pending verification codes
  approve --code <code>                          Approve the code shown on your worker
  deny    --code <code>                          Reject a pairing request
  pairing-close                                  Close pairing and deny pending requests
  nodes                                          List enrolled machines (not live presence)
  pair    --endpoint <https-origin> --node <name> --allow-delivery
                                                 Request approval without transferring files
  invite  --node <name> --out <private-file>       Issue a 15-minute pairing file
  pair    --invite <file> --allow-delivery        Trust this broker to send workflows
          [--profiles <json-file>]                Or approve installed profiles
  worker                                         Run the outbound worker
  submit  --node <name> --profile <name> --id <id> Submit once; same ID is idempotent
  submit  --node <name> --workflow-file <path> --id <id>  Deliver local sources
          [--include <path>] [--dependency <package-dir>] [--artifact <path>]
          [--workdir <directory>] [--dry-run]     Preview files and digest without sending
          [--expected-digest <sha256>]            Require the reviewed snapshot
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
	resolver := flags.String("resolver", "", "remote access resolver profile")
	hostname := flags.String("hostname", "", "public edge hostname")
	node := flags.String("node", "", "worker name")
	profile := flags.String("profile", "", "worker-approved workflow profile")
	id := flags.String("id", "", "stable job ID or node for revoke")
	out := flags.String("out", "", "private output destination")
	endpoint := flags.String("endpoint", "", "broker HTTPS origin for online pairing")
	code := flags.String("code", "", "verification code displayed on the worker")
	invite := flags.String("invite", "", "pairing file")
	profiles := flags.String("profiles", "", "locally approved profiles file")
	allowDelivery := flags.Bool("allow-delivery", false, "approve execution of operator-supplied code as this user")
	timeout := flags.Int("timeout", 3600, "worker-local delivery timeout in seconds")
	workflow := flags.String("workflow-file", "", "local workflow to deliver with its source directory")
	workdir := flags.String("workdir", ".", "source project directory")
	expectedDigest := flags.String("expected-digest", "", "require the exact previously previewed bundle digest")
	dryRun := flags.Bool("dry-run", false, "preview delivery files and digest without submitting")
	var includes, dependencies, artifacts stringList
	flags.Var(&includes, "include", "additional source path inside workdir (repeatable)")
	flags.Var(&dependencies, "dependency", "unpacked package directory to deliver (repeatable)")
	flags.Var(&artifacts, "artifact", "exact result file relative to delivered workdir (repeatable)")
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
	if (*dryRun || *expectedDigest != "") && command != "submit" {
		return errors.New("--dry-run and --expected-digest are supported only by remote submit")
	}
	if (*workflow != "" || len(includes)+len(dependencies)+len(artifacts) != 0) && command != "submit" {
		return errors.New("workflow delivery options are supported only by remote submit")
	}
	if *endpoint != "" && command != "pair" {
		return errors.New("--endpoint is supported only by remote pair")
	}
	if *code != "" && command != "approve" && command != "deny" {
		return errors.New("--code is supported only by remote approve or deny")
	}
	if *allowDelivery && command != "pair" {
		return errors.New("--allow-delivery must be approved on the worker with remote pair")
	}
	root, err := stateRoot(*state)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Setup hands the terminal to dependency installers and provider authentication.
	// An outer spinner would overwrite their prompts even when nested operations are quiet.
	options := infrastructure.OperationOptions{Spinner: command != "setup" && command != "pair"}
	return infrastructure.RunOperationWithOptions(os.Stderr, "remote."+command, "Remote "+command, nil, options, func() error {
		switch command {
		case "info":
			info, err := describeRemote(root)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(info)
		case "init":
			_, err := initialize(root, *listen)
			return err
		case "setup":
			return setup(ctx, root, *listen, *resolver, *hostname, *workdir, checkDependencies)
		case "serve":
			return serve(ctx, root)
		case "pairing-open", "pairing-close", "pairings", "approve", "deny":
			return pairingOperator(ctx, root, command, *code)
		case "pair":
			if *endpoint != "" {
				if *invite != "" || *profiles != "" {
					return errors.New("online pairing cannot be combined with invitation or profiles files")
				}
				return pairOnline(ctx, root, *endpoint, *node, *allowDelivery, *timeout)
			}
			return pairWithDelivery(ctx, root, *invite, *profiles, *allowDelivery, *timeout)
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
		case "submit":
			if *workflow != "" {
				if *profile != "" {
					return errors.New("choose --workflow-file or --profile, not both")
				}
				return submitDelivery(ctx, root, *node, *id, *workdir, *workflow, includes, dependencies, artifacts, *dryRun, *expectedDigest)
			}
			if *dryRun || *expectedDigest != "" || len(includes)+len(dependencies)+len(artifacts) != 0 {
				return errors.New("delivery options require --workflow-file")
			}
			return operator(ctx, root, command, *node, *profile, *id, *out)
		case "invite", "nodes", "jobs", "result", "cancel", "revoke":
			return operator(ctx, root, command, *node, *profile, *id, *out)
		default:
			return fmt.Errorf("unknown remote command %q\n%s", command, usage)
		}
	})
}

func operator(ctx context.Context, root, command, node, profile, id, output string) error {
	client, err := operatorClient(root)
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
		info, err := describeRemote(root)
		if err != nil {
			return err
		}
		if info.Broker == nil {
			return errors.New("broker connection is not configured")
		}
		invitation.Endpoint = info.Broker.Endpoint
		if err := remoteio.WritePrivate(output, invitation); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Invitation saved to %s (expires in 15 minutes). Transfer it privately to %s. There, run: dockpipe remote pair --invite <private-file> --allow-delivery.\n", output, node)
		return nil
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

func pairWithDelivery(ctx context.Context, root, invitePath, profilesPath string, allowDelivery bool, timeout int) error {
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
	if profilesPath != "" {
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
	}
	var delivery *contract.DeliveryPermission
	if allowDelivery {
		delivery = &contract.DeliveryPermission{TimeoutSeconds: timeout}
	}
	if err := remoteio.ValidateWorkerAuthority(contract.WorkerConfig{Profiles: profiles, Delivery: delivery}); err != nil {
		return fmt.Errorf("approve --allow-delivery or provide --profiles: %w", err)
	}
	configPath := filepath.Join(root, "worker.json")
	config := contract.WorkerConfig{}
	if err := remoteio.ReadPrivate(configPath, &config); os.IsNotExist(err) {
		token, err := remoteio.Secret()
		if err != nil {
			return err
		}
		config = contract.WorkerConfig{Schema: contract.Version, Endpoint: invitation.Endpoint, Node: invitation.Node, Token: token, Profiles: profiles, Delivery: delivery}
		if err := remoteio.WritePrivate(configPath, config); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if config.Endpoint != invitation.Endpoint || config.Node != invitation.Node {
		return errors.New("worker is already bound to another invitation")
	}
	if !reflect.DeepEqual(config.Profiles, profiles) || !reflect.DeepEqual(config.Delivery, delivery) {
		return errors.New("worker authority differs from the saved pairing; inspect the local worker configuration before changing authority")
	}
	client, err := remoteio.NewClient(config.Endpoint, "")
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	if err := client.Call(ctx, "/v1/pair", map[string]string{"node": config.Node, "secret": invitation.Secret, "token": config.Token}, nil); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Paired worker %s. Next: dockpipe remote worker --state %q (foreground), or dockpipe remote service --role worker --state %q.\n", config.Node, root, root)
	if delivery != nil {
		fmt.Fprintln(os.Stderr, "Delivery enabled: this broker may execute supplied workflows with your user permissions. It is not a sandbox.")
	}
	return nil
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
