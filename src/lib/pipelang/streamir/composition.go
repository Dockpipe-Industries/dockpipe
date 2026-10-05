package streamir

import "fmt"

type Status uint32

const (
	OK Status = iota
	InvalidArgument
	InvalidStream
	Unsupported
	LimitExceeded
	IOError
	HostFailure
	Denied
)

func ParseStatus(name string) (Status, bool) {
	for i, n := range []string{"ok", "invalidArgument", "invalidStream", "unsupported", "limitExceeded", "ioError", "hostFailure", "denied"} {
		if name == n {
			return Status(i), true
		}
	}
	return 0, false
}

// Expression is typed effect IR, distinct from pure Core. Variable slots are
// lexical bindings; native/helper argument arrays preserve source evaluation order.
type Expression struct {
	Kind      string
	Type      Type
	Slot      int
	Target    int
	Arguments []*Expression
	Member    string
	Int       int64
	Uint      uint64
	Bool      bool
	Status    Status
}
type Statement struct {
	Kind       string
	Slot       int
	Type       Type
	Value      *Expression
	Then, Else []Statement
}

func ValueType(t Type) bool {
	return t == Int || t == Bool || t == Uint64 || t == Result || t == StatusType || t == Step
}
func ParameterType(t Type) bool {
	return ValueType(t) || t == ReadStream || t == WriteStream || t == Session || t == InputBuffer || t == OutputBuffer
}
func MemberType(name string) Type {
	switch name {
	case "ok":
		return Bool
	case "status":
		return StatusType
	case "inputBytes", "outputBytes", "chunks":
		return Uint64
	}
	return ""
}
func copyTypes(env map[int]Type) map[int]Type {
	r := map[int]Type{}
	for k, v := range env {
		r[k] = v
	}
	return r
}

type compositionValidator struct {
	p        Program
	function int
	nodes    int
	slots    map[int]bool
	calls    [][]int
}

