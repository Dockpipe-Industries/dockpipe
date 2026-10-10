package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ", ") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func submitDelivery(ctx context.Context, root, node, id, workdir, workflow string, includes, dependencies, artifacts []string, dryRun bool, expectedDigest string) error {
	bundle, err := remoteio.BuildBundle(workdir, workflow, includes, dependencies, artifacts)
	if err != nil {
		return err
	}
	digest, err := bundle.Digest()
	if err != nil {
		return err
	}
	if expectedDigest != "" && expectedDigest != digest {
		return errors.New("workflow snapshot changed since preview; preview again before submitting")
	}
	request := contract.SubmitRequest{Submission: contract.Submission{ID: id, Node: node, BundleHash: digest}, Bundle: bundle}
	if err := request.Validate(); err != nil {
		return err
	}
	if dryRun {
		paths := make([]string, 0, len(bundle.Files))
		total := 0
		for _, file := range bundle.Files {
			paths = append(paths, file.Path)
			total += len(file.Data)
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"submission": request.Submission, "files": paths, "bytes": total, "artifacts": bundle.Artifacts})
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	var job contract.Job
	if err := client.Call(ctx, "/v1/submit", request, &job); err != nil {
		return err
	}
	job.Result = nil
	fmt.Fprintf(os.Stderr, "Workflow queued for %s. Inspect with dockpipe remote result --id %s --state %q; download with --out <new-directory>.\n", node, id, root)
	return json.NewEncoder(os.Stdout).Encode(job)
}
