package streamcpp

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/pipelang/streamir"
)

func cppType(t streamir.Type) string {
	return map[streamir.Type]string{streamir.Int: "std::int64_t", streamir.Uint64: "std::uint64_t", streamir.Bool: "bool", streamir.ReadStream: "pl_stream_v1::ReadStream&", streamir.WriteStream: "pl_stream_v1::WriteStream&", streamir.Result: "pl_stream_v1::Result", streamir.StatusType: "pl_stream_v1::Status"}[t]
}
func compositionSymbol(p streamir.Program, f streamir.Function) string {
	// Distinct application sources and SDK pins must not share inline symbols
	// when consumers link several generated targets into the same application.
	identity := p.SourceSHA256
	for _, binding := range p.Bindings {
		identity += "/" + binding.ManifestSHA256
	}
	return "stream_" + streamir.Digest([]byte(identity + "/" + f.Class + "." + f.Name))[:24]
}
func signature(p streamir.Program, f streamir.Function) string {
	var b strings.Builder
	fmt.Fprintf(&b, "inline %s %s(pl_stream_v1::Host& host", cppType(f.ReturnType), compositionSymbol(p, f))
	for i, t := range f.Parameters {
		fmt.Fprintf(&b, ", %s v%d", cppType(t), i)
	}
	b.WriteString(") noexcept")
	return b.String()
}
func generateComposition(p streamir.Program) Generated {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s; source SHA256 %s\n", p.Profile, p.SourceSHA256)
	b.WriteString(runtimeHeader)
	b.WriteString("\nnamespace pl_native_stream {\n")
	for _, f := range p.Functions {
		b.WriteString(signature(p, f) + ";\n")
	}
	result := Generated{}
	for _, f := range p.Functions {
		if f.Public {
			result.Functions = append(result.Functions, FunctionBinding{Class: f.Class, Method: f.Name, Symbol: "pl_native_stream::" + compositionSymbol(p, f), ReturnType: f.ReturnType, Parameters: append([]streamir.Type(nil), f.Parameters...)})
		}
		b.WriteString(signature(p, f) + " {\n")
		for i, t := range f.Parameters {
			if t == streamir.Result || t == streamir.StatusType {
				fmt.Fprintf(&b, "  v%d = pl_stream_v1::normalized(v%d);\n", i, i)
			}
		}
		writeCompositionBlock(&b, p, f.Body, "  ")
		b.WriteString("}\n")
	}
	b.WriteString("}\n")
	result.Source = []byte(b.String())
	return result
}
func writeCompositionBlock(b *strings.Builder, p streamir.Program, body []streamir.Statement, indent string) {
	for _, s := range body {
		switch s.Kind {
		case "local":
			fmt.Fprintf(b, "%sconst %s v%d = %s;\n", indent, cppType(s.Type), s.Slot, compositionExpression(p, s.Value))
		case "return":
			fmt.Fprintf(b, "%sreturn %s;\n", indent, compositionExpression(p, s.Value))
		case "if":
			fmt.Fprintf(b, "%sif (%s) {\n", indent, compositionExpression(p, s.Value))
			writeCompositionBlock(b, p, s.Then, indent+"  ")
			b.WriteString(indent + "}")
			if len(s.Else) > 0 {
				b.WriteString(" else {\n")
				writeCompositionBlock(b, p, s.Else, indent+"  ")
				b.WriteString(indent + "}")
			}
			b.WriteString("\n")
		case "block":
			b.WriteString(indent + "{\n")
			writeCompositionBlock(b, p, s.Then, indent+"  ")
			b.WriteString(indent + "}\n")
		}
	}
}
func compositionExpression(p streamir.Program, x *streamir.Expression) string {
	arg := func(i int) string { return compositionExpression(p, x.Arguments[i]) }
	switch x.Kind {
	case "variable":
		return fmt.Sprintf("v%d", x.Slot)
	case "int":
		if x.Int == (-1 << 63) {
			return "(-9223372036854775807LL-1)"
		}
		return fmt.Sprintf("std::int64_t{%dLL}", x.Int)
	case "uint64":
		return fmt.Sprintf("std::uint64_t{%dULL}", x.Uint)
	case "bool":
		if x.Bool {
			return "true"
		}
		return "false"
	case "status":
		return "pl_stream_v1::Status::" + []string{"ok", "invalid_argument", "invalid_stream", "unsupported", "limit_exceeded", "io_error", "host_failure", "denied"}[x.Status]
	case "field":
		if x.Member == "ok" {
			return "((" + arg(0) + ").status == pl_stream_v1::Status::ok)"
		}
		return "(" + arg(0) + ")." + map[string]string{"status": "status", "inputBytes": "input_bytes", "outputBytes": "output_bytes", "chunks": "chunks"}[x.Member]
	case "not":
		return "(!(" + arg(0) + "))"
	case "conditional":
		return "((" + arg(0) + ") ? (" + arg(1) + ") : (" + arg(2) + "))"
	case "binary":
		if x.Member == "&&" || x.Member == "||" {
			return "((" + arg(0) + ") " + x.Member + " (" + arg(1) + "))"
		}
		return "([&]() -> bool { auto left = " + arg(0) + "; auto right = " + arg(1) + "; return left " + x.Member + " right; }())"
	case "native", "helper":
		var b strings.Builder
		fmt.Fprintf(&b, "([&]() -> %s { ", cppType(x.Type))
		for i, a := range x.Arguments {
			ref := ""
			if a.Type == streamir.ReadStream || a.Type == streamir.WriteStream {
				ref = "&"
			}
			fmt.Fprintf(&b, "auto%s a%d = %s; ", ref, i, arg(i))
		}
		if x.Kind == "native" {
			binding := p.Bindings[x.Target]
			fmt.Fprintf(&b, "return pl_stream_v1::call(host, {%q, %q, %q}", binding.Package, binding.Operation.ID, binding.ManifestSHA256)
		} else {
			fmt.Fprintf(&b, "return %s(host", compositionSymbol(p, p.Functions[x.Target]))
		}
		for i := range x.Arguments {
			fmt.Fprintf(&b, ", a%d", i)
		}
		b.WriteString("); }())")
		return b.String()
	}
	panic("validated stream expression missing codegen")
}
