package pipelang

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"dockpipe/tests/containedexec"
)

// Bound compiler memory independently of concurrent warm shape evaluation.
var generatedCompilerSlot = make(chan struct{}, 1)

func measureGeneratedBuild(command *exec.Cmd) ([]byte, containedexec.Measurement, error) {
	generatedCompilerSlot <- struct{}{}
	defer func() { <-generatedCompilerSlot }()
	return containedexec.Measure(command)
}

type generatedBatchCase struct {
	source, checks []byte
	fixtures       map[string][]byte
	tests          []string
}
type generatedBatchQueue struct {
	cases []generatedBatchCase
	bytes int
}

var generatedBatches = struct {
	sync.Mutex
	queues map[*testing.T]*generatedBatchQueue
}{queues: make(map[*testing.T]*generatedBatchQueue)}

// Only inert generated packages are eligible for shared linking. Keep the direct
// path for custom initialization, compiler directives, special testing entrypoints
// or unsupported imports; changing another package's initialization is not proof.
func generatedBatchTests(source, checks []byte) ([]string, bool) {
	var tests []string
	var packageName string
	for index, data := range [][]byte{source, checks} {
		if bytes.Contains(data, []byte("//go:")) || bytes.Contains(data, []byte("//line ")) {
			return nil, false
		}
		file, err := goparser.ParseFile(gotoken.NewFileSet(), "generated.go", data, 0)
		if err != nil {
			return nil, false
		}
		if index == 0 {
			packageName = file.Name.Name
		} else if file.Name.Name != packageName {
			return nil, false
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, false
			}
			switch name {
			case "testing", "reflect", "math", "strings", "unicode/utf8", "sort", "encoding/json", "os", "strconv", "fmt", "bytes", "errors":
			default:
				return nil, false
			}
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == "init" || d.Name.Name == "TestMain" || strings.HasPrefix(d.Name.Name, "Example") || strings.HasPrefix(d.Name.Name, "Fuzz") {
					return nil, false
				}
				if index == 1 && d.Recv == nil && strings.HasPrefix(d.Name.Name, "Test") {
					suffix := strings.TrimPrefix(d.Name.Name, "Test")
					r, _ := utf8.DecodeRuneInString(suffix)
					if suffix == "" || !unicode.IsLower(r) {
						tests = append(tests, d.Name.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if v, ok := spec.(*ast.ValueSpec); ok {
						for _, value := range v.Values {
							inert := true
							ast.Inspect(value, func(n ast.Node) bool {
								switch n.(type) {
								case *ast.CallExpr, *ast.FuncLit:
									inert = false
								}
								return inert
							})
							if !inert {
								return nil, false
							}
						}
					}
				}
			}
		}
		// A check that observes its testing name must retain the original harness.
		observesName := false
		ast.Inspect(file, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && (s.Sel.Name == "Name" || s.Sel.Name == "PkgPath") {
				observesName = true
			}
			return !observesName
		})
		if observesName {
			return nil, false
		}
	}
	return tests, true
}

func queueGeneratedBatch(t *testing.T, source, checks []byte, fixtures map[string][]byte) bool {
	if os.Getenv("PIPELANG_GENERATED_BATCH") != "1" || os.Getenv("GOFLAGS") != "" {
		return false
	}
	for name := range fixtures {
		// Extra Go files may add initialization or a custom harness that was
		// not inspected with the generated source and checks.
		if strings.HasSuffix(name, ".go") {
			return false
		}
	}
	tests, ok := generatedBatchTests(source, checks)
	if !ok {
		return false
	}
	item := generatedBatchCase{source: bytes.Clone(source), checks: bytes.Clone(checks), fixtures: make(map[string][]byte), tests: tests}
	size := len(source) + len(checks)
	for name, data := range fixtures {
		if filepath.Base(name) != name || name == "." || name == ".." || name == "generated.go" || name == "generated_test.go" || name == "checks.go" || name == "go.mod" {
			t.Fatalf("invalid generated fixture name %q", name)
		}
		item.fixtures[name] = bytes.Clone(data)
		size += len(data)
	}
	generatedBatches.Lock()
	queue, exists := generatedBatches.queues[t]
	if !exists {
		queue = &generatedBatchQueue{}
		generatedBatches.queues[t] = queue
		t.Cleanup(func() {
			generatedBatches.Lock()
			pending := generatedBatches.queues[t]
			delete(generatedBatches.queues, t)
			generatedBatches.Unlock()
			if pending != nil {
				runGeneratedBatch(t, pending.cases)
			}
		})
	}
	queue.cases = append(queue.cases, item)
	queue.bytes += size
	sourceBytes := 0
	for _, item := range queue.cases {
		sourceBytes += len(item.source) + len(item.checks)
	}
	var ready []generatedBatchCase
	if len(queue.cases) >= 4 || queue.bytes >= 8<<20 || sourceBytes >= 512<<10 {
		ready = queue.cases
		queue.cases = nil
		queue.bytes = 0
	}
	generatedBatches.Unlock()
	runGeneratedBatch(t, ready)
	return true
}

