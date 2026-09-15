package pipelang

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/streamcpp"
	"dockpipe/src/lib/pipelang/streameval"
	"dockpipe/src/lib/pipelang/streamir"
)

const compositionSource = `public Class Flow {
 private bool Succeeded(StreamResult result) => result.status == StreamStatus.ok;
 private bool Pair(StreamResult first, StreamResult second) => first.ok && second.ok;
 public StreamResult Run(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit) {
  StreamResult compressed = Codec.encode(input, packed, limit);
  if (!Succeeded(compressed)) { return compressed; }
  StreamResult decoded = Codec.decode(stored, output, limit);
  return decoded;
 }
 public bool And(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Codec.encode(input, packed, limit).ok && Codec.decode(stored, output, limit).ok;
 public bool Or(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Codec.encode(input, packed, limit).ok || Codec.decode(stored, output, limit).ok;
 public bool Eager(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Pair(Codec.encode(input, packed, limit), Codec.decode(stored, output, limit));
 public bool Compare(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Codec.encode(input, packed, limit).outputBytes < Codec.decode(stored, output, limit).outputBytes;
 public uint64 Bytes(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit) {
  StreamResult result = Codec.encode(input, packed, limit);
  uint64 bytes = result.outputBytes;
  return bytes;
 }
 public StreamStatus Status(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Codec.encode(input, packed, limit).status;
 public bool Positive(ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => Codec.encode(input, packed, limit).outputBytes > 0;
 public StreamResult Choose(bool choose, ReadStream input, WriteStream packed, ReadStream stored, WriteStream output, int limit)
  => choose ? Codec.encode(input, packed, limit) : Codec.decode(stored, output, limit);
 public bool InputIsValid(StreamResult input) => input.ok;
}`

func compositionDependency(t *testing.T) NativeStreamDependency {
	t.Helper()
	m := streamir.Manifest{Profile: streamir.Profile, Package: "example.codec", ABI: 1, Operations: []streamir.Operation{{Name: "Codec.encode", ID: "encode.v1"}, {Name: "Codec.decode", ID: "decode.v1"}}}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return NativeStreamDependency{Manifest: data, SHA256: streamir.Digest(data)}
}
func compileComposition(t *testing.T, source string) streamir.Program {
	t.Helper()
	p, err := CompileNativeStreamsWithProfile(SourceInput{Path: "flow.pipe", Data: []byte(source)}, []NativeStreamDependency{compositionDependency(t)}, streamir.CompositionProfile)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestNativeStreamCompositionSource(t *testing.T) {
	p := compileComposition(t, compositionSource)
	if len(p.Functions) != 12 {
		t.Fatalf("functions=%d", len(p.Functions))
	}
	a, e := streamcpp.Generate(p)
	if e != nil {
		t.Fatal(e)
	}
	b, e := streamcpp.Generate(p)
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("unstable composition output")
	}
	other := compileComposition(t, compositionSource+"\n// another application module\n")
	c, err := streamcpp.Generate(other)
	if err != nil || c.Functions[0].Symbol == a.Functions[0].Symbol {
		t.Fatal("distinct sources share native symbols", err)
	}
	other.Bindings[0].ManifestSHA256 = streamir.Digest([]byte("other pinned SDK"))
	d, err := streamcpp.Generate(other)
	if err != nil || d.Functions[0].Symbol == c.Functions[0].Symbol {
		t.Fatal("distinct SDK pins share native symbols", err)
	}
	if len(a.Functions) != 10 {
		t.Fatal("private helper exported in binding map")
	}
	if _, err := CompileNativeStreams(SourceInput{Path: "flow.pipe", Data: []byte(compositionSource)}, []NativeStreamDependency{compositionDependency(t)}); err == nil {
		t.Fatal("v1 silently widened")
	}
	if _, err := CompileNativeStreamsWithProfile(SourceInput{Path: "flow.pipe", Data: []byte(compositionSource)}, []NativeStreamDependency{compositionDependency(t)}, "unversioned"); err == nil {
		t.Fatal("unknown profile admitted")
	}
}

type compositionHost struct{ mode int }

