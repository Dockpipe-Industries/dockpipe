package coreeval

import (
	"math"
	"testing"

	"dockpipe/src/lib/pipelang/coreir"
)

func TestSnapshotPreservesValuesAndRejectsUnsupportedGraphs(t *testing.T) {
	for _, bits := range []uint64{0, 1 << 63, 0x7ff8000000000042, 0x7ff0000000000000} {
		original := Value{Float: math.Float64frombits(bits), String: string([]byte{0xff, 0, 0x80}), List: []Value{}}
		copied, err := snapshot(original)
		if err != nil || math.Float64bits(copied.Float) != bits || copied.String != original.String || copied.List == nil {
			t.Fatalf("snapshot changed exact representation: %#v %v", copied, err)
		}
	}
	typ := coreir.Type{Kind: coreir.TypeOptional, Optional: &coreir.OptionalType{}}
	typ.Optional.Value = typ
	if _, err := snapshot(typ); err == nil {
		t.Fatal("cyclic graph accepted")
	}
	arguments := make([]coreir.Type, 1)
	arguments[0].Arguments = arguments
	if _, err := snapshot(arguments); err == nil {
		t.Fatal("cyclic slice accepted")
	}
	if _, err := snapshot(struct{ Future chan int }{make(chan int)}); err == nil {
		t.Fatal("unsupported future field accepted")
	}
	if _, err := snapshot(struct{ hidden int }{1}); err == nil {
		t.Fatal("hidden future field accepted")
	}
}
