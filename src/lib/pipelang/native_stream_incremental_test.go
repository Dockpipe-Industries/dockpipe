package pipelang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/streamcpp"
	"dockpipe/src/lib/pipelang/streameval"
	"dockpipe/src/lib/pipelang/streamir"
)

const incrementalSource = `public Class Flow {
 public StreamStep Run(StreamSession session, InputBuffer input, OutputBuffer output, bool final) {
  StreamStep step = Codec.step(session, input, output, final);
  if (!step.ok) { return step; }
  return step;
 }
 public bool Paused(StreamStep step) => step.needOutput;
 public bool Waiting(StreamStep step) => step.needInput;
 public bool Finished(StreamStep step) => step.done;
 public uint64 Consumed(StreamStep step) => step.inputConsumed;
 public uint64 Written(StreamStep step) => step.outputWritten;
 public uint64 Total(StreamStep step) => step.outputBytes;
 public bool Skip(StreamSession session, InputBuffer input, OutputBuffer output, bool final)
  => false && Codec.step(session, input, output, final).done;
}`

func incrementalDependency() NativeStreamDependency {
	data := []byte(`{"profile":"pipelang.native-stream.v3","package":"example.codec","abi":1,"operations":[{"name":"Codec.step","id":"step.v1"}]}`)
	return NativeStreamDependency{Manifest: data, SHA256: streamir.Digest(data)}
}
func incrementalProgram(t *testing.T, source string) streamir.Program {
	t.Helper()
	p, err := CompileNativeStreamsWithProfile(SourceInput{Path: "incremental.pipe", Data: []byte(source)}, []NativeStreamDependency{incrementalDependency()}, streamir.IncrementalProfile)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type incrementalMock struct{ mode int }

func (h incrementalMock) Invoke(streamir.Binding, any, any, uint64) streameval.Outcome {
	panic("unexpected blocking call")
}
func (h incrementalMock) InvokeStep(b streamir.Binding, s, in, out any, final bool) streameval.StepOutcome {
	if b.Package != "example.codec" || b.Operation.ID != "step.v1" || b.ManifestSHA256 != incrementalDependency().SHA256 || s != "session" || in != "input" || out != "output" || !final {
		panic("binding/argument mismatch")
	}
	r := streameval.StepOutcome{Outcome: streameval.Outcome{Status: streamir.OK, InputBytes: 11, OutputBytes: 1<<63 + 7, Chunks: 1}, Progress: streamir.NeedOutput, InputConsumed: 3, OutputWritten: 2}
	switch h.mode {
	case 1:
		r.Progress = streamir.NeedInput
	case 2:
		r.Progress = streamir.Done
	case 3:
		r.Status = streamir.InvalidStream
	case 4:
		r.Status = 99
	case 5:
		r.Progress = 99
	case 6:
		r.InputConsumed = 99
	case 7:
		panic("host exception")
	case 8:
		r.OutputWritten = 99
	case 9:
		r.InputBytes = 1
	}
	return r
}
func TestNativeStreamsIncrementalReferenceAndRuntime(t *testing.T) {
	p := incrementalProgram(t, incrementalSource)
	generated, err := streamcpp.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range generated.Functions {
		names[f.Method] = f.Symbol
	}
	args := []streameval.Value{{Type: streamir.Session, Capability: "session", Available: true}, {Type: streamir.InputBuffer, Capability: "input", Available: true, Extent: 8}, {Type: streamir.OutputBuffer, Capability: "output", Available: true, Extent: 8}, {Type: streamir.Bool, Bool: true}}
	var expected strings.Builder
	for mode := 0; mode < 10; mode++ {
		want := streameval.StepOutcome{Outcome: streameval.Outcome{Status: streamir.OK, InputBytes: 11, OutputBytes: 1<<63 + 7, Chunks: 1}, Progress: streamir.NeedOutput, InputConsumed: 3, OutputWritten: 2}
		switch mode {
		case 1:
			want.Progress = streamir.NeedInput
		case 2:
			want.Progress = streamir.Done
		case 3:
			want.Status = streamir.InvalidStream
			want.Progress = streamir.NoProgress
		case 4, 5, 6, 7, 8, 9:
			want = streameval.StepOutcome{Outcome: streameval.Outcome{Status: streamir.HostFailure}}
		}
		got, e := streameval.Evaluate(p, "Flow", "Run", args, incrementalMock{mode})
		if e != nil || got.Value.Step != want || len(got.Trace) != 1 {
			t.Fatalf("mode %d: %+v %v want %+v", mode, got, e, want)
		}
		fmt.Fprintf(&expected, `host.mode=%d; r=%s(host,session,in,out,true);
 if(r.status!=static_cast<Status>(%d)||r.progress!=static_cast<Progress>(%d)||r.input_consumed!=%d||r.output_written!=%d||r.input_bytes!=%d||r.output_bytes!=%dULL||r.chunks!=%d)return %d;
`, mode, names["Run"], want.Status, want.Progress, want.InputConsumed, want.OutputWritten, want.InputBytes, want.OutputBytes, want.Chunks, 10+mode)
		fields := map[string]bool{"Paused": want.Progress == streamir.NeedOutput, "Waiting": want.Progress == streamir.NeedInput, "Finished": want.Progress == streamir.Done}
		for method, value := range fields {
			got, e = streameval.Evaluate(p, "Flow", method, []streameval.Value{{Type: streamir.Step, Step: want}}, incrementalMock{})
			if e != nil || got.Value.Bool != value || len(got.Trace) != 0 {
				t.Fatalf("field %s: %+v %v", method, got, e)
			}
			fmt.Fprintf(&expected, "if(%s(host,r)!=%t)return 30;\n", names[method], value)
		}
		for method, value := range map[string]uint64{"Consumed": want.InputConsumed, "Written": want.OutputWritten, "Total": want.OutputBytes} {
			got, e = streameval.Evaluate(p, "Flow", method, []streameval.Value{{Type: streamir.Step, Step: want}}, incrementalMock{})
			if e != nil || got.Value.Uint != value {
				t.Fatalf("counter %s: %+v %v", method, got, e)
			}
			fmt.Fprintf(&expected, "if(%s(host,r)!=%dULL)return 31;\n", names[method], value)
		}
	}
	got, e := streameval.Evaluate(p, "Flow", "Skip", args, incrementalMock{})
	if e != nil || got.Value.Bool || len(got.Trace) != 0 {
		t.Fatal("untaken native effect", got, e)
	}
	source := string(generated.Source) + fmt.Sprintf(`
using namespace pl_stream_v1;
struct Mock:Host {
 int mode=0,calls=0;
 Result invoke(const Binding&,ReadStream&,WriteStream&,std::uint64_t)override {throw 1;}
 Step invoke_step(const Binding& b,StreamSession&,InputBuffer in,OutputBuffer out,bool final)override {
  ++calls;if(b.package!="example.codec"||b.operation!="step.v1"||b.manifest_sha256!="%s"||!final||in.size!=8||out.capacity!=8)throw 2;
  Step r{Status::ok,Progress::need_output,3,2,11,(1ULL<<63)+7,1};
  switch(mode){case 1:r.progress=Progress::need_input;break;case 2:r.progress=Progress::done;break;case 3:r.status=Status::invalid_stream;break;case 4:r.status=static_cast<Status>(99);break;case 5:r.progress=static_cast<Progress>(99);break;case 6:r.input_consumed=99;break;case 7:throw 3;case 8:r.output_written=99;break;case 9:r.input_bytes=1;break;}
  return r;
 }
};
int main(){Mock host;StreamSession session;char a[8]{},b[8]{};InputBuffer in{a,8};OutputBuffer out{b,8};Step r{};
%s
 if(host.calls!=10)return 40;
 if(%s(host,session,in,out,true)||host.calls!=10)return 41;
 r=%s(host,session,{nullptr,1},out,true);if(r.status!=Status::invalid_argument||host.calls!=10)return 42;
 r=%s(host,session,in,{a+1,3},true);if(r.status!=Status::invalid_argument||host.calls!=10)return 43;
 return 0;
}
`, incrementalDependency().SHA256, expected.String(), names["Skip"], names["Run"], names["Run"])
	dir := t.TempDir()
	path := filepath.Join(dir, "main.cpp")
	binary := filepath.Join(dir, "native")
	if e := os.WriteFile(path, []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if out, e := exec.CommandContext(ctx, nativeStreamCompiler(t), "-std=c++17", "-O2", path, "-o", binary).CombinedOutput(); e != nil {
		t.Fatalf("compile: %v\n%s", e, out)
	}
	if out, e := exec.CommandContext(ctx, binary).CombinedOutput(); e != nil {
		t.Fatalf("runtime: %v\n%s", e, out)
	}
}
func TestNativeStreamsIncrementalRefusals(t *testing.T) {
	good := `public Class Flow { public StreamStep Run(StreamSession session, InputBuffer input, OutputBuffer output, bool final) => Codec.step(session,input,output,final); }`
	for name, source := range map[string]string{
		"direction":                strings.Replace(good, "session,input,output,final", "session,output,input,final", 1),
		"arity":                    strings.Replace(good, "session,input,output,final", "session,input,output", 1),
		"finish-type":              strings.Replace(good, "bool final", "int final", 1),
		"return-capability":        `public Class Flow {public StreamSession Run(StreamSession s) => s;}`,
		"local-capability":         `public Class Flow {public StreamStep Run(StreamSession s,InputBuffer i,OutputBuffer o,bool f) {StreamSession copy=s;return Codec.step(copy,i,o,f);}}`,
		"constructor":              strings.Replace(good, "Codec.step(session,input,output,final)", "StreamStep()", 1),
		"step-field-on-old-result": `public Class Flow {public bool Run(StreamResult r) => r.done;}`,
		"field-on-buffer":          `public Class Flow {public uint64 Run(InputBuffer b) => b.inputBytes;}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := CompileNativeStreamsWithProfile(SourceInput{Data: []byte(source)}, []NativeStreamDependency{incrementalDependency()}, streamir.IncrementalProfile); e == nil {
				t.Fatal("accepted invalid source")
			}
		})
	}
	dep := incrementalDependency()
	for _, profile := range []string{streamir.Profile, streamir.CompositionProfile} {
		if _, e := CompileNativeStreamsWithProfile(SourceInput{Data: []byte(good)}, []NativeStreamDependency{dep}, profile); e == nil {
			t.Fatal("v3 manifest accepted by old profile")
		}
	}
	old := streamDependency(t)
	if _, e := CompileNativeStreamsWithProfile(SourceInput{Data: []byte(good)}, []NativeStreamDependency{old}, streamir.IncrementalProfile); e == nil {
		t.Fatal("old manifest admitted as incremental")
	}
	// Names introduced by v3 must not become reserved in accepted v2 programs.
	for _, name := range []string{"StreamSession", "InputBuffer", "OutputBuffer", "StreamStep"} {
		source := "public Class " + name + " { public bool Run(bool value) => value; }"
		if _, e := CompileNativeStreamsWithProfile(SourceInput{Data: []byte(source)}, []NativeStreamDependency{old}, streamir.CompositionProfile); e != nil {
			t.Fatalf("v3 reserved a v2 class name: %s: %v", name, e)
		}
	}
	for _, mutate := range []func(*streamir.Program){
		func(p *streamir.Program) { p.Profile = streamir.CompositionProfile },
		func(p *streamir.Program) { p.Functions[0].ReturnType = streamir.Session },
		func(p *streamir.Program) { p.Functions[0].Body[0].Value.Arguments[1].Type = streamir.OutputBuffer },
		func(p *streamir.Program) {
			p.Functions[0].Body[0].Value.Arguments = p.Functions[0].Body[0].Value.Arguments[:3]
		},
	} {
		p := incrementalProgram(t, good)
		mutate(&p)
		if _, e := streamcpp.Generate(p); e == nil {
			t.Fatal("forged IR accepted")
		}
	}
}
