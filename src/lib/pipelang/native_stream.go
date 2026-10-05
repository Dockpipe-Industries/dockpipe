package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"dockpipe/src/lib/pipelang/streamir"
)

// NativeStreamDependency supplies inert SDK metadata and its caller-pinned digest.
// Compilation neither locates libraries nor grants permission to perform I/O.
type NativeStreamDependency struct {
	Manifest []byte
	SHA256   string
}

// CompileNativeStreams admits a deliberately small, separately selected effect
// profile. Each public method forwards three typed parameters to one declared
// native operation and returns StreamResult. Pure language contracts do not gain
// effects, and this API does not claim general FFI/async/stream ownership support.
func CompileNativeStreams(source SourceInput, dependencies []NativeStreamDependency) (streamir.Program, error) {
	return CompileNativeStreamsWithProfile(source, dependencies, streamir.Profile)
}

// CompileNativeStreamsWithProfile explicitly selects direct v1 calls or v2
// composition. SDK operation manifests retain their ABI-1 contract in both.
func CompileNativeStreamsWithProfile(source SourceInput, dependencies []NativeStreamDependency, profile string) (streamir.Program, error) {
	var result streamir.Program
	if profile != streamir.Profile && profile != streamir.CompositionProfile && profile != streamir.IncrementalProfile {
		return result, fmt.Errorf("unsupported native-stream profile")
	}
	if len(source.Data) == 0 || len(source.Data) > 65536 || len(dependencies) == 0 || len(dependencies) > 16 {
		return result, fmt.Errorf("native-stream source/dependency extent outside profile")
	}
	result.Profile = profile
	result.SourceSHA256 = streamir.Digest(source.Data)
	packages := map[string]bool{}
	aliases := map[string]int{}
	namespaces := map[string]bool{}
	for _, dep := range dependencies {
		if len(dep.Manifest) > 65536 || !streamir.ValidDigest(dep.SHA256) || streamir.Digest(dep.Manifest) != dep.SHA256 {
			return streamir.Program{}, fmt.Errorf("native-stream manifest lock mismatch")
		}
		var manifest streamir.Manifest
		decoder := json.NewDecoder(bytes.NewReader(dep.Manifest))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			return streamir.Program{}, fmt.Errorf("native-stream manifest: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return streamir.Program{}, fmt.Errorf("trailing manifest data")
		}
		if err := streamir.ValidateManifest(manifest); err != nil {
			return streamir.Program{}, err
		}
		if manifest.Profile != streamir.ManifestProfile(profile) {
			return streamir.Program{}, fmt.Errorf("manifest profile does not match selected language profile")
		}
		if packages[manifest.Package] {
			return streamir.Program{}, fmt.Errorf("duplicate native-stream package")
		}
		packages[manifest.Package] = true
		for _, op := range manifest.Operations {
			namespaces[strings.SplitN(op.Name, ".", 2)[0]] = true
			if _, exists := aliases[op.Name]; exists {
				return streamir.Program{}, fmt.Errorf("ambiguous native-stream alias %s", op.Name)
			}
			aliases[op.Name] = len(result.Bindings)
			result.Bindings = append(result.Bindings, streamir.Binding{Package: manifest.Package, ManifestSHA256: dep.SHA256, Operation: op})
		}
	}
	set, diagnostics := NewSourceSet([]SourceInput{source})
	if diagnostics.HasErrors() {
		return streamir.Program{}, diagnosticError(set, diagnostics)
	}
	file := set.Files()[0]
	tokens, err := lex(set, file)
	if err != nil {
		return streamir.Program{}, err
	}
	if len(tokens) > 8192 {
		return streamir.Program{}, fmt.Errorf("native-stream token extent outside profile")
	}
	depth := 0
	for _, token := range tokens {
		switch token.kind {
		case tokLParen, tokLBrace, tokLBracket:
			depth++
		case tokRParen, tokRBrace, tokRBracket:
			depth--
		}
		if depth > 64 || depth < 0 {
			return streamir.Program{}, fmt.Errorf("native-stream delimiter nesting outside profile")
		}
	}
	parser := &parser{sources: set, file: file, languageContract: PipeLangLanguageContractV1130, nativeStreams: true, toks: tokens}
	if profile != streamir.Profile {
		parser.languageContract = PipeLangLanguageContractV1140
	}
	program, err := parser.parseProgram()
	if err != nil {
		return streamir.Program{}, err
	}
	refuse := func(span Span, message string) (streamir.Program, error) {
		return streamir.Program{}, oneDiagnostic(set, CodeInvalidProgram, CategorySemantic, span, message)
	}
	if len(program.Interfaces) != 0 || len(program.Records) != 0 || len(program.Enums) != 0 || len(program.Classes) == 0 {
		return refuse(program.Span, "native-stream profile requires public callable classes only")
	}
	if profile != streamir.Profile {
		return lowerNativeComposition(program, result, namespaces)
	}
	classes := map[string]bool{}
	for _, class := range program.Classes {
		if namespaces[class.Name] || classes[class.Name] || class.Visibility != VisibilityPublic || len(class.Annotations) != 0 || class.Implements != nil || len(class.Fields) != 0 || len(class.Methods) == 0 {
			return refuse(class.Span, "native-stream classes cannot own fields, annotations, inheritance or duplicate identities")
		}
		classes[class.Name] = true
		for _, method := range class.Methods {
			if method.Visibility != VisibilityPublic || len(method.Annotations) != 0 || !nativeStreamType(method.ReturnType, "StreamResult", TypeRefNamed) || len(method.Params) != 3 {
				return refuse(method.Span, "native-stream methods require StreamResult and three borrowed stream/limit parameters")
			}
			f := streamir.Function{Class: class.Name, Name: method.Name}
			params := map[string]int{}
			for i, param := range method.Params {
				if namespaces[param.Name] {
					return refuse(param.Span, "native-stream parameter shadows an SDK namespace")
				}
				if _, exists := params[param.Name]; exists {
					return refuse(param.Span, "duplicate native-stream parameter")
				}
				params[param.Name] = i
				typ := streamir.Type(param.Type.Name)
				kind := TypeRefNamed
				if typ == streamir.Int {
					kind = TypeRefPrimitive
				}
				if (typ != streamir.ReadStream && typ != streamir.WriteStream && typ != streamir.Int) || !nativeStreamType(param.Type, string(typ), kind) {
					return refuse(param.Span, "native-stream parameters must be ReadStream, WriteStream or int")
				}
				f.Parameters = append(f.Parameters, typ)
			}
			call, ok := method.Body.(*CallExpr)
			if !ok || len(call.Arguments) != 3 {
				return refuse(method.Span, "native-stream body must directly return one declared operation call")
			}
			binding, ok := aliases[call.Name]
			if !ok {
				return refuse(call.Span, "undeclared native-stream operation")
			}
			f.Binding = binding
			for i, arg := range call.Arguments {
				id, ok := arg.(*IdentExpr)
				if !ok {
					return refuse(arg.SourceSpan(), "native-stream arguments must be borrowed parameters")
				}
				position, ok := params[id.Name]
				if !ok {
					return refuse(id.Span, "unknown native-stream argument")
				}
				f.Arguments[i] = position
			}
			result.Functions = append(result.Functions, f)
		}
	}
	if err := streamir.Validate(result); err != nil {
		return refuse(program.Span, err.Error())
	}
	return result, nil
}

func nativeStreamType(t UnresolvedTypeRef, name string, kind TypeRefKind) bool {
	return t.Name == name && t.Kind == kind && t.Qualifier == "" && len(t.Arguments) == 0
}