func runGeneratedBatch(t *testing.T, cases []generatedBatchCase) {
	t.Helper()
	if len(cases) == 0 {
		return
	}
	dir, err := os.MkdirTemp("/tmp", "pipelang-linked-go-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte("module pipelang-generated-check\n\ngo 1.25\n"))
	var imports, checks strings.Builder
	imports.WriteString("package linkedcheck\nimport (\"testing\"\n")
	for i, item := range cases {
		name := fmt.Sprintf("case%04d", i)
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(name, "generated.go"), item.source)
		write(filepath.Join(name, "checks.go"), item.checks)
		for filename, data := range item.fixtures {
			write(filepath.Join(name, filename), data)
		}
		if len(item.tests) == 0 {
			// Compile-only checks still bind every source byte and execute a
			// fresh driver, while retaining the compiled package artifact.
			fmt.Fprintf(&imports, "_ %q\n", "pipelang-generated-check/"+name)
		} else {
			fmt.Fprintf(&imports, "c%d %q\n", i, "pipelang-generated-check/"+name)
		}
		fmt.Fprintf(&checks, "func TestCase%04d(t *testing.T) {\n", i)
		for _, test := range item.tests {
			fmt.Fprintf(&checks, "t.Run(%q,c%d.%s)\n", test, i, test)
		}
		checks.WriteString("}\n")
	}
	imports.WriteString(")\n")
	write("linked_test.go", []byte(imports.String()+checks.String()))
	binary := filepath.Join(dir, "linked.test")
	cacheRoot, err := generatedCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	var key string
	hit := false
	if cacheRoot != "" {
		key, err = generatedArtifactKey(dir)
		if err != nil {
			t.Fatal(err)
		}
		unlock, err := lockGeneratedArtifact(cacheRoot, key)
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		if cached, ok := readGeneratedArtifact(cacheRoot, key); ok {
			binary = cached
			hit = true
		} else if err := quarantineGeneratedArtifact(cacheRoot, key); err != nil {
			t.Fatal(err)
		}
	}
	if !hit {
		command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-c", "-p=1", "-o", binary, ".")
		command.Dir = dir
		command.Env = append(os.Environ(), "GOROOT="+runtime.GOROOT(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
		output, measurement, err := measureGeneratedBuild(command)
		if err != nil {
			t.Fatalf("link %d generated packages: %v\n%s", len(cases), err, output)
		}
		if cacheRoot != "" {
			binary, err = publishGeneratedArtifact(cacheRoot, key, binary)
			if err != nil {
				t.Fatal(err)
			}
		}
		if os.Getenv("PIPELANG_PERFORMANCE_PROFILE") == "1" {
			t.Logf("generated_shared_link packages=%d elapsed_ns=%d waited_child_rss_kib=%d", len(cases), measurement.Elapsed.Nanoseconds(), measurement.MaxRSSKiB)
		}
	}
	if cacheRoot != "" {
		t.Logf("generated_compiled_artifact packages=%d cache_hit=%t key=%s", len(cases), hit, key)
	}
	// Each original generated module still executes in a new contained child.
	// Its fixture cwd and package globals are isolated; no test result is cached.
	for i, item := range cases {
		name := fmt.Sprintf("case%04d", i)
		// Runtime contents match the original isolated module; only executable
		// construction is shared. Always write the current fixture data.
		execution, err := os.MkdirTemp("/tmp", "pipelang-generated-go-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(execution)
		runtimeFiles := map[string][]byte{"go.mod": []byte("module pipelang-generated-check\n\ngo 1.25\n"), "generated.go": item.source, "generated_test.go": item.checks}
		for filename, data := range item.fixtures {
			runtimeFiles[filename] = data
		}
		for filename, data := range runtimeFiles {
			if err := os.WriteFile(filepath.Join(execution, filename), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if cacheRoot != "" {
			if _, ok := readGeneratedArtifact(cacheRoot, key); !ok {
				t.Fatal("compiled artifact changed before execution")
			}
		}

		command := exec.Command(binary, "-test.run", fmt.Sprintf("^TestCase%04d$", i), "-test.count=1", "-test.timeout=25s", "-test.v")
		command.Dir = execution
		output, measurement, err := containedexec.Measure(command)
		if err != nil {
			t.Fatalf("generated package %s (%v): %v\n%s", name, item.tests, err, output)
		}
		if os.Getenv("PIPELANG_PERFORMANCE_PROFILE") == "1" {
			t.Logf("generated_native_run package=%s tests=%d elapsed_ns=%d waited_child_rss_kib=%d", name, len(item.tests), measurement.Elapsed.Nanoseconds(), measurement.MaxRSSKiB)
		}
	}
}
