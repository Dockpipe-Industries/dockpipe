package invocation

import (
	"context"
	"errors"
	"testing"
)

// This fixed, memory-only fixture isolates admission overhead from command
// execution. Whole-process cold/warm latency, service starts and persistent I/O
// still require measurements at each integrated native entrypoint.
func benchmarkPrepare(_ context.Context, value int) (Prepared[int], error) {
	if value < 0 {
		return Prepared[int]{}, errors.New("negative fixture input")
	}
	return Prepared[int]{Execute: func(context.Context) (int, error) { return value + 1, nil }}, nil
}

func BenchmarkDirectTypedOperation(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		prepared, err := benchmarkPrepare(ctx, 41)
		if err != nil {
			b.Fatal(err)
		}
		result, err := prepared.Execute(ctx)
		if err != nil || result != 42 {
			b.Fatalf("unexpected fixture result: %d, %v", result, err)
		}
	}
}

func BenchmarkAdmittedTypedOperation(b *testing.B) {
	definition := fixtureDefinition()
	contract, err := New(definition, benchmarkPrepare, nil)
	if err != nil {
		b.Fatal(err)
	}
	call := fixtureCall(definition)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		result, err := contract.Invoke(ctx, call, 41)
		if err != nil || result != 42 {
			b.Fatalf("unexpected fixture result: %d, %v", result, err)
		}
	}
}
