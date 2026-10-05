package streameval

import "dockpipe/src/lib/pipelang/streamir"

func (e *evaluator) invokeStep(binding streamir.Binding, args []Value) (out StepOutcome) {
	fail := func(status streamir.Status) StepOutcome { return StepOutcome{Outcome: Outcome{Status: status}} }
	if !args[0].Available || (!args[1].Available && args[1].Extent != 0) || (!args[2].Available && args[2].Extent != 0) {
		return fail(streamir.InvalidArgument)
	}
	if e.host == nil {
		return fail(streamir.Denied)
	}
	host, ok := e.host.(IncrementalHost)
	if !ok {
		return fail(streamir.Unsupported)
	}
	e.trace = append(e.trace, binding.Package+"/"+binding.Operation.ID)
	defer func() {
		if recover() != nil {
			out = fail(streamir.HostFailure)
		}
	}()
	out = host.InvokeStep(binding, args[0].Capability, args[1].Capability, args[2].Capability, args[3].Bool)
	out = normalize(Value{Type: streamir.Step, Step: out}).Step
	if out.InputConsumed > args[1].Extent || out.OutputWritten > args[2].Extent || out.InputConsumed > out.InputBytes || out.OutputWritten > out.OutputBytes {
		out = fail(streamir.HostFailure)
	}
	return
}
