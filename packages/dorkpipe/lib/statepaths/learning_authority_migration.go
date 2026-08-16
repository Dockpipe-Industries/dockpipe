package statepaths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	learningAuthorityMigrationCohort     = "dorkpipe-insights-metrics-training-authority"
	learningAuthorityDestinationName     = "learning"
	learningAuthorityLockName            = ".learning-migration.lock"
	learningAuthorityPendingName         = ".learning-migration.pending.json"
	learningAuthorityProvenanceName      = ".learning-migration.json"
	learningAuthorityTemporaryNamePrefix = ".learning-import-"
)

// LearningAuthorityMigrationStatus reports compatibility evidence without making legacy bytes
// authoritative after the durable learning directory has been published.
type LearningAuthorityMigrationStatus struct {
	DurableAuthoritative bool
	ImportedLegacy       bool
	LegacyDiverged       bool
}

var learningAuthorityMigrationTestHook func(string) error

func learningAuthorityMigrationSpec() durableCohortMigrationSpec {
	return durableCohortMigrationSpec{
		label:               "learning authority",
		cohort:              learningAuthorityMigrationCohort,
		destinationName:     learningAuthorityDestinationName,
		lockName:            learningAuthorityLockName,
		pendingName:         learningAuthorityPendingName,
		provenanceName:      learningAuthorityProvenanceName,
		temporaryNamePrefix: learningAuthorityTemporaryNamePrefix,
		legacyContentRoot:   func(legacyRoot string) string { return legacyRoot },
		collectLegacy:       collectLearningAuthorityLegacy,
		testHook:            learningAuthorityMigrationTestHook,
	}
}

// PrepareLearningAuthority resolves DorkPipe's collision-safe durable package owner and
// atomically imports only insights/history and cumulative metrics/training. The legacy tree is
// validated and read-only; disposable analysis exports and run-local training artifacts remain.
func PrepareLearningAuthority(workdir string) (string, LearningAuthorityMigrationStatus, error) {
	root, status, err := prepareDurableCohortAuthority(workdir, learningAuthorityMigrationSpec())
	return root, LearningAuthorityMigrationStatus(status), err
}

func collectLearningAuthorityLegacy(legacyRoot string) (providerRecoveryObservedInventory, error) {
	device, err := migrationDeviceIdentity(legacyRoot)
	if err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	collector := providerRecoveryCollector{root: legacyRoot, device: device, identities: map[string]string{}}
	if err := collector.addOptionalRegular("metrics.jsonl"); err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	if err := collectLearningAnalysis(&collector); err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	if err := collectLearningTraining(&collector); err != nil {
		return providerRecoveryObservedInventory{}, err
	}
	collector.sort()
	return providerRecoveryObservedInventory{entries: collector.entries, identities: collector.identities}, nil
}

func collectLearningAnalysis(collector *providerRecoveryCollector) error {
	root := filepath.Join(collector.root, "analysis")
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := collector.addDirectory("analysis"); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "queue.json", "insights.json", "history.jsonl":
			if err := collector.addRegular(filepath.Join("analysis", entry.Name())); err != nil {
				return err
			}
		case "by-category":
			// Deterministic exports remain disposable and are never imported.
		default:
			return fmt.Errorf("learning analysis contains unclassified entry %q", entry.Name())
		}
	}
	return nil
}

func collectLearningTraining(collector *providerRecoveryCollector) error {
	root := filepath.Join(collector.root, "training")
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := collector.addDirectory("training"); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "metrics.jsonl" {
			return fmt.Errorf("learning training state contains unclassified entry %q", entry.Name())
		}
		if err := collector.addRegular(filepath.Join("training", entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (c *providerRecoveryCollector) addOptionalRegular(rel string) error {
	_, err := os.Lstat(filepath.Join(c.root, rel))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.addRegular(rel)
}

func validateLearningAuthorityManifest(path, phase string) (providerRecoveryMigrationManifest, error) {
	manifest, err := readProviderRecoveryManifestForSpec(path, phase, learningAuthorityMigrationSpec())
	if err != nil {
		return providerRecoveryMigrationManifest{}, err
	}
	if manifest.Cohort != learningAuthorityMigrationCohort {
		return providerRecoveryMigrationManifest{}, errors.New("learning authority provenance cohort is invalid")
	}
	return manifest, nil
}
