package remote

import (
	"errors"
	"os"
	"path/filepath"

	contract "dockpipe/src/lib/domain/remote"
)

func (b *Broker) storeBundle(request contract.SubmitRequest) error {
	if request.BundleHash == "" {
		if request.Bundle != nil {
			return errors.New("unexpected delivery payload")
		}
		return nil
	}
	if request.Bundle == nil {
		return errors.New("delivery payload required")
	}
	digest, err := request.Bundle.Digest()
	if err != nil {
		return err
	}
	if digest != request.BundleHash {
		return errors.New("delivery digest mismatch")
	}
	path := filepath.Join(filepath.Dir(b.path), "bundles", request.ID+".json")
	var existing contract.Bundle
	if err := ReadPrivate(path, &existing); err == nil {
		previous, err := existing.Digest()
		if err != nil || previous != digest {
			return errors.New("stored delivery conflicts with submission")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// The payload is durable before the queue accepts it. An interrupted submit
	// can recover only the identical bytes under the same retained job ID.
	return WritePrivate(path, request.Bundle)
}

func (b *Broker) loadBundle(job contract.Job) (*contract.Bundle, error) {
	if job.Validate() != nil || job.BundleHash == "" {
		return nil, errors.New("job does not select a delivery")
	}
	var bundle contract.Bundle
	if err := ReadPrivate(filepath.Join(filepath.Dir(b.path), "bundles", job.ID+".json"), &bundle); err != nil {
		return nil, err
	}
	digest, err := bundle.Digest()
	if err != nil || digest != job.BundleHash {
		return nil, errors.New("stored delivery digest mismatch")
	}
	return &bundle, nil
}
