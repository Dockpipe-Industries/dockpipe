package hostdependencies

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/application/internal/operationids"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func (service Service) installMissing(missing []missingHostDependency, approveSystemChanges bool) error {
	for _, missingDep := range missing {
		dep := missingDep.Dep
		installCmd := InstallCommandForCurrentPlatform(dep)
		if strings.TrimSpace(installCmd) == "" {
			logDependencyResult("dependency.host.install", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
				"platform":    CurrentPlatform(),
				"result":      "skip",
				"skip_reason": "no_installer",
			}), "")
			continue
		}
		if !PlatformsSupportCurrent(missingDep.Platforms) {
			logDependencyResult("dependency.host.install", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
				"platform":    CurrentPlatform(),
				"result":      "skip",
				"skip_reason": "unsupported_platform",
			}), "")
			continue
		}
		approved, err := service.approveInstall(dep, installCmd, approveSystemChanges)
		if err != nil {
			return err
		}
		if !approved {
			continue
		}
		ids := operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
			"platform": CurrentPlatform(),
		})
		// The installer inherits the terminal and may prompt without a trailing newline.
		// Background progress output would obscure those prompts, including passwords.
		err = infrastructure.RunOperationWithOptions(os.Stderr, "dependency.host.install", "Installing host dependency…", ids, infrastructure.OperationOptions{Spinner: false}, func() error {
			fmt.Fprintln(os.Stderr, "[dockpipe] Running dependency installer. Follow its prompts below; password input may be hidden.")
			ids["result"] = "installed"
			return service.adapters.RunInstaller(installCmd)
		})
		if err != nil {
			return fmt.Errorf("install dependency %s: %w", hostDependencyID(dep), err)
		}
	}
	return nil
}

func (service Service) approveInstall(dep domain.HostDependency, installCmd string, approveSystemChanges bool) (bool, error) {
	if approveSystemChanges {
		logDependencyResult("dependency.host.approval", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
			"approval":        "approved",
			"approval_source": "flag",
			"platform":        CurrentPlatform(),
		}), "")
		return true, nil
	}
	if envBool(os.Getenv("DOCKPIPE_APPROVE_PROMPTS")) {
		logDependencyResult("dependency.host.approval", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
			"approval":        "approved",
			"approval_source": "env",
			"platform":        CurrentPlatform(),
		}), "")
		return true, nil
	}
	if !service.adapters.TerminalAvailable() {
		logDependencyResult("dependency.host.approval", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
			"approval":        "declined",
			"approval_source": "non_interactive",
			"platform":        CurrentPlatform(),
		}), "")
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Missing dependency %s (%s).\n", hostDependencyID(dep), hostDependencyCommand(dep))
	fmt.Fprintf(os.Stderr, "[dockpipe] Installer from workflow/package metadata:\n  %s\n", installCmd)
	fmt.Fprintf(os.Stderr, "Run this installer now? [y/N]: ")
	br := bufio.NewReader(os.Stdin)
	line, _ := br.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	approved := line == "y" || line == "yes"
	approval := "declined"
	if approved {
		approval = "approved"
	}
	logDependencyResult("dependency.host.approval", infrastructure.OperationStatusDone, operationids.Merge(hostDependencyResultIDs(dep), map[string]string{
		"approval":        approval,
		"approval_source": "prompt",
		"platform":        CurrentPlatform(),
	}), "")
	return approved, nil
}

func envBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
