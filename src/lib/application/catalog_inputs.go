package application

import (
	"bufio"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"dockpipe/src/lib/application/internal/pipelangmaterialize"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/pipelang"
)

var (
	pipeSummaryStartRe   = regexp.MustCompile(`^\s*///\s*<summary>\s*(.*?)\s*$`)
	pipeSummaryEndRe     = regexp.MustCompile(`^(.*?)\s*</summary>\s*$`)
	pipeAnnotationLineRe = regexp.MustCompile(`^\s*\[\s*[A-Za-z_][A-Za-z0-9_]*\s*=.*\]\s*$`)
	pipeTypeLineRe       = regexp.MustCompile(`^\s*(?:public\s+|private\s+)?(?:Interface|Class|Struct)\s+([A-Za-z0-9_]+)\b`)
	pipeFieldLineRe      = regexp.MustCompile(`^\s*public\s+([A-Za-z0-9_<>]+)\s+([A-Za-z0-9_]+)\s*(?:[;=].*)?$`)
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

type catalogPipeTypeShape struct {
	Annotations []pipelang.Annotation
	Fields      []pipelang.FieldSig
	ClassName   string
}

type catalogWorkflowTypeEntry struct {
	Spec       string
	ModuleRoot string
}

func buildCatalogWorkflowInputs(cfgPath string, wf *domain.Workflow) []catalogWorkflowInputRecord {
	return buildCatalogWorkflowInputsForStepWithProjectRoot(cfgPath, "", wf, nil)
}

func buildCatalogWorkflowInputsForStepWithProjectRoot(cfgPath, projectRoot string, wf *domain.Workflow, step *domain.Step) []catalogWorkflowInputRecord {
	entries, err := catalogWorkflowTypeEntries(cfgPath, projectRoot, wf, step)
	if err != nil || len(entries) == 0 {
		return nil
	}
	return buildCatalogWorkflowInputsFromTypeEntries(entries, wf.Vars)
}

func buildCatalogWorkflowInputsFromTypeEntries(entries []catalogWorkflowTypeEntry, workflowVars map[string]string) []catalogWorkflowInputRecord {
	defaultsByClass := map[string]map[string]string{}
	seen := map[string]struct{}{}
	filesByRoot := map[string]map[string][]byte{}
	progByRoot := map[string]*pipelang.Program{}
	var out []catalogWorkflowInputRecord

	for _, entry := range entries {
		filePath, typeRef, err := parseCatalogTypeSpec(entry.ModuleRoot, entry.Spec)
		if err != nil {
			continue
		}
		typeRoot := filepath.Dir(filePath)
		files, ok := filesByRoot[typeRoot]
		if !ok {
			files, _, err = pipelangmaterialize.ReadFilesUnder(typeRoot)
			if err != nil || len(files) == 0 {
				continue
			}
			filesByRoot[typeRoot] = files
		}
		prog, ok := progByRoot[typeRoot]
		if !ok {
			prog, err = mergePipeLangProgram(files)
			if err != nil {
				continue
			}
			progByRoot[typeRoot] = prog
		}
		shape := findCatalogPipeTypeShape(prog, typeRef)
		if shape == nil {
			continue
		}
		docsByType := extractCatalogPipeFieldDocsByType(files)
		className := shape.ClassName
		if className == "" {
			className, err = pipelangmaterialize.InferEntryClassFromTypeRef(files, typeRef)
		}
		classDefaults := map[string]string{}
		if className != "" && err == nil {
			if cached, ok := defaultsByClass[className]; ok {
				classDefaults = cached
			} else {
				classDefaults = findCatalogClassDefaults(prog, className)
				defaultsByClass[className] = classDefaults
			}
		}
		envPrefix := catalogInferredEnvPrefix(shape.ClassName, typeRef)
		shapeName := strings.TrimSpace(typeRef)
		if strings.TrimSpace(shape.ClassName) != "" {
			shapeName = strings.TrimSpace(shape.ClassName)
		}
		for _, field := range shape.Fields {
			record := buildCatalogWorkflowInputRecord(prog, field, envPrefix, shapeName, docsByType, classDefaults, workflowVars, seen, 0)
			if record == nil {
				continue
			}
			out = append(out, *record)
		}
	}
	return out
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

func buildCatalogWorkflowInputRecord(prog *pipelang.Program, field pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, classDefaults, workflowVars map[string]string, seen map[string]struct{}, depth int) *catalogWorkflowInputRecord {
	if depth > 8 {
		return nil
	}
	typ := pipelang.TypeName(field.Type)
	attrs := catalogAnnotationMap(field.Annotations)
	doc := strings.TrimSpace(catalogFieldDocForType(docsByType, ownerType, field.Name))
	base := &catalogWorkflowInputRecord{
		FieldName:   field.Name,
		Type:        string(field.Type),
		Description: doc,
		Attributes:  attrs,
	}

	if inner, ok := typ.ListElementType(); ok {
		base.ElementType = string(inner)
		if inner.IsPrimitive() {
			envName := catalogFieldEnvName(field, envPrefix)
			if envName == "" {
				return nil
			}
			key := strings.ToUpper(strings.TrimSpace(envName))
			if _, ok := seen[key]; ok {
				return nil
			}
			seen[key] = struct{}{}
			base.EnvName = key
			base.DefaultValue = catalogFieldDefaultValue(key, field.Name, classDefaults, workflowVars)
			if base.Attributes == nil {
				base.Attributes = map[string]string{}
			}
			if _, ok := base.Attributes["control"]; !ok {
				base.Attributes["control"] = "list"
			}
			return base
		}
		if childShape := findCatalogPipeTypeShape(prog, string(inner)); childShape != nil {
			childPrefix := catalogChildEnvPrefix(field, envPrefix)
			base.Children = buildCatalogChildWorkflowInputs(prog, childShape.Fields, childPrefix, string(inner), docsByType, workflowVars, seen, depth+1)
			if len(base.Children) == 0 {
				return nil
			}
			if base.Attributes == nil {
				base.Attributes = map[string]string{}
			}
			if _, ok := base.Attributes["control"]; !ok {
				base.Attributes["control"] = "collection"
			}
			return base
		}
		return base
	}

	if typ.IsPrimitive() {
		envName := catalogFieldEnvName(field, envPrefix)
		if envName == "" {
			return nil
		}
		key := strings.ToUpper(strings.TrimSpace(envName))
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		base.EnvName = key
		base.DefaultValue = catalogFieldDefaultValue(key, field.Name, classDefaults, workflowVars)
		return base
	}

	if childShape := findCatalogPipeTypeShape(prog, string(typ)); childShape != nil {
		childPrefix := catalogChildEnvPrefix(field, envPrefix)
		nestedDefaults := catalogNestedClassDefaults(prog, string(typ))
		base.Children = buildCatalogChildWorkflowInputsWithDefaults(prog, childShape.Fields, childPrefix, string(typ), docsByType, workflowVars, seen, nestedDefaults, depth+1)
		if len(base.Children) == 0 {
			return nil
		}
		if base.Attributes == nil {
			base.Attributes = map[string]string{}
		}
		if _, ok := base.Attributes["control"]; !ok {
			base.Attributes["control"] = "object"
		}
		return base
	}

	return nil
}

func buildCatalogChildWorkflowInputs(prog *pipelang.Program, fields []pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, workflowVars map[string]string, seen map[string]struct{}, depth int) []catalogWorkflowInputRecord {
	return buildCatalogChildWorkflowInputsWithDefaults(prog, fields, envPrefix, ownerType, docsByType, workflowVars, seen, nil, depth)
}

func buildCatalogChildWorkflowInputsWithDefaults(prog *pipelang.Program, fields []pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, workflowVars map[string]string, seen map[string]struct{}, classDefaults map[string]string, depth int) []catalogWorkflowInputRecord {
	out := make([]catalogWorkflowInputRecord, 0, len(fields))
	for _, child := range fields {
		record := buildCatalogWorkflowInputRecord(prog, child, envPrefix, ownerType, docsByType, classDefaults, workflowVars, seen, depth)
		if record == nil {
			continue
		}
		out = append(out, *record)
	}
	return out
}

func catalogNestedClassDefaults(prog *pipelang.Program, typeName string) map[string]string {
	if decl := findCatalogClassDecl(prog, typeName); decl != nil {
		return findCatalogClassDefaults(prog, decl.Name)
	}
	trimmed := strings.TrimSpace(typeName)
	if trimmed == "" {
		return nil
	}
	var implName string
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Implements) != trimmed {
			continue
		}
		if implName != "" {
			return nil
		}
		implName = decl.Name
	}
	if implName == "" {
		return nil
	}
	return findCatalogClassDefaults(prog, implName)
}

