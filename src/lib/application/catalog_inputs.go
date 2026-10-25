package application

import (
	"strings"

	"dockpipe/src/lib/application/internal/workflowinputs"
	"dockpipe/src/lib/domain"
)

func cloneCatalogVars(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildCatalogWorkflowInputs(configPath string, workflow *domain.Workflow) []catalogWorkflowInputRecord {
	// Catalog listing remains tolerant of unavailable or malformed type sources.
	discovery := workflowinputs.Discover(configPath, "", workflow, nil)
	return catalogWorkflowInputRecords(discovery.Fields)
}

func catalogWorkflowInputRecords(fields []workflowinputs.Field) []catalogWorkflowInputRecord {
	if len(fields) == 0 {
		return nil
	}
	records := make([]catalogWorkflowInputRecord, 0, len(fields))
	for _, field := range fields {
		record := catalogWorkflowInputRecord{
			FieldName:    field.FieldName,
			EnvName:      field.EnvName,
			Type:         field.Type,
			ElementType:  field.ElementType,
			Description:  field.Description,
			DefaultValue: field.DefaultValue,
			Attributes:   cloneCatalogVars(field.Attributes),
			Children:     catalogWorkflowInputRecords(field.Children),
		}
		control := ""
		if field.ElementType != "" && field.EnvName != "" {
			control = "list"
		} else if field.ElementType != "" && len(field.Children) > 0 {
			control = "collection"
		} else if len(field.Children) > 0 {
			control = "object"
		}
		if control != "" {
			if record.Attributes == nil {
				record.Attributes = map[string]string{}
			}
			if _, exists := record.Attributes["control"]; !exists {
				record.Attributes["control"] = control
			}
		}
		records = append(records, record)
	}
	return records
}

func buildCatalogWorkflowView(view domain.WorkflowView, inputs []catalogWorkflowInputRecord) *catalogWorkflowViewRecord {
	if view.Entry == nil && len(view.Pages) == 0 {
		return nil
	}
	known := map[string]struct{}{}
	collectCatalogInputPaths(inputs, "", known)
	out := &catalogWorkflowViewRecord{}
	pageIDs := map[string]struct{}{}
	for _, page := range view.Pages {
		pageRec := catalogWorkflowViewPageRecord{
			ID:          strings.TrimSpace(page.ID),
			Title:       strings.TrimSpace(page.Title),
			Description: strings.TrimSpace(page.Description),
		}
		for _, section := range page.Sections {
			secRec := catalogWorkflowViewSectionRecord{
				ID:          strings.TrimSpace(section.ID),
				Title:       strings.TrimSpace(section.Title),
				Description: strings.TrimSpace(section.Description),
			}
			for _, field := range section.Fields {
				field = strings.TrimSpace(field)
				if field == "" {
					continue
				}
				if _, ok := known[field]; !ok {
					continue
				}
				secRec.Fields = append(secRec.Fields, field)
			}
			if len(secRec.Fields) == 0 && secRec.Title == "" && secRec.Description == "" && secRec.ID == "" {
				continue
			}
			if len(secRec.Fields) == 0 {
				continue
			}
			pageRec.Sections = append(pageRec.Sections, secRec)
		}
		if len(pageRec.Sections) == 0 {
			continue
		}
		if pageRec.ID != "" {
			pageIDs[pageRec.ID] = struct{}{}
		}
		out.Pages = append(out.Pages, pageRec)
	}
	if view.Entry != nil {
		entryField := strings.TrimSpace(view.Entry.Field)
		if _, ok := known[entryField]; ok && entryField != "" {
			entry := &catalogWorkflowViewEntryRecord{
				Type:        strings.TrimSpace(view.Entry.Type),
				Field:       entryField,
				Title:       strings.TrimSpace(view.Entry.Title),
				Description: strings.TrimSpace(view.Entry.Description),
			}
			for _, option := range view.Entry.Options {
				opt := catalogWorkflowViewEntryOptionRecord{
					Value: strings.TrimSpace(option.Value),
					Label: strings.TrimSpace(option.Label),
					Next:  strings.TrimSpace(option.Next),
				}
				if opt.Value == "" {
					continue
				}
				if opt.Label == "" {
					opt.Label = opt.Value
				}
				for _, pageID := range option.Pages {
					pageID = strings.TrimSpace(pageID)
					if pageID == "" {
						continue
					}
					if _, ok := pageIDs[pageID]; !ok {
						continue
					}
					opt.Pages = append(opt.Pages, pageID)
				}
				if opt.Next != "" {
					if _, ok := pageIDs[opt.Next]; !ok {
						opt.Next = ""
					}
				}
				if len(opt.Pages) == 0 && opt.Next != "" {
					opt.Pages = []string{opt.Next}
				}
				entry.Options = append(entry.Options, opt)
			}
			if len(entry.Options) > 0 {
				out.Entry = entry
			}
		}
	}
	if out.Entry == nil && len(out.Pages) == 0 {
		return nil
	}
	return out
}

func collectCatalogInputPaths(inputs []catalogWorkflowInputRecord, prefix string, out map[string]struct{}) {
	for _, input := range inputs {
		name := strings.TrimSpace(input.FieldName)
		if name == "" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		out[path] = struct{}{}
		if len(input.Children) > 0 {
			collectCatalogInputPaths(input.Children, path, out)
		}
	}
}
