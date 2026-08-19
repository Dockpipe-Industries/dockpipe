# TASK-018 Cross-Platform MIDI Package And Dual-Host Workflow

## Goal

Build a useful Linux- and Windows-first MIDI package that proves DockPipe's deterministic buffered
control model with existing hardware, then use it as the first capability-package customer of the
portable embedded execution target.

This is a deferred, non-MVP backlog contract. It does not authorize implementation or expand
TASK-015.

## Proving Chain

```text
DockPipe on Linux or Windows
        |
MIDI device / virtual MIDI / network MIDI
        |
SL MkIII -> Crave -> Eurorack
```

The first useful host package should:

- discover physical, virtual, and supported network MIDI endpoints
- send notes, control changes, clock, and transport
- schedule events deterministically and execute buffered sequences
- validate note ranges, density, rate changes, timing bounds, and endpoint capabilities
- trace and replay bounded sequences
- provide emergency note-off and deterministic teardown behavior
- generate on one host and execute on another through an explicit governed handoff
- optionally accept LLM-generated musical plans only as untrusted plans that pass the same
  validation and scheduling boundary

## Package And Embedded Boundary

MIDI owns musical plans, MIDI endpoint discovery, protocol validation, scheduling policy, traces,
and safe device behavior. It must not own MCU instruction sets, bytecode compilation, USB/serial
framing, firmware generation, simulation, flashing, or board adapters.

TASK-017 owns those generic embedded capabilities. With that target available, MIDI becomes one
capability package alongside future CV, display, sensor, and lighting packages.

## Delivery Phases

1. Ship a package-local Linux/Windows host path for discovery, validation, deterministic buffering,
   playback, trace/replay, and emergency note-off. This phase can proceed independently.
2. Add governed dual-host generation and execution using TASK-015 artifacts, capability snapshots,
   leases, receipts, and explicit decision boundaries after those contracts are stable.
3. Use TASK-017 to run the same bounded MIDI program through an Arduino USB endpoint in simulated,
   streamed, and standalone-after-reboot modes.

## Safety And Authority Rules

- Treat every endpoint, remote claim, and optional generated plan as untrusted input.
- Validate ranges, channel/device capabilities, density, rate, clock changes, buffer size, and total
  duration before execution.
- Make clock ownership explicit; never let multiple implicit clock sources drive a sequence.
- Bound queued events and execution time, and preserve a deterministic emergency note-off path.
- Keep generation, validation, execution, evidence, and follow-on authorization as separate
  decisions.
- Do not infer execution success from connection presence, send acknowledgement, or provider status.
- Never let an LLM bypass the canonical musical-plan validator or directly control a live endpoint.

## Dependencies And Adjacency

- The first Linux/Windows host package has no dependency on TASK-015 or TASK-017.
- Governed cross-host generation and execution should depend on TASK-015 rather than inventing a
  MIDI-specific remote authority or receipt model.
- Arduino streaming and standalone firmware should depend on TASK-017 rather than adding
  firmware-specific code to the MIDI package.
- Neither dependency makes MIDI part of TASK-015 or the embedded core.

## Success Criteria

- Linux and Windows enumerate and address declared MIDI endpoints through one package contract.
- A canonical buffered sequence produces deterministic ordering and bounded timing behavior.
- Invalid ranges, excessive density/rate, unsupported features, stale plans, and replay conflicts
  fail closed before live output.
- Trace/replay and emergency note-off work without granting apply, publish, or follow-on authority.
- A plan generated on one host can be validated and executed on another with exact identity and
  capability bindings.
- The Arduino proof runs the same program in simulator, live streamed, and standalone modes without
  moving firmware concerns into the MIDI package.

## Explicitly Out Of Scope

- implementing the MIDI package, embedded target, firmware, or hardware adapters in this
  backlog-only task
- audio synthesis, recording, mixing, or a DAW replacement
- unconstrained generative performance or direct LLM control of live equipment
- silently choosing devices, channels, clock sources, or network peers
- treating transport connectivity as execution authority or proof of completion

## Next Bounded Slice

Design the package-local fixture contract for Linux/Windows endpoint discovery, a canonical bounded
sequence, deterministic scheduling, validation failures, trace/replay, and emergency note-off. Do
not add remote execution, live hardware requirements, embedded bytecode, or firmware in that slice.
