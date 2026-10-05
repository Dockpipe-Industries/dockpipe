package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"dockpipe.vm/tools/internal/executor"
	"dockpipe.vm/tools/internal/identitymaterial"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
	"dockpipe.vm/tools/internal/xdg"
)

const version = "1.3.4"

func main() {
	manifestPath := flag.String("validate-manifest", "", "validate an offline qualification manifest")
	planRuntime := flag.String("plan-runtime-dir", "", "print an inert QEMU argv plan for the validated manifest")
	configurationSHA256 := flag.Bool("configuration-sha256", false, "print the deterministic qualification configuration SHA-256")
	provisioningPath := flag.String("plan-provisioning", "", "print the exact inert provisioning plan for one qualification instance")
	liveAuthorizationPath := flag.String("live-authorization", "", "validate a distinct short-lived authorization against the exact inert plan")
	prepareIdentityPath := flag.String("prepare-identity-material", "", "exclusively create a durable pre-authorization identity bundle")
	runID := flag.String("run-id", "", "exact qualification run ID for identity preparation")
	cohortID := flag.String("cohort-id", "", "exact qualification cohort ID for identity preparation")
	executeQualification := flag.Bool("execute-qualification", false, "execute the exact authorized typed Linux qualification plan")
	identityMaterialPath := flag.String("identity-material", "", "load the exact prepared identity-material bundle")
	cleanupExecutorPath := flag.String("cleanup-executor", "", "load a preserved executor contract for separately authorized exact cleanup")
	cleanupAuthorizationPath := flag.String("cleanup-authorization", "", "load the fresh exact cleanup authorization")
	gate3ExecutorPath := flag.String("gate3-executor", "", "load the exact qualified executor for inert Gate 3 planning")
	gate3ReconstitutePath := flag.String("gate3-reconstitute-executor", "", "read-only export of proof-bound Gate 3 planning inputs from a qualified executor")
	gate3ReconstitutionPath := flag.String("gate3-reconstitution", "", "load proof-bound historical inputs for inert Gate 3 planning")
	gate3ProvisioningPath := flag.String("gate3-provisioning", "", "load the exact provisioning contract for Gate 3")
	gate3ManifestPath := flag.String("gate3-manifest", "", "load the exact qualification manifest for Gate 3")
	gate3PlanPath := flag.String("gate3-plan", "", "load the exact inert Gate 3 plan for execution")
	gate3AuthorizationPath := flag.String("gate3-authorization", "", "load the fresh exact Gate 3 authorization")
	gate3TokenPath := flag.String("gate3-token", "", "load the owner-only destructive Gate 3 token")
	executeGate3 := flag.Bool("execute-gate3", false, "execute the exact authorized Gate 3 cohort")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("dockpipe-qemu-controller", version)
		return
	}
	if *gate3ReconstitutePath != "" {
		if *manifestPath != "" || *planRuntime != "" || *configurationSHA256 || *provisioningPath != "" || *liveAuthorizationPath != "" || *prepareIdentityPath != "" || *runID != "" || *cohortID != "" || *executeQualification || *identityMaterialPath != "" || *cleanupExecutorPath != "" || *cleanupAuthorizationPath != "" || *gate3ExecutorPath != "" || *gate3ReconstitutionPath != "" || *gate3ProvisioningPath != "" || *gate3ManifestPath != "" || *gate3PlanPath != "" || *gate3AuthorizationPath != "" || *gate3TokenPath != "" || *executeGate3 {
			fatal("Gate 3 reconstitution is a separate read-only operation")
		}
		reconstitution, err := executor.ReconstituteGate3(*gate3ReconstitutePath)
		if err != nil {
			fatal(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(reconstitution); err != nil {
			fatal(err.Error())
		}
		return
	}
	gate3InputWithoutExecutor := *gate3ExecutorPath == "" && (*gate3ReconstitutionPath != "" || *gate3ProvisioningPath != "" || *gate3ManifestPath != "" || *gate3PlanPath != "" || *gate3AuthorizationPath != "" || *gate3TokenPath != "" || *executeGate3)
	if gate3InputWithoutExecutor {
		fatal("Gate 3 inputs require --gate3-executor")
	}
	if *prepareIdentityPath != "" {
		if *manifestPath != "" || *configurationSHA256 || *provisioningPath != "" || *liveAuthorizationPath != "" || *executeQualification || *identityMaterialPath != "" || *cleanupExecutorPath != "" || *cleanupAuthorizationPath != "" || *gate3ExecutorPath != "" || *gate3ReconstitutionPath != "" || *executeGate3 {
			fatal("identity preparation is a separate offline operation")
		}
		paths, checkout := environment()
		descriptor, err := identitymaterial.Prepare(*prepareIdentityPath, checkout, *runID, *cohortID, provisioning.Roots{Instances: paths.Instances, Evidence: paths.Evidence, Config: paths.Config, Runtime: paths.Runtime}, time.Now())
		if err != nil {
			fatal(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(descriptor); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *gate3ExecutorPath != "" {
		if *cleanupExecutorPath != "" || *prepareIdentityPath != "" || *manifestPath != "" || *provisioningPath != "" || *liveAuthorizationPath != "" || *executeQualification || *identityMaterialPath != "" || *cleanupAuthorizationPath != "" || *configurationSHA256 || *planRuntime != "" {
			fatal("Gate 3 is a separate exact executor operation")
		}
		execution, executorFileSHA256, err := executor.LoadWithSHA256(*gate3ExecutorPath)
		if err != nil {
			fatal(err.Error())
		}
		var derived executor.Gate3Plan
		var contract provisioning.Contract
		var qualification manifest.Manifest
		if *gate3ReconstitutionPath != "" {
			if *gate3ProvisioningPath != "" || *gate3ManifestPath != "" {
				fatal("Gate 3 planning requires either reconstitution or the exact provisioning and manifest inputs")
			}
			if *executeGate3 {
				fatal("reconstituted Gate 3 inputs are inert and cannot execute")
			}
			reconstitution, err := executor.LoadGate3Reconstitution(*gate3ReconstitutionPath, execution, executorFileSHA256)
			if err != nil {
				fatal(err.Error())
			}
			derived, err = executor.BuildGate3PlanFromReconstitution(execution, reconstitution, executorFileSHA256)
			if err != nil {
				fatal(err.Error())
			}
		} else {
			if *gate3ProvisioningPath == "" || *gate3ManifestPath == "" {
				fatal("Gate 3 requires proof-bound reconstitution or the exact provisioning contract and qualification manifest")
			}
			contract, qualification, err = executor.LoadRetainedGate3Inputs(execution, *gate3ProvisioningPath, *gate3ManifestPath)
			if err != nil {
				fatal(err.Error())
			}
			derived, err = executor.BuildGate3Plan(execution, contract, qualification)
			if err != nil {
				fatal(err.Error())
			}
		}
		if !*executeGate3 {
			if *gate3PlanPath != "" || *gate3AuthorizationPath != "" || *gate3TokenPath != "" {
				fatal("inert Gate 3 planning accepts no plan, authorization, or token input")
			}
			if err := json.NewEncoder(os.Stdout).Encode(derived); err != nil {
				fatal(err.Error())
			}
			return
		}
		if *gate3PlanPath == "" || *gate3AuthorizationPath == "" || *gate3TokenPath == "" {
			fatal("live Gate 3 requires the exact plan, authorization, and token")
		}
		plan, err := executor.LoadGate3PlanForExecution(*gate3PlanPath, execution, derived)
		if err != nil {
			fatal("Gate 3 plan does not match the freshly derived inert plan")
		}
		authorization, err := executor.LoadGate3Authorization(*gate3AuthorizationPath)
		if err != nil {
			fatal(err.Error())
		}
		token, err := loadGate3Token(*gate3TokenPath)
		if err != nil {
			fatal(err.Error())
		}
		identityRoot := filepath.Join(contract.Roots.Config, "instances", contract.RunID, contract.CohortID, "identity")
		keys, err := provisioning.LoadReservedKeyMaterial(identityRoot, contract)
		if err != nil {
			fatal(err.Error())
		}
		runner, err := executor.NewGate3LinuxRunner(executor.Gate3RunnerConfig{Plan: plan, Execution: execution, Contract: contract, Manifest: qualification, Keys: keys, Authorization: authorization, Token: token, Now: time.Now})
		if err != nil {
			fatal(err.Error())
		}
		result, err := executor.ExecuteGate3(context.Background(), plan, execution, authorization, time.Now(), runner)
		if err != nil {
			fatal(err.Error())
		}
		if err := executor.StoreGate3Result(plan, result); err != nil {
			fatal(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *cleanupExecutorPath != "" {
		if *cleanupAuthorizationPath == "" || *manifestPath != "" || *configurationSHA256 || *provisioningPath != "" || *liveAuthorizationPath != "" || *executeQualification || *identityMaterialPath != "" {
			fatal("cleanup requires only --cleanup-executor and --cleanup-authorization")
		}
		contract, err := executor.Load(*cleanupExecutorPath)
		if err != nil {
			fatal(err.Error())
		}
		authorization, err := executor.LoadCleanupAuthorization(*cleanupAuthorizationPath)
		if err != nil {
			fatal(err.Error())
		}
		result, err := executor.ExecuteCleanup(context.Background(), contract, authorization, time.Now(), executor.NewCleanupRunner())
		if err != nil {
			fatal(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *executeQualification && (*provisioningPath == "" || *manifestPath == "") {
		fatal("--execute-qualification requires --validate-manifest and --plan-provisioning")
	}
	if *identityMaterialPath != "" && !*executeQualification {
		fatal("--identity-material is valid only with --execute-qualification")
	}
	if *cleanupAuthorizationPath != "" {
		fatal("--cleanup-authorization is valid only with --cleanup-executor")
	}
	if *manifestPath == "" {
		fatal("an explicit version, identity, cleanup, or manifest operation is required")
	}
	m, manifestJSON, err := manifest.LoadWithBytes(*manifestPath)
	if err != nil {
		fatal(err.Error())
	}
	if *configurationSHA256 {
		if *provisioningPath != "" || *planRuntime != "" || *liveAuthorizationPath != "" || *executeQualification || *identityMaterialPath != "" {
			fatal("configuration hashing is a separate offline operation")
		}
		digest, err := manifest.ConfigurationSHA256(m)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println(digest)
		return
	}
	if *provisioningPath != "" {
		if *planRuntime != "" {
			fatal("QEMU argv and provisioning plans must be requested separately")
		}
		contract, provisioningJSON, err := provisioning.LoadWithBytes(*provisioningPath)
		if err != nil {
			fatal(err.Error())
		}
		home, err := os.UserHomeDir()
		if err != nil {
			fatal(err.Error())
		}
		paths, err := xdg.Resolve(home, map[string]string{
			"XDG_CACHE_HOME": os.Getenv("XDG_CACHE_HOME"), "XDG_STATE_HOME": os.Getenv("XDG_STATE_HOME"),
			"XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME"), "XDG_RUNTIME_DIR": os.Getenv("XDG_RUNTIME_DIR"),
		})
		if err != nil {
			fatal(err.Error())
		}
		checkout, err := os.Getwd()
		if err != nil {
			fatal(err.Error())
		}
		plan, err := provisioning.BuildPlan(contract, paths, m, checkout, provisioning.OSImageInspector{})
		if err != nil {
			fatal(err.Error())
		}
		if *liveAuthorizationPath != "" {
			auth, err := provisioning.LoadAuthorization(*liveAuthorizationPath)
			if err != nil {
				fatal(err.Error())
			}
			plan, err = provisioning.AuthorizePlan(plan, contract, auth, time.Now())
			if err != nil {
				fatal(err.Error())
			}
		}
		if *executeQualification {
			if *liveAuthorizationPath == "" || *identityMaterialPath == "" {
				fatal("live execution requires exact live authorization and identity material")
			}
			descriptor, keys, err := identitymaterial.Load(*identityMaterialPath, checkout, contract, time.Now())
			if err != nil {
				fatal(err.Error())
			}
			controllerBinary, err := os.ReadFile(contract.Artifacts.ControllerBinary)
			if err != nil {
				fatal(err.Error())
			}
			guestBinary, err := os.ReadFile(contract.Artifacts.GuestAgentBinary)
			if err != nil {
				fatal(err.Error())
			}
			harnessBinary, err := os.ReadFile(contract.Artifacts.HarnessBinary)
			if err != nil {
				fatal(err.Error())
			}
			execution, err := executor.Build(contract, plan, m, checkout, provisioning.RenderMaterial{Keys: keys, ControllerBinary: controllerBinary, GuestAgentBinary: guestBinary, HarnessBinary: harnessBinary})
			if err != nil {
				fatal(err.Error())
			}
			identityRoot, err := executor.PrepareLiveRoots(contract)
			if err != nil {
				fatal(err.Error())
			}
			if _, err := executor.StoreRetainedGate3Inputs(execution, contract, provisioningJSON, m, manifestJSON); err != nil {
				fatal(err.Error())
			}
			reserved, err := provisioning.ReserveIdentity(identityRoot, contract, keys)
			if err != nil {
				fatal(err.Error())
			}
			configRoot := filepath.Dir(identityRoot)
			if err := executor.Store(filepath.Join(configRoot, "executor.json"), execution); err != nil {
				fatal(err.Error())
			}
			if err := identitymaterial.Consume(*identityMaterialPath, descriptor, reserved); err != nil {
				fatal(err.Error())
			}
			runner, err := executor.NewLinuxRunner(executor.RunnerConfig{Contract: contract, Manifest: m, Keys: keys, Execution: execution, Now: time.Now})
			if err != nil {
				fatal(err.Error())
			}
			result, err := executor.Execute(context.Background(), execution, runner)
			if err != nil {
				fatal(err.Error())
			}
			if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
				fatal(err.Error())
			}
			return
		}
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *liveAuthorizationPath != "" {
		fatal("--live-authorization is valid only with --plan-provisioning")
	}
	if *planRuntime == "" {
		fmt.Println("qualification manifest valid; live execution unavailable")
		return
	}
	plan, err := manifest.PlanQEMU(m, *planRuntime)
	if err != nil {
		fatal(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
		fatal(err.Error())
	}
}

func environment() (xdg.Paths, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal(err.Error())
	}
	paths, err := xdg.Resolve(home, map[string]string{"XDG_CACHE_HOME": os.Getenv("XDG_CACHE_HOME"), "XDG_STATE_HOME": os.Getenv("XDG_STATE_HOME"), "XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME"), "XDG_RUNTIME_DIR": os.Getenv("XDG_RUNTIME_DIR")})
	if err != nil {
		fatal(err.Error())
	}
	checkout, err := os.Getwd()
	if err != nil {
		fatal(err.Error())
	}
	return paths, checkout
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "dockpipe-qemu-controller:", message)
	os.Exit(2)
}

func loadGate3Token(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("Gate 3 token path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("Gate 3 token must be a regular owner-only file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return "", fmt.Errorf("Gate 3 token must be owned by the current user")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := string(data)
	if len(token) != 64 {
		return "", fmt.Errorf("Gate 3 token must contain exactly 64 bytes")
	}
	return token, nil
}
