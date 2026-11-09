package remote

import (
	"errors"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

// validateDeliveryTarget runs under the broker lock at submission and assignment.
// Feature enrollment selects identity only; the worker still owns delivery grants.
func (broker *Broker) validateDeliveryTarget(submission contract.Submission) error {
	if submission.Target == (contract.FeatureTarget{}) {
		return nil
	}
	if broker.features == nil {
		return errors.New("captured workflow machine is unavailable")
	}
	catalog, exists := broker.features.catalogs[submission.Node]
	enrollment := broker.state.Nodes[submission.Node]
	if !exists || !broker.featureCatalogCurrent(catalog) || catalog.Target != submission.Target ||
		remoteidentity.Fingerprint(enrollment.MachineKey) != submission.Target.Fingerprint {
		return errors.New("captured workflow machine or worker generation changed")
	}
	if submission.Root != "" {
		for _, root := range catalog.Roots {
			if root.ID == submission.Root {
				return nil
			}
		}
		return errors.New("captured workflow root grant is unavailable")
	}
	return nil
}

// validateWorkerDeliveryTarget rechecks current local grants immediately before
// materialization. A stale broker catalog cannot redirect a captured selection.
func validateWorkerDeliveryTarget(root string, config contract.WorkerConfig, instance string, submission contract.Submission) error {
	if submission.Target == (contract.FeatureTarget{}) {
		return nil
	}
	if !config.Features || instance == "" {
		return errors.New("captured workflow target requires the paired feature worker")
	}
	permissions, err := loadFeaturePermissions(root)
	if err != nil {
		return err
	}
	target := contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey),
		Generation: Hash(instance + permissionFingerprint(permissions))}
	if target != submission.Target {
		return errors.New("captured workflow machine or worker generation changed")
	}
	if submission.Root != "" {
		for _, grant := range permissions.Roots {
			if grant.ID == submission.Root {
				return nil
			}
		}
		return errors.New("captured workflow root grant is unavailable")
	}
	return nil
}
