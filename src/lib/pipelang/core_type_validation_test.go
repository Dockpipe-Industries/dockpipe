package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func supportedCoreTypes() map[string]coreir.Type {
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	boolean := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}
	record := coreir.Type{Kind: coreir.TypeRecord, Name: "Row",
		Identity: &coreir.SemanticIdentity{PackageID: "test", Path: "row"},
		Record: &coreir.RecordType{Fields: []coreir.RecordField{{Name: "Text",
			Identity: coreir.SemanticIdentity{PackageID: "test", Path: "row.text"}, Type: text}}}}
	list := coreir.Type{Kind: coreir.TypeList, Name: "List",
		Identity: &coreir.SemanticIdentity{PackageID: coreir.BuiltinPackageID, Path: coreir.ListSemanticPath},
		List:     &coreir.ListType{Element: record}}
	types := map[string]coreir.Type{
		"text": text, "bool": boolean, "int64": coreir.SignedInteger(64), "binary64": coreir.BinaryFloat(64),
		"arithmetic-error": coreir.ArithmeticErrorType(), "record": record, "list": list,
		"int-result":      coreir.ArithmeticResult(coreir.SignedInteger(64)),
		"float-result":    coreir.ArithmeticResult(coreir.BinaryFloat(64)),
		"text-result":     {Kind: coreir.TypeResult, Result: &coreir.ResultType{Success: text, Failure: text}},
		"snapshot-result": {Kind: coreir.TypeResult, Result: &coreir.ResultType{Success: list, Failure: text}},
	}
	for name, value := range map[string]coreir.Type{"text": text, "bool": boolean, "int64": coreir.SignedInteger(64), "binary64": coreir.BinaryFloat(64), "record": record} {
		types["optional-"+name] = coreir.Type{Kind: coreir.TypeOptional, Optional: &coreir.OptionalType{Value: value}}
	}
	return types
}

func typeIdentity(typ coreir.Type) coreir.Function {
	position := 0
	return coreir.Function{
		Identity: coreir.SemanticIdentity{PackageID: "test", Path: "echo"}, Name: "Echo",
		Parameters: []coreir.Parameter{{Position: 0, Name: "value", Type: typ}}, ReturnType: typ,
		Body: coreir.Expr{Kind: coreir.ExprReference, Type: typ, Parameter: &position},
	}
}

func typeProgram(function coreir.Function) coreir.Program {
	return coreir.Program{CompilerContract: coreir.CompilerContractV1, LanguageContract: coreir.LanguageContractV800,
		Functions: []coreir.Function{function}}
}

func rejectCoreType(t *testing.T, typ coreir.Type) {
	t.Helper()
	for _, location := range []string{"identity", "unused-parameter", "return"} {
		t.Run(location, func(t *testing.T) {
			function := typeIdentity(typ)
			if location != "identity" {
				function = typeIdentity(coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool})
				if location == "unused-parameter" {
					function.Parameters = append(function.Parameters, coreir.Parameter{Position: 1, Name: "unused", Type: typ})
				} else {
					function.ReturnType = typ
				}
			}
			if err := coreir.ValidateFunction(function); err == nil {
				t.Fatal("function admitted malformed type")
			}
			if _, err := coreeval.Evaluate(function, []coreeval.Value{{Type: typ, Int: 999}}); err == nil {
				t.Fatal("function evaluator admitted malformed type")
			}
			assertAdmissionRejected(t, typeProgram(function), "type")
		})
	}
}

func TestCoreTypeValidationKindsAndNumbers(t *testing.T) {
	invalid := map[string]coreir.Type{
		"empty-kind": {}, "invented-kind": {Kind: "invented"},
		"named": {Kind: coreir.TypeNamed}, "applied": {Kind: coreir.TypeApplied},
		"empty-primitive":    {Kind: coreir.TypePrimitive},
		"invented-primitive": {Kind: coreir.TypePrimitive, Primitive: "invented"},
		"source-int":         {Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveInt},
		"source-float":       {Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveFloat},
		"missing-numeric":    {Kind: coreir.TypeNumeric}, "missing-result": {Kind: coreir.TypeResult},
		"missing-optional": {Kind: coreir.TypeOptional}, "missing-list": {Kind: coreir.TypeList},
		"missing-record": {Kind: coreir.TypeRecord},
	}
	for _, representation := range []coreir.NumericRepresentation{"", "invented", coreir.NumericInteger, coreir.NumericBinaryFloat} {
		for _, bits := range []int{-1, 0, 3, 8, 16, 32, 64, 128} {
			for _, signed := range []bool{false, true} {
				if bits == 64 && ((representation == coreir.NumericInteger && signed) || (representation == coreir.NumericBinaryFloat && !signed)) {
					continue
				}
				invalid[fmt.Sprintf("numeric-%s-%d-%t", representation, bits, signed)] = coreir.Type{Kind: coreir.TypeNumeric,
					Numeric: &coreir.NumericType{Representation: representation, Bits: bits, Signed: signed}}
			}
		}
	}
	for name, typ := range invalid {
		t.Run(name, func(t *testing.T) { rejectCoreType(t, typ) })
	}
}

