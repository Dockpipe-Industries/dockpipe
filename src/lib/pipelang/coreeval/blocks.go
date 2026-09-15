package coreeval

import (
	"dockpipe/src/lib/pipelang/coreir"
	"fmt"
)

func evalBlock(b *coreir.Block, args *argumentFrame, functions map[string]coreir.Function) (Outcome, bool, error) {
	if b == nil {
		return Outcome{}, false, fmt.Errorf("missing block")
	}
	base := len(args.values)
	defer func() {
		for len(args.values) > base {
			args.pop()
		}
	}()
	for _, s := range b.Statements {
		switch s.Kind {
		case "local":
			o, err := evalExprWithProgram(*s.Value, args, functions)
			if err != nil {
				return Outcome{}, false, err
			}
			var v Value
			if s.Local.Type.Kind == coreir.TypeResult {
				c := cloneOutcome(o)
				v = Value{Type: s.Local.Type, Result: &c}
			} else {
				if !o.OK {
					return o, true, nil
				}
				v = cloneValue(o.Value)
			}
			if err := validateValue(v); err != nil {
				return Outcome{}, false, err
			}
			args.push(v)
		case "return":
			o, err := evalExprWithProgram(*s.Value, args, functions)
			return cloneOutcome(o), true, err
		case "if":
			o, err := evalExprWithProgram(*s.Value, args, functions)
			if err != nil || !o.OK {
				return o, true, err
			}
			branch := s.Else
			if o.Value.Bool {
				branch = s.Then
			}
			if branch != nil {
				o, returned, err := evalBlock(branch, args, functions)
				if returned || err != nil {
					return o, returned, err
				}
			}
		case "block":
			o, returned, err := evalBlock(s.Then, args, functions)
			if returned || err != nil {
				return o, returned, err
			}
		default:
			return Outcome{}, false, fmt.Errorf("invalid statement")
		}
	}
	return Outcome{}, false, nil
}
