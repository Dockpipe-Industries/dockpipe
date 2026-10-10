package pipelang

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/streamcpp"
)

func nativeStreamCompiler(t *testing.T) string {
	t.Helper()
	if compiler := os.Getenv("PIPELANG_TEST_CXX"); compiler != "" {
		if !filepath.IsAbs(compiler) {
			t.Fatal("PIPELANG_TEST_CXX must name the declared absolute compiler")
		}
		return compiler
	}
	return "c++"
}

func TestNativeStreamsGeneratedRuntime(t *testing.T) {
	dep := streamDependency(t)
	program, err := CompileNativeStreams(SourceInput{Path: "transfer.pipe", Data: []byte(nativeStreamSource)}, []NativeStreamDependency{dep})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := streamcpp.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "main.cpp")
	source := string(generated.Source) + fmt.Sprintf(`
#include <stdexcept>
using namespace pl_stream_v1;
struct Mock : Host {
 int calls=0; int mode=0; void* source=nullptr; void* sink=nullptr;
 Result invoke(const Binding& b,ReadStream& input,WriteStream& output,std::uint64_t limit) override {
  ++calls;
  if(b.package!="example.codec"||b.operation!="transfer.v1"||b.manifest_sha256!="%s"||input.context!=source||output.context!=sink||limit!=42)throw std::runtime_error("binding");
  if(mode==1)throw 1;
  if(mode==2)return {static_cast<Status>(99)};
  return {Status::io_error,11,7,1};
 }
};
int read_cb(void*,void*,std::size_t,std::size_t*) {return 1;}
int write_cb(void*,const void*,std::size_t,std::size_t*) {return 1;}
int main() {
 int a=1,b=2;Mock host;host.source=&a;host.sink=&b;
 ReadStream input{read_cb,&a};WriteStream output{write_cb,&b};
 auto call=[&](std::int64_t limit){return %s(host,output,limit,input);};
 auto r=call(42);
 if(r.status!=Status::io_error||r.input_bytes!=11||r.output_bytes!=7||r.chunks!=1||host.calls!=1)return 1;
 r=call(-1);if(r.status!=Status::invalid_argument||host.calls!=1)return 2;
 auto saved=input.read;input.read=nullptr;r=call(42);if(r.status!=Status::invalid_argument||host.calls!=1)return 3;input.read=saved;
 host.mode=1;r=call(42);if(r.status!=Status::host_failure||host.calls!=2)return 4;
 host.mode=2;r=call(42);if(r.status!=Status::host_failure||host.calls!=3)return 5;
 return 0;
}
`, dep.SHA256, generated.Functions[0].Symbol)
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary := filepath.Join(dir, "native")
	if out, err := exec.CommandContext(ctx, nativeStreamCompiler(t), "-std=c++17", "-O2", path, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("native stream compile: %v\n%s", err, out)
	}
	if out, err := exec.CommandContext(ctx, binary).CombinedOutput(); err != nil {
		t.Fatalf("native stream runtime: %v\n%s", err, out)
	}
}
