package remotecmd

import (
	"context"
	"encoding/json"
	"os"

	contract "dockpipe/src/lib/domain/remote"
)

func reviewJobInterruption(ctx context.Context, root string, request contract.JobInterruptionRequest, acknowledge bool) error {
	if err := request.Validate(acknowledge); err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()

	path := contract.JobInterruptionInspectPath
	if acknowledge {
		path = contract.JobInterruptionAcknowledgePath
	}
	var review contract.JobInterruptionReview
	if err := client.Call(ctx, path, request, &review); err != nil {
		return err
	}
	if err := review.Validate(request, acknowledge); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(review)
}
