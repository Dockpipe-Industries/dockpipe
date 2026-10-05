package gobackend

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dockpipe/src/lib/pipelang/coreir"
)

func enumGoName(t coreir.Type) string {
	sum := sha256.Sum256([]byte(t.Identity.PackageID + "\x00" + t.Identity.Path))
	return fmt.Sprintf("PipelangEnum_%x", sum)
}
func enumValidationName(t coreir.Type) string { return "validate" + enumGoName(t) }
func emitEnumTypes(out *strings.Builder, fs []coreir.Function) error {
	types := map[string]coreir.Type{}
	add := func(t coreir.Type) {
		if t.Kind == coreir.TypeEnum {
			types[enumGoName(t)] = t
		}
	}
	walk := func(e coreir.Expr) { coreir.WalkExpression(e, func(c coreir.Expr) bool { add(c.Type); return true }) }
	for _, f := range fs {
		for _, typ := range coreir.BlockLocalTypes(f.Body.Block) {
			add(typ)
		}
		add(f.ReturnType)
		for _, p := range f.Parameters {
			add(p.Type)
		}
		walk(f.Body)
	}
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := types[k]
		// A local constant array keeps one comparison loop when Go inlines a
		// small validator into repeated pure calls. A switch per member would
		// multiply control-flow branches by both member and local counts.
		fmt.Fprintf(out, "type %s string\n\nfunc %s(value %s) {\n for _, member := range [...]%s{", k, enumValidationName(t), k, k)
		for _, m := range t.Enum.Members {
			fmt.Fprintf(out, "%s,", strconv.Quote(m.Tag))
		}
		out.WriteString("} {\n if value == member {return}\n }\n panic(\"invalid PipeLang enum value\")\n}\n\n")
	}
	return nil
}
