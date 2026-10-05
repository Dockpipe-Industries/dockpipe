package main

import (
	"math"
	"strconv"
	"testing"
)

func TestTranscriptIntegerWidthsRejectOverflow(t *testing.T) {
	if got := transcriptUint16(math.MaxUint16); got != math.MaxUint16 {
		t.Fatalf("largest uint16 changed: %d", got)
	}
	if got := transcriptUint32(0); got != 0 {
		t.Fatalf("zero changed: %d", got)
	}
	cases := []struct {
		name string
		run  func()
	}{
		{name: "negative", run: func() { transcriptUint32(-1) }},
		{name: "uint16 overflow", run: func() { transcriptUint16(math.MaxUint16 + 1) }},
	}
	if strconv.IntSize == 64 {
		overflow := uint64(math.MaxUint32) + 1
		cases = append(cases, struct {
			name string
			run  func()
		}{name: "uint32 overflow", run: func() { transcriptUint32(int(overflow)) }})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("overflow was accepted")
				}
			}()
			test.run()
		})
	}
}