func (h compositionHost) Invoke(binding streamir.Binding, input, output any, limit uint64) streameval.Outcome {
	if limit != 42 {
		panic("limit")
	}
	if binding.Operation.ID == "encode.v1" {
		if input != "input" || output != "packed" {
			panic("encode handles")
		}
		switch h.mode {
		case 1:
			return streameval.Outcome{Status: streamir.LimitExceeded, InputBytes: 11, OutputBytes: 7, Chunks: 1}
		case 2:
			panic("host failed")
		case 3:
			return streameval.Outcome{Status: 99}
		case 4:
			return streameval.Outcome{Status: streamir.OK, OutputBytes: 1<<63 + 5}
		}
		return streameval.Outcome{Status: streamir.OK, InputBytes: 100, OutputBytes: 50, Chunks: 2}
	}
	if binding.Operation.ID != "decode.v1" || input != "stored" || output != "output" {
		panic("decode identity/handles")
	}
	return streameval.Outcome{Status: streamir.OK, InputBytes: 50, OutputBytes: 100, Chunks: 2}
}
func compositionArgs() []streameval.Value {
	return []streameval.Value{
		{Type: streamir.ReadStream, Capability: "input", Available: true}, {Type: streamir.WriteStream, Capability: "packed", Available: true},
		{Type: streamir.ReadStream, Capability: "stored", Available: true}, {Type: streamir.WriteStream, Capability: "output", Available: true}, {Type: streamir.Int, Int: 42},
	}
}
func compositionValue(v streameval.Value) string {
	switch v.Type {
	case streamir.Result:
		r := v.Result
		return fmt.Sprintf("result:%d:%d:%d:%d", r.Status, r.InputBytes, r.OutputBytes, r.Chunks)
	case streamir.Bool:
		if v.Bool {
			return "bool:1"
		}
		return "bool:0"
	case streamir.Uint64:
		return fmt.Sprintf("uint64:%d", v.Uint)
	case streamir.StatusType:
		return fmt.Sprintf("status:%d", v.Status)
	}
	panic("unexpected value")
}
func TestNativeStreamCompositionValueTrace(t *testing.T) {
	p := compileComposition(t, compositionSource)
	cases := []struct {
		method          string
		mode            int
		choose          bool
		expected, trace string
	}{
		{"Run", 0, false, "result:0:50:100:2", "encode.v1;decode.v1;"},
		{"Run", 1, false, "result:4:11:7:1", "encode.v1;"},
		{"Run", 2, false, "result:6:0:0:0", "encode.v1;"},
		{"Run", 3, false, "result:6:0:0:0", "encode.v1;"},
		{"And", 1, false, "bool:0", "encode.v1;"},
		{"And", 0, false, "bool:1", "encode.v1;decode.v1;"},
		{"Or", 0, false, "bool:1", "encode.v1;"},
		{"Or", 1, false, "bool:1", "encode.v1;decode.v1;"},
		{"Eager", 1, false, "bool:0", "encode.v1;decode.v1;"},
		{"Compare", 0, false, "bool:1", "encode.v1;decode.v1;"},
		{"Bytes", 4, false, "uint64:9223372036854775813", "encode.v1;"},
		{"Status", 1, false, "status:4", "encode.v1;"},
		{"Positive", 0, false, "bool:1", "encode.v1;"},
		{"Choose", 0, true, "result:0:100:50:2", "encode.v1;"},
		{"Choose", 0, false, "result:0:50:100:2", "decode.v1;"},
	}
	generated, err := streamcpp.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range generated.Functions {
		names[f.Method] = f.Symbol
	}
	var invocations strings.Builder
	var expected []string
	for _, tc := range cases {
		args := compositionArgs()
		nativeArgs := "input, packed, stored, output, 42"
		if tc.method == "Choose" {
			args = append([]streameval.Value{{Type: streamir.Bool, Bool: tc.choose}}, args...)
			nativeArgs = fmt.Sprintf("%t, ", tc.choose) + nativeArgs
		}
		r, err := streameval.Evaluate(p, "Flow", tc.method, args, compositionHost{tc.mode})
		if err != nil {
			t.Fatal(err)
		}
		trace := ""
		for _, entry := range r.Trace {
			trace += strings.TrimPrefix(entry, "example.codec/") + ";"
		}
		if compositionValue(r.Value) != tc.expected || trace != tc.trace {
			t.Fatalf("%s mode %d: %s %s", tc.method, tc.mode, compositionValue(r.Value), trace)
		}
		expected = append(expected, tc.expected+"|"+tc.trace)
		fmt.Fprintf(&invocations, "{ Mock host(%d); auto value=%s(host,%s); emit(value,host); }\n", tc.mode, names[tc.method], nativeArgs)
	}
	// Host-supplied invalid result tags normalize in both implementations.
	r, err := streameval.Evaluate(p, "Flow", "InputIsValid", []streameval.Value{{Type: streamir.Result, Result: streameval.Outcome{Status: 99}}}, compositionHost{})
	if err != nil || r.Value.Bool || len(r.Trace) != 0 {
		t.Fatal("invalid input result normalization")
	}
	fmt.Fprintf(&invocations, "{ Mock host(0); emit(%s(host,Result{static_cast<Status>(99)}),host); }\n", names["InputIsValid"])
	expected = append(expected, "bool:0|")
	args := compositionArgs()
	args[4].Int = -1
	r, err = streameval.Evaluate(p, "Flow", "Run", args, compositionHost{})
	if err != nil || compositionValue(r.Value) != "result:1:0:0:0" || len(r.Trace) != 0 {
		t.Fatal("negative limit called host")
	}
	fmt.Fprintf(&invocations, "{ Mock host(0); emit(%s(host,input,packed,stored,output,-1),host); }\n", names["Run"])
	expected = append(expected, "result:1:0:0:0|")
	cpp := string(generated.Source) + `
#include <iostream>
#include <string>
#include <stdexcept>
using namespace pl_stream_v1;
struct Mock:Host {
 int mode;std::string trace;explicit Mock(int m):mode(m){}
 Result invoke(const Binding& b,ReadStream& input,WriteStream& output,std::uint64_t limit)override{
  trace+=std::string(b.operation)+";";
  if(limit!=42)throw 1;
  if(b.operation=="encode.v1"){
   if(*static_cast<int*>(input.context)!=1||*static_cast<int*>(output.context)!=2)throw 2;
   if(mode==1)return {Status::limit_exceeded,11,7,1};
   if(mode==2)throw 3;
   if(mode==3)return {static_cast<Status>(99)};
   if(mode==4)return {Status::ok,0,9223372036854775813ULL,0};
   return {Status::ok,100,50,2};
  }
  if(b.operation!="decode.v1"||*static_cast<int*>(input.context)!=3||*static_cast<int*>(output.context)!=4)throw 4;
  return {Status::ok,50,100,2};
 }
};
void emit(Result r,const Mock& h){std::cout<<"result:"<<static_cast<unsigned>(r.status)<<":"<<r.input_bytes<<":"<<r.output_bytes<<":"<<r.chunks<<"|"<<h.trace<<"\n";}
void emit(bool r,const Mock& h){std::cout<<"bool:"<<r<<"|"<<h.trace<<"\n";}
void emit(std::uint64_t r,const Mock& h){std::cout<<"uint64:"<<r<<"|"<<h.trace<<"\n";}
void emit(Status r,const Mock& h){std::cout<<"status:"<<static_cast<unsigned>(r)<<"|"<<h.trace<<"\n";}
int read_cb(void*,void*,std::size_t,std::size_t*){return 1;}
int write_cb(void*,const void*,std::size_t,std::size_t*){return 1;}
int main(){int a=1,b=2,c=3,d=4;ReadStream input{read_cb,&a},stored{read_cb,&c};WriteStream packed{write_cb,&b},output{write_cb,&d};
` + invocations.String() + "}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "main.cpp")
	if err := os.WriteFile(path, []byte(cpp), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary := filepath.Join(dir, "native")
	if out, err := exec.CommandContext(ctx, "c++", "-std=c++17", "-O2", path, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err := exec.CommandContext(ctx, binary).CombinedOutput()
	if err != nil {
		t.Fatalf("native: %v\n%s", err, out)
	}
	if string(out) != strings.Join(expected, "\n")+"\n" {
		t.Fatalf("native Value/Trace mismatch:\n%s\nwant:\n%s", out, strings.Join(expected, "\n"))
	}
}

func TestNativeStreamCompositionRefusals(t *testing.T) {
	base := `public Class Flow { public StreamResult Run(ReadStream input, WriteStream output, int limit) { StreamResult result = Codec.encode(input,output,limit); return result; } }`
	for name, source := range map[string]string{
		"missing return":      strings.Replace(base, "return result;", "", 1),
		"unreachable effect":  strings.Replace(base, "return result;", "return result; StreamResult late = Codec.encode(input,output,limit);", 1),
		"self reference":      strings.Replace(base, "Codec.encode(input,output,limit)", "result", 1),
		"wrong direction":     strings.Replace(base, "input,output,limit)", "output,input,limit)", 1),
		"borrow stored":       strings.Replace(base, "StreamResult result = Codec.encode(input,output,limit);", "ReadStream alias = input;", 1),
		"borrow returned":     strings.Replace(strings.Replace(base, "public StreamResult", "public ReadStream", 1), "return result;", "return input;", 1),
		"bad condition":       strings.Replace(base, "return result;", "if (result) { return result; } return result;", 1),
		"bad status":          strings.Replace(base, "return result;", "if (result.status == StreamStatus.missing) { return result; } return result;", 1),
		"result constructor":  strings.Replace(base, "Codec.encode(input,output,limit)", "StreamResult()", 1),
		"branch escape":       strings.Replace(base, "StreamResult result = Codec.encode(input,output,limit);", "if (true) { StreamResult result = Codec.encode(input,output,limit); }", 1),
		"recursion":           strings.Replace(base, "Codec.encode(input,output,limit)", "Run(input,output,limit)", 1),
		"shadowing":           strings.Replace(base, "return result;", "{ bool result = true; } return result;", 1),
		"status ordering":     strings.Replace(base, "return result;", "if (result.status < StreamStatus.ok) { return result; } return result;", 1),
		"count narrowing":     strings.Replace(base, "return result;", "int bytes = result.outputBytes; return result;", 1),
		"arithmetic":          strings.Replace(base, "return result;", "int n = limit + 1; return result;", 1),
		"private cross class": `public Class A {private bool Hidden()=>true;} public Class B {public bool Run()=>A.Hidden();}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CompileNativeStreamsWithProfile(SourceInput{Path: "bad.pipe", Data: []byte(source)}, []NativeStreamDependency{compositionDependency(t)}, streamir.CompositionProfile); err == nil {
				t.Fatal("accepted excluded composition")
			}
		})
	}
}
func TestNativeStreamCompositionForgedIR(t *testing.T) {
	for name, change := range map[string]func(*streamir.Program){
		"unknown tag":        func(p *streamir.Program) { p.Functions[0].Body[0].Value.Arguments[1].Status = 99 },
		"wrong field type":   func(p *streamir.Program) { p.Functions[0].Body[0].Value.Arguments[0].Type = streamir.Int },
		"no all-path return": func(p *streamir.Program) { p.Functions[2].Body = p.Functions[2].Body[:2] },
		"cyclic expression":  func(p *streamir.Program) { x := p.Functions[0].Body[0].Value; x.Arguments[0] = x },
		"forged local slot":  func(p *streamir.Program) { p.Functions[2].Body[0].Slot = 0 },
		"bad native target":  func(p *streamir.Program) { p.Functions[2].Body[0].Value.Target = 999 },
		"unknown statement":  func(p *streamir.Program) { p.Functions[2].Body[0].Kind = "spawn" },
		"helper recursion": func(p *streamir.Program) {
			p.Functions[0].Body[0].Value = &streamir.Expression{Kind: "helper", Type: streamir.Bool, Target: 0, Arguments: []*streamir.Expression{{Kind: "variable", Type: streamir.Result, Slot: 0}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := compileComposition(t, compositionSource)
			change(&p)
			if _, err := streamcpp.Generate(p); err == nil {
				t.Fatal("backend accepted forged composition")
			}
			if _, err := streameval.Evaluate(p, "Flow", "Run", compositionArgs(), compositionHost{}); err == nil {
				t.Fatal("evaluator accepted forged composition")
			}
		})
	}
}