func TestCoreTypeValidationContradictoryRepresentations(t *testing.T) {
	types := supportedCoreTypes()
	for name, typ := range types {
		// Every Type field belongs to exactly one kind, except identity/name on
		// records and lists. Arguments belong to SemanticType, never executable Type.
		mutations := map[string]func(*coreir.Type){
			"primitive": func(v *coreir.Type) { v.Primitive = coreir.PrimitiveBool },
			"numeric":   func(v *coreir.Type) { v.Numeric = coreir.SignedInteger(64).Numeric },
			"result":    func(v *coreir.Type) { v.Result = types["text-result"].Result },
			"optional":  func(v *coreir.Type) { v.Optional = types["optional-text"].Optional },
			"list":      func(v *coreir.Type) { v.List = types["list"].List },
			"record":    func(v *coreir.Type) { v.Record = types["record"].Record },
			"identity":  func(v *coreir.Type) { v.Identity = types["record"].Identity },
			"name":      func(v *coreir.Type) { v.Name = "Extraneous" },
			"arguments": func(v *coreir.Type) { v.Arguments = []coreir.Type{types["bool"]} },
		}
		for field, mutate := range mutations {
			if field == string(typ.Kind) || ((field == "identity" || field == "name") && (typ.Kind == coreir.TypeRecord || typ.Kind == coreir.TypeList)) {
				continue
			}
			t.Run(name+"/"+field, func(t *testing.T) {
				bad := typ
				mutate(&bad)
				rejectCoreType(t, bad)
			})
		}
	}
}

func TestCoreTypeValidationNestedRepresentations(t *testing.T) {
	for _, bad := range []coreir.Type{{Kind: "invented"}, coreir.SignedInteger(3),
		{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString, Record: &coreir.RecordType{}},
		{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool, List: &coreir.ListType{}}} {
		types := supportedCoreTypes()
		for name, wrap := range map[string]func(coreir.Type) coreir.Type{
			"record-field": func(v coreir.Type) coreir.Type { r := types["record"]; r.Record.Fields[0].Type = v; return r },
			"optional-value": func(v coreir.Type) coreir.Type {
				return coreir.Type{Kind: coreir.TypeOptional, Optional: &coreir.OptionalType{Value: v}}
			},
			"list-element":   func(v coreir.Type) coreir.Type { l := types["list"]; l.List.Element = v; return l },
			"result-success": func(v coreir.Type) coreir.Type { return coreir.ArithmeticResult(v) },
			"result-failure": func(v coreir.Type) coreir.Type {
				return coreir.Type{Kind: coreir.TypeResult, Result: &coreir.ResultType{Success: types["text"], Failure: v}}
			},
		} {
			t.Run(fmt.Sprintf("%s/%s", name, bad.Kind), func(t *testing.T) { rejectCoreType(t, wrap(bad)) })
		}
	}
	// Supported outer envelopes must not conceal malformed record field types.
	for _, name := range []string{"record", "optional-record", "list", "snapshot-result"} {
		t.Run(name, func(t *testing.T) {
			types := supportedCoreTypes()
			types["record"].Record.Fields[0].Type.Numeric = coreir.SignedInteger(64).Numeric
			rejectCoreType(t, types[name])
		})
	}
}

