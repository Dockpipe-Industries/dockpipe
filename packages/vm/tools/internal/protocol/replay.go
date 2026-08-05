package protocol

import "fmt"

type ReplayGuard struct {
	identity Context
	next     uint64
	nonces   map[string]struct{}
}

func NewReplayGuard(identity Context) *ReplayGuard {
	identity.Sequence = 0
	identity.Nonce = ""
	identity.Phase = ""
	return &ReplayGuard{identity: identity, next: FirstSequence, nonces: map[string]struct{}{}}
}

func (g *ReplayGuard) Accept(frame SignedFrame) error {
	ctx := frame.Context
	if ctx.MachineUUID != g.identity.MachineUUID || ctx.DiskSerial != g.identity.DiskSerial || ctx.BootID != g.identity.BootID || ctx.RunID != g.identity.RunID || ctx.Scenario != g.identity.Scenario || ctx.DurabilityBoundary != g.identity.DurabilityBoundary {
		return fmt.Errorf("authenticated identity substitution rejected")
	}
	if ctx.Sequence != g.next {
		return fmt.Errorf("out-of-order sequence: got %d want %d", ctx.Sequence, g.next)
	}
	if _, exists := g.nonces[ctx.Nonce]; exists {
		return fmt.Errorf("replayed nonce")
	}
	g.nonces[ctx.Nonce] = struct{}{}
	g.next++
	return nil
}
