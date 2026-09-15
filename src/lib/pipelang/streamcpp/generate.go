// Package streamcpp emits native C++ for the explicit stream effect profile.
// It consumes validated typed stream IR, never parser nodes or SDK implementation.
package streamcpp

import (
	_ "embed"
	"fmt"
	"strings"

	"dockpipe/src/lib/pipelang/streamir"
)

//go:embed runtime.hpp
var runtimeHeader string

func RuntimeHeader() []byte { return []byte(runtimeHeader) }

type FunctionBinding struct {
	Class, Method, Symbol string
	ReturnType            streamir.Type   `json:",omitempty"`
	Parameters            []streamir.Type `json:",omitempty"`
}
type Generated struct {
	Source    []byte
	Functions []FunctionBinding
}

func Generate(program streamir.Program) (Generated, error) {
	if err := streamir.Validate(program); err != nil {
		return Generated{}, err
	}
	if program.Profile == streamir.CompositionProfile {
		return generateComposition(program), nil
	}
	var text strings.Builder
	fmt.Fprintf(&text, "// %s; source SHA256 %s\n", program.Profile, program.SourceSHA256)
	text.WriteString(runtimeHeader)
	text.WriteString("\nnamespace pl_native_stream {\n")
	result := Generated{}
	for _, fn := range program.Functions {
		binding := program.Bindings[fn.Binding]
		symbol := "stream_" + streamir.Digest([]byte(fn.Class + "." + fn.Name))[:24]
		result.Functions = append(result.Functions, FunctionBinding{Class: fn.Class, Method: fn.Name, Symbol: "pl_native_stream::" + symbol})
		fmt.Fprintf(&text, "inline pl_stream_v1::Result %s(pl_stream_v1::Host& host", symbol)
		for i, typ := range fn.Parameters {
			cpp := map[streamir.Type]string{streamir.ReadStream: "pl_stream_v1::ReadStream&", streamir.WriteStream: "pl_stream_v1::WriteStream&", streamir.Int: "std::int64_t"}[typ]
			fmt.Fprintf(&text, ", %s p%d", cpp, i)
		}
		fmt.Fprintf(&text, ") noexcept {\n  return pl_stream_v1::call(host, {%q, %q, %q}, p%d, p%d, p%d);\n}\n",
			binding.Package, binding.Operation.ID, binding.ManifestSHA256, fn.Arguments[0], fn.Arguments[1], fn.Arguments[2])
	}
	text.WriteString("}\n")
	result.Source = []byte(text.String())
	return result, nil
}
