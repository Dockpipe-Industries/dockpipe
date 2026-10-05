package pipelang

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/streamcpp"
	"dockpipe/src/lib/pipelang/streamir"
)

const nativeStreamSource = `public Class Transfer {
 public StreamResult Run(WriteStream output, int limit, ReadStream input) => Codec.transfer(input, output, limit);
}`

func streamDependency(t *testing.T) NativeStreamDependency {
	t.Helper()
	data, err := json.Marshal(streamir.Manifest{Profile: streamir.Profile, Package: "example.codec", ABI: 1, Operations: []streamir.Operation{{Name: "Codec.transfer", ID: "transfer.v1"}}})
	if err != nil {
		t.Fatal(err)
	}
	return NativeStreamDependency{Manifest: data, SHA256: streamir.Digest(data)}
}
func TestNativeStreamsCompile(t *testing.T) {
	dep := streamDependency(t)
	program, err := CompileNativeStreams(SourceInput{Path: "transfer.pipe", Data: []byte(nativeStreamSource)}, []NativeStreamDependency{dep})
	if err != nil {
		t.Fatal(err)
	}
	if program.Functions[0].Arguments != [3]int{2, 0, 1} {
		t.Fatal("source argument binding lost")
	}
	if program.SourceSHA256 != streamir.Digest([]byte(nativeStreamSource)) || program.Bindings[0].ManifestSHA256 != dep.SHA256 {
		t.Fatal("identity lost")
	}
	a, err := streamcpp.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	b, err := streamcpp.Generate(program)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic codegen")
	}
	if !strings.Contains(string(a.Source), "p2, p0, p1") || !strings.Contains(string(a.Source), dep.SHA256) {
		t.Fatal("native binding missing")
	}
	// The default compiler has no implicit host-effect/native-library lane.
	if _, err := Compile([]byte(nativeStreamSource), "Transfer"); err == nil {
		t.Fatal("ordinary compiler admitted stream effects")
	}
}
func TestNativeStreamsRefusals(t *testing.T) {
	dep := streamDependency(t)
	for name, source := range map[string]string{
		"SDK namespace class collision":     strings.Replace(nativeStreamSource, "Class Transfer", "Class Codec", 1),
		"SDK namespace parameter collision": strings.Replace(nativeStreamSource, "ReadStream input", "ReadStream Codec", 1),
		"deep source":                       strings.Replace(nativeStreamSource, "Codec.transfer(input, output, limit)", strings.Repeat("(", 65)+"Codec.transfer(input, output, limit)"+strings.Repeat(")", 65), 1),
		"unknown operation":                 strings.Replace(nativeStreamSource, "Codec.transfer", "Codec.unknown", 1),
		"wrong stream direction":            strings.Replace(nativeStreamSource, "(input, output, limit)", "(output, input, limit)", 1),
		"duplicate borrow":                  strings.Replace(nativeStreamSource, "(input, output, limit)", "(input, input, limit)", 1),
		"forged handle":                     strings.Replace(nativeStreamSource, "(input, output, limit)", "(1, output, limit)", 1),
		"undeclared parameter":              strings.Replace(nativeStreamSource, "(input, output, limit)", "(other, output, limit)", 1),
		"nested effects":                    strings.Replace(nativeStreamSource, "(input, output, limit)", "(input, output, Codec.transfer(input, output, limit))", 1),
		"arithmetic":                        strings.Replace(nativeStreamSource, "output, limit);", "output, limit + 1);", 1),
		"wrong return":                      strings.Replace(nativeStreamSource, "StreamResult", "int", 1),
		"stored capability":                 strings.Replace(nativeStreamSource, "public Class Transfer {", "public Class Transfer { public ReadStream Saved;", 1),
		"duplicate parameter":               strings.Replace(nativeStreamSource, "ReadStream input", "ReadStream output", 1),
		"duplicate class":                   nativeStreamSource + nativeStreamSource,
		"private":                           strings.Replace(nativeStreamSource, "public StreamResult", "private StreamResult", 1),
		"extra parameter":                   strings.Replace(nativeStreamSource, "ReadStream input)", "ReadStream input, int extra)", 1),
		"generic handle":                    strings.Replace(nativeStreamSource, "ReadStream input", "ReadStream<int> input", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CompileNativeStreams(SourceInput{Path: "bad.pipe", Data: []byte(source)}, []NativeStreamDependency{dep}); err == nil {
				t.Fatal("accepted excluded source")
			}
		})
	}
	for name, bad := range map[string]NativeStreamDependency{
		"stale lock":     {Manifest: append(append([]byte{}, dep.Manifest...), ' '), SHA256: dep.SHA256},
		"missing lock":   {Manifest: dep.Manifest},
		"unknown fields": {Manifest: []byte(`{"profile":"pipelang.native-stream.v1","evil":true}`)},
		"trailing":       {Manifest: append(append([]byte{}, dep.Manifest...), []byte(` {}`)...)},
		"wrong ABI":      {Manifest: []byte(strings.Replace(string(dep.Manifest), `"abi":1`, `"abi":2`, 1))},
	} {
		t.Run(name, func(t *testing.T) {
			if name != "stale lock" && name != "missing lock" {
				bad.SHA256 = streamir.Digest(bad.Manifest)
			}
			if _, err := CompileNativeStreams(SourceInput{Path: "input.pipe", Data: []byte(nativeStreamSource)}, []NativeStreamDependency{bad}); err == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
	if _, err := CompileNativeStreams(SourceInput{Path: "input.pipe", Data: []byte(nativeStreamSource)}, []NativeStreamDependency{dep, dep}); err == nil {
		t.Fatal("duplicate dependency admitted")
	}
}
func TestNativeStreamsForgedIR(t *testing.T) {
	for name, mutate := range map[string]func(*streamir.Program){
		"profile":            func(p *streamir.Program) { p.Profile = "pure" },
		"manifest identity":  func(p *streamir.Program) { p.Bindings[0].ManifestSHA256 = "" },
		"wrong type":         func(p *streamir.Program) { p.Functions[0].Parameters[0] = streamir.Int },
		"bad position":       func(p *streamir.Program) { p.Functions[0].Arguments[1] = 7 },
		"unknown binding":    func(p *streamir.Program) { p.Functions[0].Binding = 9 },
		"symbol injection":   func(p *streamir.Program) { p.Bindings[0].Operation.ID = "x\";std::abort();" },
		"duplicate function": func(p *streamir.Program) { p.Functions = append(p.Functions, p.Functions[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			p, err := CompileNativeStreams(SourceInput{Path: "input.pipe", Data: []byte(nativeStreamSource)}, []NativeStreamDependency{streamDependency(t)})
			if err != nil {
				t.Fatal(err)
			}
			mutate(&p)
			if _, err := streamcpp.Generate(p); err == nil {
				t.Fatal("backend accepted forged IR")
			}
		})
	}
}
