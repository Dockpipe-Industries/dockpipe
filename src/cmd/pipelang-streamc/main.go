// pipelang-streamc is the explicit experimental native-stream compiler shell.
// It reads only named source/manifest files and writes generated C++/binding data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"dockpipe/src/lib/pipelang"
	"dockpipe/src/lib/pipelang/streamcpp"
	"dockpipe/src/lib/pipelang/streamir"
)

func boundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("input exceeds compiler extent")
	}
	return data, nil
}
func run() error {
	source := flag.String("source", "", "PipeLang source file")
	manifest := flag.String("manifest", "", "SDK native-stream binding manifest")
	lock := flag.String("manifest-sha256", "", "expected manifest SHA256 (required)")
	output := flag.String("out", "", "generated C++ header")
	bindings := flag.String("bindings-out", "", "generated function binding JSON")
	runtime := flag.String("runtime-out", "", "export generic native-stream C++ runtime header")
	profile := flag.String("profile", streamir.Profile, "explicit native-stream language profile (v1, v2 or incremental v3)")
	entry := flag.String("entry", "", "public Class.Method for a typed native entry header")
	entryHeader := flag.String("entry-header", "", "optional generated native entry header")
	entryNamespace := flag.String("entry-namespace", "pipelang_entry_Default", "namespace for the native entry header")
	flag.Parse()
	if *runtime != "" {
		if *source != "" || *manifest != "" || *lock != "" || *output != "" || *bindings != "" || *profile != streamir.Profile || *entry != "" || *entryHeader != "" || *entryNamespace != "pipelang_entry_Default" || flag.NArg() != 0 {
			return fmt.Errorf("runtime export cannot be combined with compilation")
		}
		return os.WriteFile(*runtime, streamcpp.RuntimeHeader(), 0644)
	}
	if *source == "" || *manifest == "" || *lock == "" || *output == "" || *bindings == "" || flag.NArg() != 0 {
		return fmt.Errorf("require --source, --manifest, --manifest-sha256, --out and --bindings-out")
	}
	src, err := boundedFile(*source, 1<<20)
	if err != nil {
		return err
	}
	metadata, err := boundedFile(*manifest, 65536)
	if err != nil {
		return err
	}
	program, err := pipelang.CompileNativeStreamsWithProfile(pipelang.SourceInput{Path: *source, Data: src}, []pipelang.NativeStreamDependency{{Manifest: metadata, SHA256: *lock}}, *profile)
	if err != nil {
		return err
	}
	generated, err := streamcpp.Generate(program)
	if err != nil {
		return err
	}
	var facade []byte
	if *entry != "" || *entryHeader != "" {
		if *entry == "" || *entryHeader == "" || filepath.Clean(filepath.Dir(*entryHeader)) != filepath.Clean(filepath.Dir(*output)) {
			return fmt.Errorf("entry/header must be supplied together, beside generated source")
		}
		if !regexp.MustCompile(`^pipelang_entry_[A-Za-z][A-Za-z0-9_]*$`).MatchString(*entryNamespace) || strings.Contains(*entryNamespace, "__") {
			return fmt.Errorf("invalid entry namespace")
		}
		filename := filepath.Base(*output)
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(filename) {
			return fmt.Errorf("invalid generated header filename")
		}
		for _, f := range generated.Functions {
			if f.Class+"."+f.Method == *entry {
				facade = []byte(fmt.Sprintf("#pragma once\n#include %q\nnamespace %s { inline constexpr auto invoke = &%s; }\n", filename, *entryNamespace, f.Symbol))
				break
			}
		}
		if facade == nil {
			return fmt.Errorf("unknown public entry %q", *entry)
		}
	} else if *entryNamespace != "pipelang_entry_Default" {
		return fmt.Errorf("entry namespace requires entry header")
	}
	data, err := json.MarshalIndent(struct {
		Profile        string
		SourceSHA256   string
		ManifestSHA256 string
		Functions      []streamcpp.FunctionBinding
	}{program.Profile, program.SourceSHA256, *lock, generated.Functions}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(*output, generated.Source, 0644); err != nil {
		return err
	}
	if facade != nil {
		if err = os.WriteFile(*entryHeader, facade, 0644); err != nil {
			return err
		}
	}
	return os.WriteFile(*bindings, append(data, '\n'), 0644)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pipelang-streamc:", err)
		os.Exit(1)
	}
}
