package coreeval

// argumentFrame belongs to one invocation. Parameters remain borrowed until a
// lexical binding needs storage; then detach even if the caller has spare
// capacity. Recursive expressions share the owned stack, while calls and named
// predicates create fresh frames. No frame is retained by PreparedProgram.
type argumentFrame struct {
	values []Value
	owned  bool
}

func (frame *argumentFrame) push(value Value) {
	if !frame.owned {
		values := make([]Value, len(frame.values), len(frame.values)+1)
		copy(values, frame.values)
		frame.values = values
		frame.owned = true
	}
	frame.values = append(frame.values, value)
}

func (frame *argumentFrame) pop() {
	last := len(frame.values) - 1
	frame.values[last] = Value{}
	frame.values = frame.values[:last]
}
