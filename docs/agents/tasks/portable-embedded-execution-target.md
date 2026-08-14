# TASK-017 Portable Embedded Execution Target And MCU Runtime

## Goal

Introduce a portable embedded execution target that compiles validated DockPipe instructions into
streamable bytecode or standalone MCU firmware. Keep this a generic DockPipe primitive rather than
putting firmware-specific behavior in a capability package.

This is a deferred, non-MVP backlog contract. It does not authorize implementation or expand
TASK-015.

## Product Shape

```text
DockPipe program
      |
portable embedded IR / bytecode
      |
+----------------+-----------------+
| Stream mode    | Standalone mode |
| USB / serial   | Embedded flash  |
+----------------+-----------------+
      |
tiny MCU runtime
```

The embedded target should own:

- bounded instruction-set and portable IR definitions
- deterministic bytecode compilation
- generated host and firmware bindings
- framed transport protocol for streamed execution
- target capability declarations and compatibility checks
- firmware embedding, simulation, flashing, and recovery contracts
- adapters for Arduino, RP2040, ESP32, STM32, and later targets

Generic capabilities may include GPIO, timers, ADC/DAC, I2C, and SPI. Domain behavior such as MIDI,
CV, displays, sensors, or lighting belongs in capability packages that consume this target.

## First Proof

Use an Arduino USB endpoint to run the same validated program in three modes:

1. simulated on Linux or Windows
2. streamed live over USB to the Arduino
3. embedded into firmware and run standalone after reboot

The proof must demonstrate equivalent bounded behavior and traceable results across all three
modes. Connection presence is not capability, authority, or completion evidence.

## Boundary And Safety Rules

- Preserve DockPipe's architecture: the program defines what happens; the embedded runtime defines
  where it executes; target adapters resolve board- and toolchain-specific behavior.
- Keep domain protocols and device behavior out of the generic runtime and compiler.
- Require explicit target capability declarations; reject unsupported instructions before
  streaming or flashing.
- Make instruction, memory, timing, event-rate, and transport limits explicit and fail closed.
- Treat flashing and persistent firmware mutation as separate approval-gated actions.
- Keep transport receipts, execution evidence, and lifecycle authorization distinct.
- Do not make a connected device authoritative or infer successful execution from transport
  acknowledgement alone.
- Add engine primitives only when they are demonstrably generic; board adapters, firmware assets,
  and toolchain integration should remain package- or resolver-owned where possible.

## Dependencies And Adjacency

- TASK-015 is adjacent, not the owner of this task. Its validated multi-machine artifacts and
  receipts may later carry embedded programs and execution evidence, but TASK-017 must remain a
  separate generic execution target.
- TASK-015 completion is not required for the local simulator, compiler, or single-host Arduino
  proof. Governed remote generation and execution should reuse TASK-015 after that contract is
  stable.
- TASK-018 is the first intended capability-package customer and must not own the embedded
  compiler, runtime, transport, simulator, or flashing framework.

## Proposed Slices

1. Define a fixture-only bounded IR, target capability declaration, compiler contract, and
   deterministic simulator.
2. Define the framed USB/serial transport and prove streamed execution on one Arduino target.
3. Generate firmware bindings and embed the same program for standalone execution after reboot.
4. Add explicit flashing, recovery, trace, replay, and emergency-stop contracts.
5. Generalize target adapters only after the Arduino proof identifies reusable boundaries.

## Success Criteria

- One canonical program produces deterministic simulator, streamed, and standalone behavior.
- Unsupported capabilities and malformed, oversized, stale, or replay-conflicting programs fail
  closed before device mutation.
- Host and firmware bindings are generated from one versioned instruction contract.
- Simulation and execution receipts expose bounded evidence without granting follow-on authority.
- Adding a second capability package does not require firmware-specific changes in that package.
- Adding a second MCU family does not require domain-specific changes in the embedded core.

## Explicitly Out Of Scope

- implementing the compiler, firmware, transport, flasher, simulator, or target adapters in this
  backlog-only task
- making MIDI part of the embedded core
- supporting every MCU or board in the first proof
- automatic flashing, device adoption, or firmware replacement
- hard real-time guarantees not proven for a declared target
- using connection state as authorization or completion evidence

## Next Bounded Slice

Write the fixture-only design for the smallest versioned IR, capability declaration, deterministic
simulator result, and fail-closed resource bounds. Do not add live USB, flashing, firmware, or MIDI
behavior in that slice.
