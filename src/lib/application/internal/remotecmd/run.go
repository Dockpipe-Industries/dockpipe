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
	"strings"
	"syscall"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

const usage = `dockpipe remote <command> [options]
	unlock                                         Authenticate with the OS to unlock remote features
	lock                                           Immediately lock new operator remote actions
	security-status                                Show the global remote policy and capabilities
	security-policy [--idle-seconds <60..43200>]     Update global remote auto-lock policy
	                [--absolute-seconds <60..43200>]
	                [--lock-on-screen-lock=true] [--lock-on-sleep=true]
  setup   --resolver <name> [--hostname <hostname>]  Package-owned remote setup
  edge-activate --request <json> --dry-run|--approve <id>  Supervised local edge activation
  edge-status --id <activation-id>                  Inspect activation and recovery health
  info                                           Show configuration without credentials
  init    [--listen 127.0.0.1:47831]               Initialize a private broker
  serve                                          Run broker and configured edge
  security-key-register --attestation-roots <pem>  Register a physical pairing key locally (broker stopped)
  pairing-open                                   Accept pairing requests for 15 minutes
  pairings                                       Show pending verification codes
  approve --code <code>                          Verify hardware key and approve the worker code
  deny    --code <code>                          Reject a pairing request
  pairing-close                                  Close pairing and deny pending requests
  nodes                                          List enrolled machines (not live presence)
  pair    --endpoint <https-origin> --node <name> [--allow-delivery] [--allow-features]
                                                 Request approval without transferring files
  invite  --node <name> --out <private-file>       Disabled; use hardware-key pairing
  pair    --invite <file> --allow-delivery        Trust this broker to send workflows
          [--profiles <json-file>]                Or approve installed profiles
  worker                                         Run the outbound worker
  job-log --id <job>                             Read the bounded live workflow log preview
  monitoring --system --docker                    Opt in locally on a paired worker
             --disable                           Stop sharing machine snapshots
  observe --node <name>                          Read the latest authorized machine snapshot
  desktop-config [--request <json|->]           Inspect or replace independent worker desktop grants
  desktop-catalog                               List native provider readiness and desktop identity
  desktop-prepare --request <json|->             Prepare exact native desktop reference locally
  desktop-open|desktop-status|desktop-pulse|desktop-signal|desktop-close --request <json|->
                                                Bounded native desktop control; never carries media/input
  desktop --stdio --request <intent-file>        Private viewer signaling bridge
  desktop-worker --stdio                        Private native provider signaling bridge
  desktop-cleanup --stdio --request <private-file> Exact revoked desktop cleanup with native proof
  desktop-relay --stdio --request <private-file> Opaque binary relay for an existing desktop session
  desktop-administrator-approval|desktop-administrator-grant|desktop-administrator-service --request <json|->
  desktop-administrator-relay --stdio --request <private-file> Dedicated administrator input relay
  network-config [--request <private-file>]       Inspect or replace independent TCP network grants
  network-catalog                                List live paired TCP network targets
  network-trust --request <private-file>           Read exact authenticated public TLS anchors
  network-proxy --stdio --request <private-file>   Authenticated loopback HTTP/CONNECT proxy
  network-worker --stdio                         Paired worker TCP service
  features                                       List authorized live feature catalogs
  feature-prepare --request <json-file|->         Validate one intent locally and emit its metadata reference
  feature-call --request <json-file|->            Submit one fixed feature intent and await its result
  feature-status --request <json-file|->          Inspect the captured feature operation without replay
  feature-cancel --request <json-file|->          Request cancellation of the captured operation
  feature-reconcile --request <json-file|->       Revoke a mutation and verify its native cleanup evidence
          [--recovery verify|retire-container]    Retirement requires independent worker recovery permission
          [--expected-container <full-id>]        Required exact reviewed resource for container retirement
  features-config [--request <json-file|-> | --terminal-home]
                                                 Inspect or configure worker-local feature permissions
  terminal --request <intent-file>              Private interactive terminal NDJSON transport
  terminal-status --request <reference-file>    Inspect the captured terminal session
  terminal-close --request <reference-file>     Request closure of the captured terminal session
  submit  --node <name> --profile <name> --id <id> Submit once; same ID is idempotent
  submit  --node <name> --workflow-file <path> --id <id>  Deliver local sources
          [--include <path>] [--dependency <package-dir>] [--artifact <path>]
          [--workdir <directory>] [--dry-run]     Preview files and digest without sending
          [--expected-digest <sha256>]            Require the reviewed snapshot
  jobs                                           List job states
  result  --id <id> [--out <new-private-directory>] Inspect/download a result
  job-interruption-inspect --id <id> --node <name>  Review one retained unknown result
  job-interruption-acknowledge --id <id> --node <name> --review-token <sha256>
                                                 Release one inspected assignment hold
  cancel  --id <id>                               Request cancellation
  revoke  --id <node>                             Revoke a worker
  guest-grant --request <file>                     Authorize a host to enroll disposable workflow workers
  guest-grant-revoke --id <grant>                   End all worker authority granted for the run
  guest-prepare --request <file>                   Generate a disposable guest identity and print its public claim
  guest-enroll --request <file>                    Enroll the prepared guest using the authorized host identity
  guest-release --id <grant>                       End the host's guest grant during teardown
  remove  --id <node>                             Remove a saved worker enrollment
  service --role broker|worker                    Install a user service; --replace updates and restarts it
All commands accept --state <directory>. Remote nodes currently target Linux/macOS.
Locked operator actions re-authenticate before sending. Use --auth never for unattended
or background commands; denial never retries an HTTP request or changes worker execution.
Hardware-key commands use native FIDO2: --interaction terminal (default) or gui.
The GUI uses private stdin/stdout interaction; PINs must never be passed as arguments.
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
	activationRequest := flags.String("request", "", "exact edge activation or typed feature JSON request; - reads feature stdin")
	terminalHome := flags.Bool("terminal-home", false, "grant Terminal on this paired worker, starting in the current account's home folder")
	desktopStdio := flags.Bool("stdio", false, "private bridge stdin/stdout transport and lifetime")
	prepareTerminal := flags.Bool("prepare", false, "require canonical prepared-reference acknowledgment before host terminal opening")
	activationApproval := flags.String("approve", "", "exact activation approval")
	recoveryChoice := flags.String("recovery", "verify", "feature recovery strategy: verify or retire-container (explicitly removes the exact container, without volumes/images)")
	expectedContainer := flags.String("expected-container", "", "exact reviewed full container ID for explicit retirement")
	state := flags.String("state", "", "private state directory")
	authentication := flags.String("auth", "auto", "OS re-authentication for locked operator actions: auto or never")
	attestationRoots := flags.String("attestation-roots", "", "verified hardware-key manufacturer CA PEM bundle (first key only)")
	interaction := flags.String("interaction", "terminal", "native hardware-key interaction: terminal or gui")
	listen := flags.String("listen", "127.0.0.1:47831", "loopback origin address")
	resolver := flags.String("resolver", "", "remote access resolver profile")
	hostname := flags.String("hostname", "", "public edge hostname")
	node := flags.String("node", "", "worker name")
	profile := flags.String("profile", "", "worker-approved workflow profile")
	id := flags.String("id", "", "stable job ID or node for revoke/remove")
	reviewToken := flags.String("review-token", "", "exact job interruption token from operator inspection")
	out := flags.String("out", "", "private output destination")
	endpoint := flags.String("endpoint", "", "broker HTTPS origin for online pairing")
	code := flags.String("code", "", "verification code displayed on the worker")
	invite := flags.String("invite", "", "pairing file")
	profiles := flags.String("profiles", "", "locally approved profiles file")
	allowDelivery := flags.Bool("allow-delivery", false, "approve execution of operator-supplied code as this user")
	allowFeatures := flags.Bool("allow-features", false, "opt in to separately configured managed features without granting workflow delivery")
	monitorSystem := flags.Bool("system", false, "share system resources and process identities with this worker's broker")
	monitorDocker := flags.Bool("docker", false, "share read-only Docker inventory with this worker's broker")
	disableMonitoring := flags.Bool("disable", false, "disable worker monitoring")
	timeout := flags.Int("timeout", 3600, "worker-local delivery timeout in seconds")
	workflow := flags.String("workflow-file", "", "local workflow to deliver with its source directory")
	workdir := flags.String("workdir", ".", "source project directory")
	expectedDigest := flags.String("expected-digest", "", "require the exact previously previewed bundle digest")
	expectedFingerprint := flags.String("expected-fingerprint", "", "require the captured paired machine fingerprint for workflow delivery")
	expectedGeneration := flags.String("expected-generation", "", "require the captured feature worker generation for workflow delivery")
	expectedRoot := flags.String("expected-root", "", "retain the captured root grant identity for workflow delivery")
	dryRun := flags.Bool("dry-run", false, "preview delivery files and digest without submitting")
	var includes, dependencies, artifacts stringList
	flags.Var(&includes, "include", "additional source path inside workdir (repeatable)")
	flags.Var(&dependencies, "dependency", "unpacked package directory to deliver (repeatable)")
	flags.Var(&artifacts, "artifact", "exact result file relative to delivered workdir (repeatable)")
	replaceService := flags.Bool("replace", false, "replace the user-service definition and restart it")
	role := flags.String("role", "", "broker or worker service")
	idleSeconds := flags.Int("idle-seconds", 300, "global remote inactivity timeout in seconds (60..43200)")
	absoluteSeconds := flags.Int("absolute-seconds", 3600, "global remote absolute session lifetime in seconds (60..43200)")
	lockOnScreenLock := flags.Bool("lock-on-screen-lock", true, "lock remote access when the OS session locks")
	lockOnSleep := flags.Bool("lock-on-sleep", true, "lock remote access after sleep/resume")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	desktopWorkerService := command == "service-worker-desktop"
	if *desktopStdio && command != "desktop" && command != "desktop-worker" && command != "desktop-relay" && command != "desktop-administrator-relay" && command != "desktop-cleanup" && command != "network-proxy" && command != "network-worker" && !desktopWorkerService {
		return errors.New("--stdio requires a private bridge or desktop service")
	}
	if (command == "network-proxy" || command == "network-worker") && !*desktopStdio {
		return errors.New("network bridges require --stdio private lifetime transport")
	}
	if (command == "desktop" || command == "desktop-worker" || command == "desktop-relay" || command == "desktop-administrator-relay" || command == "desktop-cleanup") && !*desktopStdio {
		return errors.New("desktop bridges require --stdio private signaling transport")
	}
	if *replaceService && command != "service" {
		return errors.New("--replace is supported only by remote service")
	}
	if *prepareTerminal && command != "terminal" {
		return errors.New("--prepare is supported only by remote terminal")
	}
	if *terminalHome && (command != "features-config" || *activationRequest != "") {
		return errors.New("--terminal-home requires features-config without --request")
	}
	if (*recoveryChoice != "verify" && *recoveryChoice != "retire-container") || *recoveryChoice != "verify" && command != "feature-reconcile" {
		return errors.New("--recovery requires feature-reconcile and verify or retire-container")
	}
	if (*expectedContainer != "" || *recoveryChoice == "retire-container") && (command != "feature-reconcile" || *recoveryChoice != "retire-container" || len(*expectedContainer) != 64 || strings.Trim(*expectedContainer, "0123456789abcdef") != "") {
		return errors.New("container retirement requires --expected-container with its exact reviewed full ID")
	}
	if *authentication != "auto" && *authentication != "never" {
		return errors.New("--auth must be auto or never")
	}
	securityChanges := map[string]bool{}
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == "idle-seconds" || flag.Name == "absolute-seconds" || flag.Name == "lock-on-screen-lock" || flag.Name == "lock-on-sleep" {
			securityChanges[flag.Name] = true
		}
	})
	if len(securityChanges) != 0 && command != "security-policy" {
		return errors.New("remote lock policy flags are supported only by remote security-policy")
	}
	if (*monitorSystem || *monitorDocker || *disableMonitoring) && command != "monitoring" {
		return errors.New("--system, --docker and --disable are supported only by remote monitoring")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected remote positional arguments")
	}
	var capturedTarget contract.FeatureTarget
	if *expectedFingerprint != "" || *expectedGeneration != "" || *expectedRoot != "" {
		capturedTarget = contract.FeatureTarget{Node: *node, Fingerprint: *expectedFingerprint, Generation: *expectedGeneration}
		if command != "submit" || *workflow == "" || capturedTarget.Validate() != nil || (*expectedRoot != "" && !contract.ValidID(*expectedRoot)) {
			return errors.New("captured target flags require workflow delivery and an exact valid machine identity")
		}
	}
	if *expectedDigest != "" && command != "submit" && !desktopWorkerService {
		return errors.New("--expected-digest is supported only by submit")
	}
	if *activationApproval != "" && command != "edge-activate" {
		return errors.New("--approve requires edge-activate")
	}
	if *reviewToken != "" && command != "job-interruption-acknowledge" {
		return errors.New("--review-token requires job-interruption-acknowledge")
	}
	if *activationRequest != "" && command != "edge-activate" && command != "feature-prepare" && command != "feature-call" && command != "feature-status" && command != "feature-cancel" && command != "feature-reconcile" && command != "features-config" && command != "terminal" && command != "terminal-status" && command != "terminal-close" && !desktopRequestCommand(command) && command != "network-config" && command != "network-proxy" && command != "network-trust" && !guestRequestCommand(command) && !desktopWorkerService {
		return errors.New("--request requires edge-activate or an explicit feature request command")
	}
	if *dryRun && command != "submit" && command != "edge-activate" {
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
	if *allowFeatures && (command != "pair" || *endpoint == "") {
		return errors.New("--allow-features requires online remote pair with --endpoint")
	}
	if *attestationRoots != "" && command != "security-key-register" {
		return errors.New("--attestation-roots is supported only by remote security-key-register")
	}
	if *interaction != "terminal" && *interaction != "gui" {
		return errors.New("--interaction must be terminal or gui")
	}
	if *interaction != "terminal" && command != "approve" && command != "security-key-register" {
		return errors.New("--interaction is supported only by remote approve or security-key-register")
	}
	if command == "desktop-prepare" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return prepareDesktop(ctx, *activationRequest, os.Stdout)
	}
	if command == "feature-prepare" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return prepareFeature(ctx, *activationRequest, os.Stdout)
	}
	root, err := stateRoot(*state)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = context.WithValue(ctx, hardwareInteractionModeKey{}, *interaction)
	ctx = context.WithValue(ctx, remoteAuthenticationKey{}, *authentication == "auto")
	// Setup hands the terminal to dependency installers and provider authentication.
	// An outer spinner would overwrite their prompts even when nested operations are quiet.
	options := infrastructure.OperationOptions{Spinner: command != "setup" && command != "pair" && command != "approve" && command != "security-key-register" && command != "unlock" && command != "security-policy" && command != "terminal" && command != "desktop" && command != "desktop-worker" && command != "desktop-relay" && command != "desktop-administrator-relay" && command != "desktop-cleanup" && command != "network-proxy" && command != "network-worker"}
	if *authentication == "auto" || desktopWorkerService {
		options.Spinner = false
	}
	return infrastructure.RunOperationWithOptions(os.Stderr, "remote."+command, "Remote "+command, nil, options, func() error {
		switch command {
		case "guest-grant", "guest-grant-revoke", "guest-prepare", "guest-enroll", "guest-release":
			return guestCommand(ctx, root, command, *activationRequest, *id)
		case "network-config", "network-catalog", "network-proxy", "network-worker", "network-trust":
			return networkCommand(ctx, root, command, *activationRequest)
		case "desktop":
			return desktopViewer(ctx, root, *activationRequest)
		case "desktop-administrator-relay":
			return desktopAdministratorRelayBridge(ctx, root, *activationRequest)
		case "desktop-administrator-approval", "desktop-administrator-grant", "desktop-administrator-service":
			return desktopAdministratorCommand(ctx, root, command, *activationRequest)
		case "desktop-relay":
			return desktopRelayBridge(ctx, root, *activationRequest)
		case "desktop-cleanup":
			return desktopCleanupBridge(ctx, root, *activationRequest)
		case "desktop-worker":
			return desktopWorkerBridge(ctx, root)
		case "desktop-config", "desktop-catalog", "desktop-open", "desktop-status", "desktop-pulse", "desktop-signal", "desktop-close":
			return desktopCommand(ctx, root, command, *activationRequest)
		case "edge-activate":
			return activateEdge(ctx, root, *activationRequest, *activationApproval, *dryRun)
		case "edge-status":
			return edgeStatus(ctx, root, *id)
		case "unlock", "lock", "security-status", "security-policy":
			return remoteSecurity(ctx, command, securityChanges, *idleSeconds, *absoluteSeconds, *lockOnScreenLock, *lockOnSleep)
		case "security-key-register":
			return registerPairingKey(ctx, root, *attestationRoots)
		case "monitoring":
			return configureMonitoring(root, *monitorSystem, *monitorDocker, *disableMonitoring)
		case "observe":
			return observeRemote(ctx, root, *node)
		case "features":
			return featureCatalog(ctx, root)
		case "feature-call":
			return callFeature(ctx, root, *activationRequest)
		case "feature-status", "feature-cancel":
			return inspectFeature(ctx, root, *activationRequest, command == "feature-cancel")
		case "feature-reconcile":
			return reconcileFeature(ctx, root, *activationRequest, *recoveryChoice, *expectedContainer)
		case "features-config":
			if *terminalHome {
				return configureWorkerFeatures(ctx, root, "", true)
			}
			return configureFeatures(ctx, root, *activationRequest)
		case "terminal":
			return remoteTerminal(ctx, root, *activationRequest, *prepareTerminal)
		case "terminal-status", "terminal-close":
			return inspectTerminal(ctx, root, *activationRequest, command == "terminal-close")
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
				return pairOnlineWithFeatures(ctx, root, *endpoint, *node, *allowDelivery, *allowFeatures, *timeout)
			}
			return pairWithDelivery(ctx, root, *invite, *profiles, *allowDelivery, *timeout)
		case "worker":
			var config contract.WorkerConfig
			if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
				return err
			}
			executable, err := process.Executable()
			if err != nil {
				return err
			}
			return remoteio.RunWorker(ctx, root, executable, config)
		case "service-worker-desktop":
			return superviseDesktopWorker(ctx, root, *activationRequest, *expectedDigest, *desktopStdio)
		case "service":
			return installServiceWithReplacement(ctx, root, *role, *replaceService)
		case "submit":
			if *workflow != "" {
				if *profile != "" {
					return errors.New("choose --workflow-file or --profile, not both")
				}
				return submitCapturedDelivery(ctx, root, *node, *id, *workdir, *workflow, includes, dependencies, artifacts, *dryRun, *expectedDigest, capturedTarget, *expectedRoot)
			}
			if *dryRun || *expectedDigest != "" || len(includes)+len(dependencies)+len(artifacts) != 0 {
				return errors.New("delivery options require --workflow-file")
			}
			return operator(ctx, root, command, *node, *profile, *id, *out)
		case "invite", "nodes", "jobs", "result", "job-log", "cancel", "revoke", "remove":
			return operator(ctx, root, command, *node, *profile, *id, *out)
		case "job-interruption-inspect", "job-interruption-acknowledge":
			request := contract.JobInterruptionRequest{ID: *id, Node: *node, ReviewToken: *reviewToken}
			return reviewJobInterruption(ctx, root, request, command == "job-interruption-acknowledge")
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
	case "job-log":
		if !contract.ValidID(id) {
			return errors.New("live log preview requires an exact --id")
		}
		var preview contract.LiveLogView
		if err := client.Call(ctx, "/v1/job-log", map[string]string{"id": id}, &preview); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(preview)
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
	unlock, err := remoteio.LockWorker(root)
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