func catalogFieldDefaultValue(envName, fieldName string, classDefaults, workflowVars map[string]string) string {
	if workflowVars != nil {
		if v, ok := workflowVars[envName]; ok {
			return v
		}
	}
	if classDefaults != nil {
		return classDefaults[fieldName]
	}
	return ""
}

func catalogChildEnvPrefix(field pipelang.FieldSig, envPrefix string) string {
	base := catalogFieldEnvName(field, envPrefix)
	if base == "" {
		return envPrefix
	}
	return strings.ToUpper(strings.TrimSpace(base)) + "_"
}

func catalogFieldEnvName(field pipelang.FieldSig, prefix string) string {
	if explicit := catalogAnnotationString(field.Annotations, "envname"); explicit != "" {
		return explicit
	}
	base := catalogFieldNameToEnv(field.Name)
	if base == "" {
		return ""
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return base
	}
	return prefix + base
}

func catalogAnnotationString(in []pipelang.Annotation, name string) string {
	want := strings.TrimSpace(strings.ToLower(name))
	if want == "" {
		return ""
	}
	for _, ann := range in {
		if strings.TrimSpace(strings.ToLower(ann.Name)) != want {
			continue
		}
		return strings.TrimSpace(ann.Value.StringValue())
	}
	return ""
}

