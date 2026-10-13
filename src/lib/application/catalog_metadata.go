package application

import "dockpipe/src/lib/domain"

func catalogWorkflowCategory(workflow *domain.Workflow) string {
	if workflow.CatalogType == "app" {
		return "app"
	}
	if workflow.CatalogType != "" && workflow.Category == "app" {
		return ""
	}
	return workflow.Category
}
