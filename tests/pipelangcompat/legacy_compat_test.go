package pipelangcompat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang"
)

const (
	wantSourceDigest     = "b0d44c2af52021c1616ae4ffd4bbc234433b36a327cfdbf2403b9ec880d8857c"
	wantProjectionDigest = "a68f1924df93447e42265e4510a53dd96c7bb1e05a58fc1b8db9d322f97b3815"
	wantArtifactDigest   = "167fc9825c3e56b7541a80b1d309e4ecd93eb37f45bdb00630838077f9a48597"
	wantEvaluationDigest = "c10ff75686f372bb81dfb9cdca0aaaee708234a56a0bf6ef9d783a7ed672a3a0"
)

func TestLegacyV0001Compatibility(t *testing.T) {
	repoRoot := repositoryRoot(t)
	paths := frozenInventory(t, repoRoot)
	wantPaths := authoredPipeLangPaths(t, repoRoot)
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("frozen inventory differs from authored PipeLang paths\nfrozen:\n%s\nauthored:\n%s", strings.Join(paths, "\n"), strings.Join(wantPaths, "\n"))
	}

	groups := map[string]map[string][]byte{}
	programs := map[string]*pipelang.Program{}
	var sourceProjection strings.Builder
	var languageProjection strings.Builder
	interfaceFiles := 0
	classFiles := 0
	methodCount := 0

	for _, rel := range paths {
		src, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		fmt.Fprintf(&sourceProjection, "%s\x00%s\x00", rel, src)
		prog, err := pipelang.Parse(src)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		programs[rel] = prog
		if len(prog.Interfaces) > 0 {
			interfaceFiles++
		}
		if len(prog.Classes) > 0 {
			classFiles++
		}
		writeProgramProjection(&languageProjection, rel, prog)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if groups[dir] == nil {
			groups[dir] = map[string][]byte{}
		}
		groups[dir][rel] = src
	}

	var artifactProjection strings.Builder
	var evaluationProjection strings.Builder
	dirs := sortedKeys(groups)
	for _, dir := range dirs {
		files := groups[dir]
		var classNames []string
		for rel := range files {
			for _, class := range programs[rel].Classes {
				if class.Visibility == pipelang.VisibilityPublic {
					classNames = append(classNames, class.Name)
				}
			}
		}
		sort.Strings(classNames)
		for _, className := range classNames {
			out, err := pipelang.CompileFiles(files, className)
			if err != nil {
				t.Fatalf("compile %s/%s: %v", dir, className, err)
			}
			fmt.Fprintf(&artifactProjection, "%s\x00%s\x00%s\x00%s\x00%s\x00", dir, className, out.WorkflowYAML, out.BindingsJSON, out.BindingsEnv)
		}
		for rel := range files {
			for _, class := range programs[rel].Classes {
				if class.Visibility != pipelang.VisibilityPublic {
					continue
				}
				for _, method := range class.Methods {
					if method.Visibility != pipelang.VisibilityPublic {
						continue
					}
					args := make([]string, len(method.Params))
					for i, param := range method.Params {
						args[i] = zeroArgument(param.Type)
					}
					out, err := pipelang.InvokeFiles(files, class.Name, method.Name, args)
					if err != nil {
						t.Fatalf("invoke %s/%s.%s: %v", dir, class.Name, method.Name, err)
					}
					methodCount++
					fmt.Fprintf(&evaluationProjection, "%s\x00%s\x00%s\x00%s\x00%s\x00", dir, class.Name, method.Name, out.Type, out.Value.StringValue())
				}
			}
		}
	}

	if len(paths) != 45 || interfaceFiles != 22 || classFiles != 31 || methodCount != 1 {
		t.Fatalf("legacy shape drifted: files=%d interface_files=%d class_files=%d methods=%d", len(paths), interfaceFiles, classFiles, methodCount)
	}
	assertDigest(t, "source", sourceProjection.String(), wantSourceDigest)
	assertDigest(t, "projection", languageProjection.String(), wantProjectionDigest)
	assertDigest(t, "artifact", artifactProjection.String(), wantArtifactDigest)
	assertDigest(t, "evaluation", evaluationProjection.String(), wantEvaluationDigest)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func frozenInventory(t *testing.T, repoRoot string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "docs", "agents", "tasks", "fixtures", "pipelang-vnext", "legacy-v0.0.0.1-inventory.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "frozen-v0.0.0.1 ") {
			paths = append(paths, strings.TrimPrefix(line, "frozen-v0.0.0.1 "))
		}
	}
	return paths
}

func authoredPipeLangPaths(t *testing.T, repoRoot string) []string {
	t.Helper()
	var paths []string
	for _, root := range []string{"packages", filepath.Join("src", "core", "workflows"), "workflows"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".pipe" {
				return nil
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("inventory %s: %v", root, err)
		}
	}
	sort.Strings(paths)
	return paths
}

func writeProgramProjection(out *strings.Builder, rel string, prog *pipelang.Program) {
	fmt.Fprintf(out, "file %s\n", rel)
	for _, decl := range prog.Interfaces {
		fmt.Fprintf(out, "interface %s %s\n", decl.Visibility, decl.Name)
		writeAnnotations(out, decl.Annotations)
		for _, field := range decl.Fields {
			fmt.Fprintf(out, "field %s %s %s\n", field.Visibility, field.Type, field.Name)
			writeAnnotations(out, field.Annotations)
		}
		for _, method := range decl.Methods {
			fmt.Fprintf(out, "method %s %s %s", method.Visibility, method.ReturnType, method.Name)
			writeParams(out, method.Params)
			out.WriteByte('\n')
			writeAnnotations(out, method.Annotations)
		}
	}
	for _, decl := range prog.Classes {
		fmt.Fprintf(out, "class %s %s : %s\n", decl.Visibility, decl.Name, implementsName(decl))
		writeAnnotations(out, decl.Annotations)
		for _, field := range decl.Fields {
			fmt.Fprintf(out, "field %s %s %s = %s\n", field.Visibility, field.Type, field.Name, pipelang.ExprString(field.Default))
			writeAnnotations(out, field.Annotations)
		}
		for _, method := range decl.Methods {
			fmt.Fprintf(out, "method %s %s %s", method.Visibility, method.ReturnType, method.Name)
			writeParams(out, method.Params)
			fmt.Fprintf(out, " => %s\n", pipelang.ExprString(method.Body))
			writeAnnotations(out, method.Annotations)
		}
	}
}

func writeAnnotations(out *strings.Builder, annotations []pipelang.Annotation) {
	for _, annotation := range annotations {
		fmt.Fprintf(out, "annotation %s %s %s\n", annotation.Name, annotation.Value.Type, annotation.Value.StringValue())
	}
}

func writeParams(out *strings.Builder, params []pipelang.Param) {
	for _, param := range params {
		fmt.Fprintf(out, " %s:%s", param.Name, param.Type)
	}
}

func zeroArgument(typ pipelang.UnresolvedTypeRef) string {
	switch typ.String() {
	case string(pipelang.TypeInt), string(pipelang.TypeFloat):
		return "0"
	case string(pipelang.TypeBool):
		return "false"
	default:
		return ""
	}
}

func implementsName(decl *pipelang.ClassDecl) string {
	if decl == nil || decl.Implements == nil {
		return ""
	}
	return decl.Implements.String()
}

func assertDigest(t *testing.T, name, value, want string) {
	t.Helper()
	sum := sha256.Sum256([]byte(value))
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("%s compatibility digest = %s want %s", name, got, want)
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