func catalogInferredEnvPrefix(className, typeRef string) string {
	name := strings.TrimSpace(className)
	if name == "" {
		name = strings.TrimSpace(typeRef)
	}
	if strings.Contains(strings.ToLower(name), "vm") {
		return "DOCKPIPE_VM_"
	}
	return ""
}

func catalogAnnotationMap(in []pipelang.Annotation) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, ann := range in {
		key := strings.ToLower(strings.TrimSpace(ann.Name))
		if key == "" {
			continue
		}
		out[key] = ann.Value.StringValue()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergePipeLangProgram(files map[string][]byte) (*pipelang.Program, error) {
	merged := &pipelang.Program{}
	for name, b := range files {
		p, err := pipelang.Parse(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		merged.Interfaces = append(merged.Interfaces, p.Interfaces...)
		merged.Classes = append(merged.Classes, p.Classes...)
	}
	return merged, nil
}

func parseCatalogTypeSpec(moduleRoot, raw string) (string, string, error) {
	spec := strings.TrimSpace(raw)
	if spec == "" {
		return "", "", fmt.Errorf("empty type spec")
	}
	left := spec
	typeRef := ""
	if i := strings.Index(spec, "<"); i >= 0 {
		j := strings.LastIndex(spec, ">")
		if j <= i+1 {
			return "", "", fmt.Errorf("invalid type spec %q", spec)
		}
		left = strings.TrimSpace(spec[:i])
		typeRef = strings.TrimSpace(spec[i+1 : j])
	}
	if filepath.Ext(left) == "" {
		left += ".pipe"
	}
	abs, err := filepath.Abs(filepath.Join(moduleRoot, filepath.FromSlash(left)))
	if err != nil {
		return "", "", err
	}
	if typeRef == "" {
		typeRef = strings.TrimSuffix(filepath.Base(left), filepath.Ext(left))
	}
	return abs, typeRef, nil
}

func findCatalogInterfaceDecl(prog *pipelang.Program, name string) *pipelang.InterfaceDecl {
	for _, decl := range prog.Interfaces {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findCatalogClassDecl(prog *pipelang.Program, name string) *pipelang.ClassDecl {
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findCatalogPipeTypeShape(prog *pipelang.Program, name string) *catalogPipeTypeShape {
	if decl := findCatalogInterfaceDecl(prog, name); decl != nil {
		return &catalogPipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      decl.Fields,
		}
	}
	if decl := findCatalogClassDecl(prog, name); decl != nil {
		if ifaceName := strings.TrimSpace(decl.Implements); ifaceName != "" {
			if iface := findCatalogInterfaceDecl(prog, ifaceName); iface != nil {
				return &catalogPipeTypeShape{
					Annotations: iface.Annotations,
					Fields:      iface.Fields,
					ClassName:   decl.Name,
				}
			}
		}
		fields := make([]pipelang.FieldSig, 0, len(decl.Fields))
		for _, field := range decl.Fields {
			fields = append(fields, pipelang.FieldSig{
				Visibility:  field.Visibility,
				Annotations: field.Annotations,
				Type:        field.Type,
				Name:        field.Name,
			})
		}
		return &catalogPipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      fields,
			ClassName:   decl.Name,
		}
	}
	return nil
}

func findCatalogClassDefaults(prog *pipelang.Program, className string) map[string]string {
	out := map[string]string{}
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Name) != strings.TrimSpace(className) {
			continue
		}
		for _, field := range decl.Fields {
			if lit, ok := field.Default.(*pipelang.LiteralExpr); ok {
				out[field.Name] = lit.Value.StringValue()
			}
		}
		break
	}
	return out
}

func extractCatalogPipeFieldDocsByType(files map[string][]byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, b := range files {
		perFile := extractCatalogPipeFieldDocsByTypeFromSource(string(b))
		for typeName, fields := range perFile {
			dst := out[typeName]
			if dst == nil {
				dst = map[string]string{}
				out[typeName] = dst
			}
			for fieldName, doc := range fields {
				if strings.TrimSpace(doc) == "" {
					continue
				}
				dst[fieldName] = doc
			}
		}
	}
	return out
}

func extractCatalogPipeFieldDocsByTypeFromSource(src string) map[string]map[string]string {
	out := map[string]map[string]string{}
	var pending []string
	var currentType string
	inSummary := false
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if inSummary {
			if m := pipeSummaryEndRe.FindStringSubmatch(line); len(m) == 2 {
				text := strings.TrimSpace(m[1])
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = false
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "///"))
			if text != "" {
				pending = append(pending, text)
			}
			continue
		}
		if m := pipeSummaryStartRe.FindStringSubmatch(line); len(m) == 2 {
			text := strings.TrimSpace(m[1])
			if strings.Contains(text, "</summary>") {
				text = strings.TrimSpace(strings.TrimSuffix(text, "</summary>"))
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = false
			} else {
				if text != "" {
					pending = append(pending, text)
				}
				inSummary = true
			}
			continue
		}
		if m := pipeTypeLineRe.FindStringSubmatch(line); len(m) == 2 {
			currentType = strings.TrimSpace(m[1])
			pending = nil
			continue
		}
		if strings.TrimSpace(line) == "}" {
			currentType = ""
			pending = nil
			continue
		}
		if m := pipeFieldLineRe.FindStringSubmatch(line); len(m) == 3 {
			fieldName := strings.TrimSpace(m[2])
			if currentType != "" && fieldName != "" && len(pending) > 0 {
				dst := out[currentType]
				if dst == nil {
					dst = map[string]string{}
					out[currentType] = dst
				}
				dst[fieldName] = strings.Join(pending, " ")
			}
			pending = nil
			continue
		}
		if pipeAnnotationLineRe.MatchString(line) {
			continue
		}
		if strings.TrimSpace(line) != "" {
			pending = nil
		}
	}
	return out
}

func catalogFieldDocForType(docsByType map[string]map[string]string, typeName, fieldName string) string {
	if docsByType == nil {
		return ""
	}
	return docsByType[strings.TrimSpace(typeName)][fieldName]
}

func catalogFieldNameToEnv(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	prevLowerOrDigit := false
	prevUpper := false
	for i, r := range name {
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if i > 0 && isUpper && (prevLowerOrDigit || (prevUpper && i+1 < len(name) && name[i+1] >= 'a' && name[i+1] <= 'z')) {
			b.WriteByte('_')
		} else if i > 0 && isDigit && !prevLowerOrDigit && !prevUpper {
			b.WriteByte('_')
		}
		if r == '-' || r == ' ' {
			b.WriteByte('_')
			prevLowerOrDigit = false
			prevUpper = false
			continue
		}
		if isLower {
			r = r - 'a' + 'A'
		}
		b.WriteRune(r)
		prevLowerOrDigit = isLower || isDigit
		prevUpper = isUpper
	}
	return b.String()
}