func TestCoreTypeValidationExpressionTypes(t *testing.T) {
	// Equal literal types used to pass comparison checks even when both carried
	// an unrelated representation. They are not signature types.
	for _, location := range []string{"comparison", "unselected-branch", "uncalled-function"} {
		t.Run(location, func(t *testing.T) {
			boolean := supportedCoreTypes()["bool"]
			bad := boolean
			bad.Numeric = coreir.SignedInteger(64).Numeric
			literal := func(typ coreir.Type) *coreir.Expr {
				return &coreir.Expr{Kind: coreir.ExprLiteral, Type: typ, Literal: &coreir.Literal{Bool: true}}
			}
			function := typeIdentity(boolean)
			function.Body = coreir.Expr{Kind: coreir.ExprBinary, Type: boolean, Binary: &coreir.Binary{
				Operator: coreir.OperatorEqual, Left: literal(bad), Right: literal(bad)}}
			if location == "unselected-branch" {
				comparison := function.Body
				function.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: boolean, Conditional: &coreir.Conditional{
					Condition: literal(boolean), WhenTrue: literal(boolean), WhenFalse: &comparison}}
			}
			program := typeProgram(function)
			if location == "uncalled-function" {
				valid := typeIdentity(boolean)
				valid.Identity.Path, valid.Name = "valid", "Valid"
				program.Functions = append([]coreir.Function{valid}, function)
			}
			assertAdmissionRejected(t, program, "expression type")
		})
	}
}

func TestCoreTypeValidationAcceptedTypes(t *testing.T) {
	for name, typ := range supportedCoreTypes() {
		t.Run(name, func(t *testing.T) {
			function := typeIdentity(typ)
			if err := coreir.ValidateFunction(function); err != nil {
				t.Fatal(err)
			}
			if err := coreir.ValidateProgram(typeProgram(function)); err != nil {
				t.Fatal(err)
			}
			// ArithmeticError is an internal Result component, not a standalone Go value.
			if typ.Kind == coreir.TypeArithmeticError {
				return
			}
			value := admissionValue(typ)
			if _, err := coreeval.EvaluateProgram(typeProgram(function), function.Identity, []coreeval.Value{value}); err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(typeProgram(function)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCoreTypeValidationTinyFixture(t *testing.T) {
	program := admissionFixture(t, "tiny-pure-function")
	function := program.Functions[0]
	if !coreir.TypeEqual(function.Parameters[0].Type, coreir.SignedInteger(64)) ||
		function.Identity.Callable.Parameters[0].Primitive != coreir.PrimitiveInt {
		t.Fatal("fixture no longer distinguishes executable and semantic numeric types")
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "tiny-pure-function.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, golden) {
		t.Fatal("tiny fixture Go golden changed")
	}
}

func TestCoreTypeValidationUnsupportedNesting(t *testing.T) {
	for name, typ := range supportedCoreTypes() {
		if typ.Kind != coreir.TypeOptional && typ.Kind != coreir.TypeList && typ.Kind != coreir.TypeResult {
			continue
		}
		for outer, wrap := range map[string]func(coreir.Type) coreir.Type{
			"optional": func(v coreir.Type) coreir.Type {
				return coreir.Type{Kind: coreir.TypeOptional, Optional: &coreir.OptionalType{Value: v}}
			},
			"list":              func(v coreir.Type) coreir.Type { l := supportedCoreTypes()["list"]; l.List.Element = v; return l },
			"arithmetic-result": coreir.ArithmeticResult,
			"record": func(v coreir.Type) coreir.Type {
				r := supportedCoreTypes()["record"]
				r.Record.Fields[0].Type = v
				return r
			},
		} {
			t.Run(outer+"/"+name, func(t *testing.T) { rejectCoreType(t, wrap(typ)) })
		}
	}
}

func TestCoreTypeValidationLocalAndCarrierTypes(t *testing.T) {
	const source = `public Record Row { public string Id; }
		public Class Root { public Optional<Row> Read(Optional<Row> carrier) {
			Row value = propagate(carrier); return some(value);
		} }`
	for _, location := range []string{"local", "carrier"} {
		t.Run(location, func(t *testing.T) {
			program := reviewCore(t, PipeLangLanguageContractV800, source, "Read")
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
			local := program.Functions[0].Body.ImmutableLocal
			// TypeEqual intentionally compares semantic identity, not callable
			// metadata; type validation must still reject this contradiction.
			if location == "local" {
				identity := *local.Type.Identity
				identity.Callable = &coreir.CallableIdentity{}
				local.Type.Identity = &identity
			} else {
				carrier := local.Initializer.Propagate.Carrier
				payload := *carrier.Optional
				identity := *payload.Value.Identity
				identity.Callable = &coreir.CallableIdentity{}
				payload.Value.Identity = &identity
				carrier.Optional = &payload
				local.Initializer.Propagate.Carrier = carrier
			}
			assertAdmissionRejected(t, program, "type")
		})
	}
}
