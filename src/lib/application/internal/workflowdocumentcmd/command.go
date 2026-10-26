package workflowdocumentcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"dockpipe/src/lib/domain"
)

// The bridge is stdin/stdout only: saving and executing remain separate actions.
func Run(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: dockpipe workflow document < request.json")
	}
	return workflowDocumentBridge(os.Stdin, os.Stdout)
}

func workflowDocumentBridge(input io.Reader, output io.Writer) error {
	// JSON escaping can expand a source character to six bytes. Edits share the
	// same transport budget and are further checked by the domain operation.
	const maxRequestBytes = 6 * domain.WorkflowDocumentMaxBytes
	data, err := io.ReadAll(io.LimitReader(input, maxRequestBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxRequestBytes {
		return fmt.Errorf("workflow document request exceeds 6 MiB")
	}
	var request domain.WorkflowDocumentRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("document request: %w", err)
	}
	result, err := domain.EditWorkflowDocument(request)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}
