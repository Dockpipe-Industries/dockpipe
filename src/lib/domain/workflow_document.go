package domain

import "dockpipe/src/lib/domain/workflowdocument"

// WorkflowDocumentMaxBytes bounds a visual editor source document.
const WorkflowDocumentMaxBytes = workflowdocument.WorkflowDocumentMaxBytes

type WorkflowDocumentEdit = workflowdocument.WorkflowDocumentEdit
type WorkflowDocumentRequest = workflowdocument.WorkflowDocumentRequest
type WorkflowDocumentResult = workflowdocument.WorkflowDocumentResult

// EditWorkflowDocument edits YAML without executing workflows or reading imports.
func EditWorkflowDocument(request WorkflowDocumentRequest) (*WorkflowDocumentResult, error) {
	return workflowdocument.EditWorkflowDocument(request)
}
