// Package streameval is the reference evaluator for native-stream composition.
// Only an explicitly supplied Host can perform effects. It does not open streams.
package streameval

import (
	"fmt"

	"dockpipe/src/lib/pipelang/streamir"
)

type Outcome struct {
	Status                          streamir.Status
	InputBytes, OutputBytes, Chunks uint64
}
type Value struct {
	Type       streamir.Type
	Int        int64
	Uint       uint64
	Bool       bool
	Status     streamir.Status
	Result     Outcome
	Capability any
	Available  bool
}
type Host interface {
	Invoke(streamir.Binding, any, any, uint64) Outcome
}
type Execution struct {
	Value Value
	Trace []string
}
type evaluator struct {
	program streamir.Program
	host    Host
	trace   []string
}

func Evaluate(p streamir.Program, class, method string, args []Value, host Host) (Execution, error) {
	if err := streamir.Validate(p); err != nil {
		return Execution{}, err
	}
	if p.Profile != streamir.CompositionProfile {
		return Execution{}, fmt.Errorf("reference evaluator requires stream v2")
	}
	for i, f := range p.Functions {
		if f.Class == class && f.Name == method && f.Public {
			if len(args) != len(f.Parameters) {
				return Execution{}, fmt.Errorf("entry argument count")
			}
			for i, t := range f.Parameters {
				if args[i].Type != t {
					return Execution{}, fmt.Errorf("entry argument type")
				}
			}
			e := evaluator{program: p, host: host}
			v := e.function(i, args)
			return Execution{v, e.trace}, nil
		}
	}
	return Execution{}, fmt.Errorf("unknown public stream entry")
}
func normalize(v Value) Value {
	if v.Type == streamir.Result && v.Result.Status > streamir.Denied {
		v.Result = Outcome{Status: streamir.HostFailure}
	}
	if v.Type == streamir.StatusType && v.Status > streamir.Denied {
		v.Status = streamir.HostFailure
	}
	return v
}
func (e *evaluator) function(index int, args []Value) Value {
	env := map[int]Value{}
	for i, v := range args {
		env[i] = normalize(v)
	}
	v, _ := e.block(e.program.Functions[index].Body, env)
	return v
}
func (e *evaluator) block(body []streamir.Statement, outer map[int]Value) (Value, bool) {
	env := map[int]Value{}
	for k, v := range outer {
		env[k] = v
	}
	for _, s := range body {
		switch s.Kind {
		case "local":
			env[s.Slot] = e.expression(s.Value, env)
		case "return":
			return e.expression(s.Value, env), true
		case "if":
			branch := s.Else
			if e.expression(s.Value, env).Bool {
				branch = s.Then
			}
			if v, returned := e.block(branch, env); returned {
				return v, true
			}
		case "block":
			if v, returned := e.block(s.Then, env); returned {
				return v, true
			}
		}
	}
	return Value{}, false
}
func (e *evaluator) invoke(binding streamir.Binding, a, b Value, limit int64) (out Outcome) {
	if limit < 0 || !a.Available || !b.Available {
		return Outcome{Status: streamir.InvalidArgument}
	}
	if e.host == nil {
		return Outcome{Status: streamir.Denied}
	}
	e.trace = append(e.trace, binding.Package+"/"+binding.Operation.ID)
	defer func() {
		if recover() != nil {
			out = Outcome{Status: streamir.HostFailure}
		}
	}()
	out = e.host.Invoke(binding, a.Capability, b.Capability, uint64(limit))
	if out.Status > streamir.Denied {
		out = Outcome{Status: streamir.HostFailure}
	}
	return
}
func (e *evaluator) expression(x *streamir.Expression, env map[int]Value) Value {
	arg := func(i int) Value { return e.expression(x.Arguments[i], env) }
	v := Value{Type: x.Type}
	switch x.Kind {
	case "variable":
		return env[x.Slot]
	case "int":
		v.Int = x.Int
	case "uint64":
		v.Uint = x.Uint
	case "bool":
		v.Bool = x.Bool
	case "status":
		v.Status = x.Status
	case "not":
		v.Bool = !arg(0).Bool
	case "conditional":
		if arg(0).Bool {
			return arg(1)
		}
		return arg(2)
	case "field":
		r := arg(0).Result
		switch x.Member {
		case "ok":
			v.Bool = r.Status == streamir.OK
		case "status":
			v.Status = r.Status
		case "inputBytes":
			v.Uint = r.InputBytes
		case "outputBytes":
			v.Uint = r.OutputBytes
		case "chunks":
			v.Uint = r.Chunks
		}
	case "native", "helper":
		args := []Value{}
		for _, x := range x.Arguments {
			args = append(args, e.expression(x, env))
		}
		if x.Kind == "helper" {
			return e.function(x.Target, args)
		}
		v.Result = e.invoke(e.program.Bindings[x.Target], args[0], args[1], args[2].Int)
	case "binary":
		a := arg(0)
		if x.Member == "&&" && !a.Bool {
			v.Bool = false
			return v
		}
		if x.Member == "||" && a.Bool {
			v.Bool = true
			return v
		}
		b := arg(1)
		order := 0
		switch a.Type {
		case streamir.Bool:
			if !a.Bool && b.Bool {
				order = -1
			} else if a.Bool && !b.Bool {
				order = 1
			}
		case streamir.Int:
			if a.Int < b.Int {
				order = -1
			} else if a.Int > b.Int {
				order = 1
			}
		case streamir.Uint64:
			if a.Uint < b.Uint {
				order = -1
			} else if a.Uint > b.Uint {
				order = 1
			}
		case streamir.StatusType:
			if a.Status < b.Status {
				order = -1
			} else if a.Status > b.Status {
				order = 1
			}
		}
		switch x.Member {
		case "&&":
			v.Bool = a.Bool && b.Bool
		case "||":
			v.Bool = a.Bool || b.Bool
		case "==":
			v.Bool = order == 0
		case "!=":
			v.Bool = order != 0
		case "<":
			v.Bool = order < 0
		case "<=":
			v.Bool = order <= 0
		case ">":
			v.Bool = order > 0
		case ">=":
			v.Bool = order >= 0
		}
	}
	return v
}
