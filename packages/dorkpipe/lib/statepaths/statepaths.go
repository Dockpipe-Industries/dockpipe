package statepaths

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure"
)

const dorkpipeScope = "dorkpipe"

func PackageRuntimeDir(workdir string) (string, error) {
	return infrastructure.PackageRuntimeDir(workdir, dorkpipeScope)
}

func EditArtifactsDir(workdir, requestID string) (string, error) {
	return packageRuntimePath(workdir, "edit", requestID)
}

func ReasoningArtifactsDir(workdir, requestID string) (string, error) {
	return packageRuntimePath(workdir, "reasoning", requestID)
}

func MetricsPath(workdir string) (string, error) {
	root, _, err := PrepareLearningAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "metrics.jsonl"), nil
}

func TrainingMetricsPath(workdir string) (string, error) {
	root, _, err := PrepareLearningAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "training", "metrics.jsonl"), nil
}

func RunPath(workdir string) (string, error) {
	return packageRuntimePath(workdir, "run.json")
}

func NodesDir(workdir string) (string, error) {
	return packageRuntimePath(workdir, "nodes")
}

func AnalysisDir(workdir string) (string, error) {
	root, _, err := PrepareLearningAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "analysis"), nil
}

func QueuePath(workdir string) (string, error) {
	root, err := AnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "queue.json"), nil
}

func InsightsPath(workdir string) (string, error) {
	root, err := AnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "insights.json"), nil
}

func AnalysisHistoryPath(workdir string) (string, error) {
	root, err := AnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "history.jsonl"), nil
}

func InsightsByCategoryDir(workdir string) (string, error) {
	return packageRuntimePath(workdir, "analysis", "by-category")
}

func PackageCIDir(workdir string) (string, error) {
	return packageRuntimePath(workdir, "ci")
}

func PackageCIRawDir(workdir string) (string, error) {
	root, err := PackageCIDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "raw"), nil
}

func PackageCIAnalysisDir(workdir string) (string, error) {
	root, err := PackageCIDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "analysis"), nil
}

func PackageCIFindingsPath(workdir string) (string, error) {
	root, err := PackageCIAnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "findings.json"), nil
}

func PackageCISummaryPath(workdir string) (string, error) {
	root, err := PackageCIAnalysisDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "SUMMARY.md"), nil
}

func SelfAnalysisDir(workdir string) (string, error) {
	return packageRuntimePath(workdir, "self-analysis")
}

func ProviderPoolsDir(workdir string) (string, error) {
	return packageRuntimePath(workdir, "provider-pools")
}

func ProviderPoolLeasesDir(workdir string) (string, error) {
	root, err := ProviderPoolsDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "leases"), nil
}

func ProviderPoolSessionsPath(workdir string) (string, error) {
	root, _, err := PrepareProviderRecoveryAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions.json"), nil
}

func ProviderPoolSessionAdaptersDir(workdir string) (string, error) {
	root, _, err := PrepareProviderRecoveryAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "session-adapters"), nil
}

func ProviderPoolAppServerDir(workdir string) (string, error) {
	root, _, err := PrepareProviderRecoveryAuthority(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "app-server"), nil
}

func ProviderPoolAppServerAggregatePath(workdir, sessionID string) (string, error) {
	if !validProviderPoolAggregateSessionID(sessionID) {
		return "", fmt.Errorf("provider-pool App Server aggregate requires a valid session identity")
	}
	root, err := ProviderPoolAppServerDir(workdir)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(sessionID))
	return filepath.Join(root, "aggregates", hex.EncodeToString(digest[:])+".json"), nil
}

func validProviderPoolAggregateSessionID(sessionID string) bool {
	if sessionID == "" || len(sessionID) > 256 || !utf8.ValidString(sessionID) {
		return false
	}
	for _, character := range sessionID {
		if !unicode.IsGraphic(character) || unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func ProviderPoolScratchDir(workdir string) (string, error) {
	root, err := ProviderPoolsDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "scratch"), nil
}

func packageRuntimePath(workdir string, parts ...string) (string, error) {
	root, err := PackageRuntimeDir(workdir)
	if err != nil {
		return "", err
	}
	return infrastructure.JoinStatePath(root, filepath.ToSlash(filepath.Join(parts...)))
}

func CursorPromptPath(workdir string) (string, error) {
	return packageRuntimePath(workdir, "handoff", "orchestrator-cursor-prompt.md")
}

func CursorRefinedPromptPath(workdir string) (string, error) {
	return packageRuntimePath(workdir, "handoff", "orchestrator-cursor-prompt.refined.md")
}

func PastePromptPath(workdir string) (string, error) {
	return packageRuntimePath(workdir, "handoff", "paste-this-prompt.txt")
}
