package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/streamir"
)

func TestEntryFacade(t *testing.T) {
	oldArgs, oldFlags := os.Args, flag.CommandLine
	defer func() { os.Args, flag.CommandLine = oldArgs, oldFlags }()
	for _, test := range []struct {
		name, entry, namespace, profile string
		header                          bool
		want                            string
	}{
		{"public", "Flow.Run", "pipelang_entry_Flow", streamir.CompositionProfile, true, ""},
		{"incremental", "Flow.Run", "pipelang_entry_Flow", streamir.IncrementalProfile, true, ""},
		{"private", "Flow.Good", "pipelang_entry_Flow", streamir.CompositionProfile, true, "unknown public entry"},
		{"unknown", "Flow.Missing", "pipelang_entry_Flow", streamir.CompositionProfile, true, "unknown public entry"},
		{"namespace", "Flow.Run", "injected;", streamir.CompositionProfile, true, "invalid entry namespace"},
		{"missing header", "Flow.Run", "pipelang_entry_Flow", streamir.CompositionProfile, false, "entry/header"},
		{"default remains v1", "Flow.Run", "pipelang_entry_Flow", "", true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "Flow.pipe")
			manifest := filepath.Join(dir, "manifest.json")
			data, _ := json.Marshal(streamir.Manifest{Profile: streamir.ManifestProfile(test.profile), Package: "example.codec", ABI: 1, Operations: []streamir.Operation{{Name: "Codec.transfer", ID: "transfer.v1"}}})
			if err := os.WriteFile(manifest, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte(`public Class Flow {
    private bool Good(StreamResult result) => result.ok;
    public StreamResult Run(ReadStream input, WriteStream output, int limit) {
      StreamResult result = Codec.transfer(input, output, limit);
      if (Good(result)) { return result; }
      return result;
    }
   }`), 0600); err != nil {
				t.Fatal(err)
			}
			if test.profile == streamir.IncrementalProfile {
				if err := os.WriteFile(source, []byte(`public Class Flow {
                    public StreamStep Run(StreamSession session, InputBuffer input, OutputBuffer output, bool final)
                        => Codec.transfer(session, input, output, final);
                }`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out := filepath.Join(dir, "program.hpp")
			binding := filepath.Join(dir, "bindings.json")
			entry := filepath.Join(dir, "entry.hpp")
			os.Args = []string{"pipelang-streamc", "--source", source, "--manifest", manifest, "--manifest-sha256", streamir.Digest(data), "--out", out, "--bindings-out", binding, "--entry", test.entry, "--entry-namespace", test.namespace}
			if test.header {
				os.Args = append(os.Args, "--entry-header", entry)
			}
			if test.profile != "" {
				os.Args = append(os.Args, "--profile", test.profile)
			}
			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
			err := run()
			if test.want != "" || test.profile == "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("expected rejection %q, got %v", test.want, err)
				}
				for _, p := range []string{out, binding, entry} {
					if _, e := os.Stat(p); !os.IsNotExist(e) {
						t.Fatalf("rejection created output %s", p)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var bindings struct {
				Functions []struct{ Class, Method, Symbol string }
			}
			b, e := os.ReadFile(binding)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(b, &bindings); e != nil {
				t.Fatal(e)
			}
			if len(bindings.Functions) != 1 {
				t.Fatal("private method exported")
			}
			b, e = os.ReadFile(entry)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(string(b), "&"+bindings.Functions[0].Symbol) || !strings.Contains(string(b), `#include "program.hpp"`) {
				t.Fatal("entry does not match public binding")
			}
		})
	}
}