func validateComposition(p Program) error {
	methods := map[string]bool{}
	public := false
	for _, f := range p.Functions {
		key := f.Class + "." + f.Name
		if !identifier.MatchString(f.Class) || !identifier.MatchString(f.Name) || methods[key] || len(f.Parameters) > 16 || !ValueTypeFor(p.Profile, f.ReturnType) || len(f.Body) == 0 || f.Binding != 0 || f.Arguments != [3]int{} {
			return fmt.Errorf("invalid composition function")
		}
		for _, t := range f.Parameters {
			if !ParameterTypeFor(p.Profile, t) {
				return fmt.Errorf("invalid composition parameter")
			}
		}
		methods[key] = true
		public = public || f.Public
	}
	if !public {
		return fmt.Errorf("composition program has no public entry")
	}
	v := compositionValidator{p: p, calls: make([][]int, len(p.Functions))}
	costs := make([]int, len(p.Functions))
	for i, f := range p.Functions {
		v.function = i
		v.slots = map[int]bool{}
		env := map[int]Type{}
		for i, t := range f.Parameters {
			env[i] = t
			v.slots[i] = true
		}
		before := v.nodes
		returned, err := v.block(f.Body, env, 0)
		if err != nil {
			return err
		}
		if !returned {
			return fmt.Errorf("composition function may fall through")
		}
		costs[i] = v.nodes - before
	}
	// Bound acyclic expansion as well as source size; a small helper DAG must
	// not amplify a single entry into exponentially many native effects.
	var cost func(int, int, map[int]bool) (int, error)
	cost = func(i, depth int, active map[int]bool) (int, error) {
		if depth > 16 || active[i] {
			return 0, fmt.Errorf("recursive or too-deep composition call graph")
		}
		active[i] = true
		defer delete(active, i)
		n := costs[i]
		for _, target := range v.calls[i] {
			c, e := cost(target, depth+1, active)
			if e != nil {
				return 0, e
			}
			n += c
			if n > 8192 {
				return 0, fmt.Errorf("composition expansion exceeds 8192 nodes")
			}
		}
		return n, nil
	}
	for i := range p.Functions {
		if _, e := cost(i, 0, map[int]bool{}); e != nil {
			return e
		}
	}
	return nil
}
func (v *compositionValidator) count(depth int) error {
	v.nodes++
	if v.nodes > 8192 || depth > 64 {
		return fmt.Errorf("composition extent exceeded")
	}
	return nil
}
func (v *compositionValidator) block(statements []Statement, outer map[int]Type, depth int) (bool, error) {
	if len(statements) > 256 {
		return false, fmt.Errorf("composition block too large")
	}
	env := copyTypes(outer)
	returned := false
	for _, s := range statements {
		if err := v.count(depth); err != nil {
			return false, err
		}
		if returned {
			return false, fmt.Errorf("unreachable composition statement")
		}
		switch s.Kind {
		case "local":
			if !ValueTypeFor(v.p.Profile, s.Type) || s.Slot < 0 || s.Slot >= 256 || v.slots[s.Slot] || len(s.Then) != 0 || len(s.Else) != 0 {
				return false, fmt.Errorf("invalid composition local")
			}
			t, e := v.expression(s.Value, env, depth+1)
			if e != nil {
				return false, e
			}
			if t != s.Type {
				return false, fmt.Errorf("composition local type mismatch")
			}
			env[s.Slot] = t
			v.slots[s.Slot] = true
		case "return":
			if s.Type != "" || s.Slot != 0 || len(s.Then) != 0 || len(s.Else) != 0 {
				return false, fmt.Errorf("invalid return metadata")
			}
			t, e := v.expression(s.Value, env, depth+1)
			if e != nil {
				return false, e
			}
			if t != v.p.Functions[v.function].ReturnType {
				return false, fmt.Errorf("composition return type mismatch")
			}
			returned = true
		case "if":
			if s.Type != "" || s.Slot != 0 || len(s.Then) == 0 {
				return false, fmt.Errorf("invalid composition branch")
			}
			t, e := v.expression(s.Value, env, depth+1)
			if e != nil {
				return false, e
			}
			if t != Bool {
				return false, fmt.Errorf("non-Boolean composition condition")
			}
			a, e := v.block(s.Then, env, depth+1)
			if e != nil {
				return false, e
			}
			b, e := v.block(s.Else, env, depth+1)
			if e != nil {
				return false, e
			}
			returned = a && b
		case "block":
			if s.Value != nil || s.Type != "" || s.Slot != 0 || len(s.Else) != 0 {
				return false, fmt.Errorf("invalid nested block metadata")
			}
			var e error
			returned, e = v.block(s.Then, env, depth+1)
			if e != nil {
				return false, e
			}
		default:
			return false, fmt.Errorf("unknown composition statement")
		}
	}
	return returned, nil
}
func (v *compositionValidator) expression(x *Expression, env map[int]Type, depth int) (Type, error) {
	if e := v.count(depth); e != nil {
		return "", e
	}
	if x == nil || len(x.Arguments) > 16 {
		return "", fmt.Errorf("missing/oversized composition expression")
	}
	fail := func() (Type, error) { return "", fmt.Errorf("invalid %s composition expression", x.Kind) }
	var inferred Type
	switch x.Kind {
	case "variable":
		if len(x.Arguments) != 0 {
			return fail()
		}
		inferred = env[x.Slot]
	case "bool":
		if len(x.Arguments) != 0 {
			return fail()
		}
		inferred = Bool
	case "int":
		if len(x.Arguments) != 0 {
			return fail()
		}
		inferred = Int
	case "uint64":
		if len(x.Arguments) != 0 {
			return fail()
		}
		inferred = Uint64
	case "status":
		if len(x.Arguments) != 0 || x.Status > Denied {
			return fail()
		}
		inferred = StatusType
	case "native", "helper":
		var params []Type
		if x.Kind == "native" {
			if x.Target < 0 || x.Target >= len(v.p.Bindings) {
				return fail()
			}
			params, inferred = NativeSignature(v.p.Profile)
		} else {
			if x.Target < 0 || x.Target >= len(v.p.Functions) {
				return fail()
			}
			target := v.p.Functions[x.Target]
			if !target.Public && target.Class != v.p.Functions[v.function].Class {
				return fail()
			}
			params = target.Parameters
			inferred = target.ReturnType
			v.calls[v.function] = append(v.calls[v.function], x.Target)
		}
		if len(params) != len(x.Arguments) {
			return fail()
		}
		for i, arg := range x.Arguments {
			t, e := v.expression(arg, env, depth+1)
			if e != nil {
				return "", e
			}
			if t != params[i] {
				return fail()
			}
		}
	case "field":
		if len(x.Arguments) != 1 {
			return fail()
		}
		t, e := v.expression(x.Arguments[0], env, depth+1)
		if e != nil {
			return "", e
		}
		if t != Result && t != Step {
			return fail()
		}
		inferred = ResultMemberType(t, x.Member)
	case "not":
		if len(x.Arguments) != 1 {
			return fail()
		}
		t, e := v.expression(x.Arguments[0], env, depth+1)
		if e != nil {
			return "", e
		}
		if t != Bool {
			return fail()
		}
		inferred = Bool
	case "conditional":
		if len(x.Arguments) != 3 {
			return fail()
		}
		types := []Type{}
		for _, arg := range x.Arguments {
			t, e := v.expression(arg, env, depth+1)
			if e != nil {
				return "", e
			}
			types = append(types, t)
		}
		if types[0] != Bool || types[1] != types[2] || !ValueType(types[1]) {
			return fail()
		}
		inferred = types[1]
	case "binary":
		if len(x.Arguments) != 2 {
			return fail()
		}
		a, e := v.expression(x.Arguments[0], env, depth+1)
		if e != nil {
			return "", e
		}
		b, e := v.expression(x.Arguments[1], env, depth+1)
		if e != nil {
			return "", e
		}
		if a != b {
			return fail()
		}
		switch x.Member {
		case "&&", "||":
			if a != Bool {
				return fail()
			}
		case "==", "!=":
			if a != Bool && a != Int && a != Uint64 && a != StatusType {
				return fail()
			}
		case "<", ">", "<=", ">=":
			if a != Int && a != Uint64 {
				return fail()
			}
		default:
			return fail()
		}
		inferred = Bool
	default:
		return fail()
	}
	if inferred == "" || x.Type != inferred {
		return fail()
	}
	return inferred, nil
}
