package workflowinputs

import (
	"bufio"
	"regexp"
	"strings"
)

var (
	pipeSummaryStartRe   = regexp.MustCompile(`^\s*///\s*<summary>\s*(.*?)\s*$`)
	pipeSummaryEndRe     = regexp.MustCompile(`^(.*?)\s*</summary>\s*$`)
	pipeAnnotationLineRe = regexp.MustCompile(`^\s*\[\s*[A-Za-z_][A-Za-z0-9_]*\s*=.*\]\s*$`)
	pipeTypeLineRe       = regexp.MustCompile(`^\s*(?:public\s+|private\s+)?(?:Interface|Class|Struct)\s+([A-Za-z0-9_]+)\b`)
	pipeFieldLineRe      = regexp.MustCompile(`^\s*public\s+([A-Za-z0-9_<>]+)\s+([A-Za-z0-9_]+)\s*(?:[;=].*)?$`)
)

func fieldDocsByType(files map[string][]byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, b := range files {
		perFile := fieldDocsFromSource(string(b))
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

func fieldDocsFromSource(src string) map[string]map[string]string {
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

func fieldDocForType(docsByType map[string]map[string]string, typeName, fieldName string) string {
	if docsByType == nil {
		return ""
	}
	return docsByType[strings.TrimSpace(typeName)][fieldName]
}
