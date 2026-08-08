package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dockpipe.vm/tools/internal/executor"
	"dockpipe.vm/tools/internal/identitymaterial"
	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
	"dockpipe.vm/tools/internal/xdg"
)

const version = "1.1.3"

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
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("dockpipe-qemu-controller", version)
		return
	}
	if *prepareIdentityPath != "" {
		if *manifestPath != "" || *configurationSHA256 || *provisioningPath != "" || *liveAuthorizationPath != "" || *executeQualification || *identityMaterialPath != "" || *cleanupExecutorPath != "" || *cleanupAuthorizationPath != "" {
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
	m, err := manifest.Load(*manifestPath)
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
		contract, err := provisioning.Load(*provisioningPath)
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
			execution, err := executor.Build(contract, plan, m, checkout, provisioning.RenderMaterial{Keys: keys, ControllerBinary: controllerBinary, GuestAgentBinary: guestBinary})
			if err != nil {
				fatal(err.Error())
			}
			identityRoot, err := executor.PrepareLiveRoots(contract)
			if err != nil {
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
