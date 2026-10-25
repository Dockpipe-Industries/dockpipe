package workflowinputs

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dockpipe-Industries/pipelang/src/lib/pipelang"
)

type pipeTypeShape struct {
	Annotations []pipelang.Annotation
	Fields      []pipelang.FieldSig
	ClassName   string
}

func buildField(prog *pipelang.Program, field pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, classDefaults, workflowVars map[string]string, seen map[string]struct{}, depth int) *Field {
	if depth > 8 {
		return nil
	}
	typ := field.Type
	attrs := annotationMap(field.Annotations)
	doc := strings.TrimSpace(fieldDocForType(docsByType, ownerType, field.Name))
	base := &Field{
		FieldName:   field.Name,
		Type:        field.Type.String(),
		Description: doc,
		Attributes:  attrs,
	}

	if inner, ok := typ.ListElementType(); ok {
		base.ElementType = inner.String()
		if inner.IsPrimitive() {
			envName := fieldEnvName(field, envPrefix)
			if envName == "" {
				return nil
			}
			key := strings.ToUpper(strings.TrimSpace(envName))
			if _, ok := seen[key]; ok {
				return nil
			}
			seen[key] = struct{}{}
			base.EnvName = key
			base.DefaultValue = fieldDefaultValue(key, field.Name, classDefaults, workflowVars)
			return base
		}
		if childShape := findTypeShape(prog, inner.String()); childShape != nil {
			childPrefix := childEnvPrefix(field, envPrefix)
			base.Children = buildChildFields(prog, childShape.Fields, childPrefix, inner.String(), docsByType, workflowVars, seen, depth+1)
			if len(base.Children) == 0 {
				return nil
			}
			return base
		}
		return base
	}

	if typ.IsPrimitive() {
		envName := fieldEnvName(field, envPrefix)
		if envName == "" {
			return nil
		}
		key := strings.ToUpper(strings.TrimSpace(envName))
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		base.EnvName = key
		base.DefaultValue = fieldDefaultValue(key, field.Name, classDefaults, workflowVars)
		return base
	}

	if childShape := findTypeShape(prog, typ.String()); childShape != nil {
		childPrefix := childEnvPrefix(field, envPrefix)
		nestedDefaults := nestedClassDefaults(prog, typ.String())
		base.Children = buildChildFieldsWithDefaults(prog, childShape.Fields, childPrefix, typ.String(), docsByType, workflowVars, seen, nestedDefaults, depth+1)
		if len(base.Children) == 0 {
			return nil
		}
		return base
	}

	return nil
}

func buildChildFields(prog *pipelang.Program, fields []pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, workflowVars map[string]string, seen map[string]struct{}, depth int) []Field {
	return buildChildFieldsWithDefaults(prog, fields, envPrefix, ownerType, docsByType, workflowVars, seen, nil, depth)
}

func buildChildFieldsWithDefaults(prog *pipelang.Program, fields []pipelang.FieldSig, envPrefix, ownerType string, docsByType map[string]map[string]string, workflowVars map[string]string, seen map[string]struct{}, classDefaults map[string]string, depth int) []Field {
	out := make([]Field, 0, len(fields))
	for _, child := range fields {
		record := buildField(prog, child, envPrefix, ownerType, docsByType, classDefaults, workflowVars, seen, depth)
		if record == nil {
			continue
		}
		out = append(out, *record)
	}
	return out
}

func nestedClassDefaults(prog *pipelang.Program, typeName string) map[string]string {
	if decl := findClassDecl(prog, typeName); decl != nil {
		return literalClassDefaults(prog, decl.Name)
	}
	trimmed := strings.TrimSpace(typeName)
	if trimmed == "" {
		return nil
	}
	var implName string
	for _, decl := range prog.Classes {
		if decl.Implements == nil || strings.TrimSpace(decl.Implements.Name) != trimmed {
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
	return literalClassDefaults(prog, implName)
}

func fieldDefaultValue(envName, fieldName string, classDefaults, workflowVars map[string]string) string {
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

func childEnvPrefix(field pipelang.FieldSig, envPrefix string) string {
	base := fieldEnvName(field, envPrefix)
	if base == "" {
		return envPrefix
	}
	return strings.ToUpper(strings.TrimSpace(base)) + "_"
}

func fieldEnvName(field pipelang.FieldSig, prefix string) string {
	if explicit := annotationString(field.Annotations, "envname"); explicit != "" {
		return explicit
	}
	base := fieldNameToEnv(field.Name)
	if base == "" {
		return ""
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return base
	}
	return prefix + base
}

func annotationString(in []pipelang.Annotation, name string) string {
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

func inferredEnvPrefix(className, typeRef string) string {
	name := strings.TrimSpace(className)
	if name == "" {
		name = strings.TrimSpace(typeRef)
	}
	if strings.Contains(strings.ToLower(name), "vm") {
		return "DOCKPIPE_VM_"
	}
	return ""
}

func annotationMap(in []pipelang.Annotation) map[string]string {
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
	analysis := pipelang.ParseFiles(files)
	if err := analysis.Error(); err != nil {
		return nil, err
	}
	return analysis.Program, nil
}

func parseTypeSpec(moduleRoot, raw string) (string, string, error) {
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

func findInterfaceDecl(prog *pipelang.Program, name string) *pipelang.InterfaceDecl {
	for _, decl := range prog.Interfaces {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findClassDecl(prog *pipelang.Program, name string) *pipelang.ClassDecl {
	for _, decl := range prog.Classes {
		if strings.TrimSpace(decl.Name) == strings.TrimSpace(name) {
			return decl
		}
	}
	return nil
}

func findTypeShape(prog *pipelang.Program, name string) *pipeTypeShape {
	if decl := findInterfaceDecl(prog, name); decl != nil {
		return &pipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      decl.Fields,
		}
	}
	if decl := findClassDecl(prog, name); decl != nil {
		if ifaceName := implementedTypeName(decl); ifaceName != "" {
			if iface := findInterfaceDecl(prog, ifaceName); iface != nil {
				return &pipeTypeShape{
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
		return &pipeTypeShape{
			Annotations: decl.Annotations,
			Fields:      fields,
			ClassName:   decl.Name,
		}
	}
	return nil
}

func literalClassDefaults(prog *pipelang.Program, className string) map[string]string {
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

func implementedTypeName(decl *pipelang.ClassDecl) string {
	if decl == nil || decl.Implements == nil {
		return ""
	}
	return strings.TrimSpace(decl.Implements.Name)
}

func fieldNameToEnv(name string) string {
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
