package pipelang

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	sharedOracle   []byte
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
func generatedBatchTests(source, checks []byte, sharedOracle ...bool) ([]string, bool) {
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
			case "pipelang-generated-check/oracle":
				if len(sharedOracle) == 0 || !sharedOracle[0] {
					return nil, false
				}
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
	return queueGeneratedBatchWithOracle(t, source, checks, fixtures, nil)
}

func queueGeneratedBatchWithOracle(t *testing.T, source, checks []byte, fixtures map[string][]byte, sharedOracle []byte) bool {
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
	tests, ok := generatedBatchTests(source, checks, len(sharedOracle) != 0)
	if !ok {
		return false
	}
	item := generatedBatchCase{source: bytes.Clone(source), checks: bytes.Clone(checks), fixtures: make(map[string][]byte), tests: tests, sharedOracle: bytes.Clone(sharedOracle)}
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
	limit, fixtureLimit := 4, 8<<20
	if len(sharedOracle) != 0 {
		limit, fixtureLimit = 32, 32<<20
	}
	if len(queue.cases) >= limit || queue.bytes >= fixtureLimit || sourceBytes >= 512<<10 {
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
	if os.Getenv("PIPELANG_BUNDLE_AUDIT") == "1" {
		for _, item := range cases {
			fixtureHash := sha256.New()
			names := make([]string, 0, len(item.fixtures))
			for name := range item.fixtures {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				data := item.fixtures[name]
				fmt.Fprintf(fixtureHash, "%d:%s:%d:", len(name), name, len(data))
				fixtureHash.Write(data)
			}
			t.Logf("generated_case_audit source=%x fixtures=%x tests=%q", sha256.Sum256(item.source), fixtureHash.Sum(nil), item.tests)
		}
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
	var sharedOracle []byte
	for _, item := range cases {
		if len(item.sharedOracle) == 0 {
			continue
		}
		if sharedOracle != nil && !bytes.Equal(sharedOracle, item.sharedOracle) {
			t.Fatal("mixed shared oracle definitions")
		}
		sharedOracle = item.sharedOracle
	}
	if sharedOracle != nil {
		if err := os.Mkdir(filepath.Join(dir, "oracle"), 0700); err != nil {
			t.Fatal(err)
		}
		write("oracle/oracle.go", sharedOracle)
	}
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
	var sealed *os.File
	defer func() {
		if sealed != nil {
			sealed.Close()
		}
	}()
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
		sealed, err = acquireGeneratedRepresentation(cacheRoot, key)
		if err != nil {
			t.Fatal(err)
		}
		if sealed != nil {
			hit = true
			binary = filepath.Join(cacheRoot, key, "program.test")
		} else if cached, ok := readGeneratedArtifact(cacheRoot, key); ok {
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
		if sharedOracle != nil {
			buildCache := os.Getenv("PIPELANG_BUNDLE_BUILD_CACHE")
			if !filepath.IsAbs(buildCache) {
				t.Fatal("bundle population requires an absolute disposable PIPELANG_BUNDLE_BUILD_CACHE")
			}
			if err := os.MkdirAll(buildCache, 0700); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(buildCache)
			if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
				t.Fatal("bundle build cache must be a private directory")
			}
			command.Env = append(command.Env, "GOCACHE="+buildCache)
			// Populate standard dependencies separately so each compiler command keeps
			// the existing 30-second deadline even when this disposable cache is empty.
			prepare := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-p=1", "testing", "encoding/json", "reflect", "strings")
			prepare.Dir, prepare.Env = command.Dir, command.Env
			if output, _, err := measureGeneratedBuild(prepare); err != nil {
				t.Fatalf("prepare disposable bundle build cache: %v\n%s", err, output)
			}
		}
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
	if root := os.Getenv("PIPELANG_SHARED_EXPORT"); root != "" && sharedOracle != nil {
		exportGeneratedSharedExperiment(t, root, dir, binary, key)
	}
	if sealed == nil {
		sealed, err = acquireGeneratedRepresentation(cacheRoot, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Reuse one immutable snapshot across a retained batch's fresh children.
	// This preserves exact executed bytes without rehashing the same binary for
	// every case. Single-case and non-Linux batches retain path revalidation.
	if sealed == nil && (sharedOracle != nil || (cacheRoot != "" && len(cases) > 1 && runtime.GOOS == "linux")) {
		var expected string
		var err error
		if cacheRoot != "" {
			expected, err = generatedArtifactExpectedDigest(cacheRoot, key)
		} else {
			expected, err = generatedFileDigest(binary)
		}
		if err != nil {
			t.Fatal(err)
		}
		sealed, err = sealGeneratedExecutable(binary, expected)
		if err != nil {
			t.Fatal(err)
		}
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
		if cacheRoot != "" && sealed == nil {
			if _, ok := readGeneratedArtifact(cacheRoot, key); !ok {
				t.Fatal("compiled artifact changed before execution")
			}
		}

		executable := binary
		if sealed != nil {
			executable = "/proc/self/fd/3"
		}
		command := exec.Command(executable, "-test.run", fmt.Sprintf("^TestCase%04d$", i), "-test.count=1", "-test.timeout=25s", "-test.v")
		if sealed != nil {
			command.ExtraFiles = []*os.File{sealed}
		}
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
