# Coding style and reliability

This is Dockpipe's coding and review standard for new or materially changed code.
It guides focused fixes; it does not authorize unrelated refactoring or imply that
all existing code already meets it. Keep provider and package behavior outside the
generic engine, and preserve established public contracts.

## Readable responsibilities

Write for the next maintainer: descriptive names, normal formatting, separate
statements and whitespace between logical steps. Prefer cohesive functions and one
substantial type per file where idiomatic. Keep orchestration, domain decisions,
I/O, serialization and resource ownership clear. Use focused helpers when they
remove duplicated policy or enforce an invariant; avoid speculative abstractions.

Prefer straightforward control flow and iteration. Where braces are optional,
a single-statement branch may omit them, but keep the statement on its own line;
use braces consistently across a chain if any branch needs them. Avoid dense
one-liners, nested ternaries and implicit side effects.

## Reliability rules

1. **Define the operation's resource contract.** Bound work from untrusted or
   expandable inputs: bytes, members, steps, depth, concurrency and retained
   output as appropriate. Bound aggregate work as well as individual items.
   Derive limits from protocol, product and capacity requirements, document units
   and rationale, and check before allocation or expansion. Do not invent arbitrary
   caps merely to satisfy a review. A finite validated collection may already
   establish the needed bound.
2. **Make retries and polling finite.** Use an attempt budget and an overall
   deadline, cancellable backoff and explicit exhaustion. Retry only failures
   whose operation contract permits it. Reconcile uncertain external effects
   before retrying; a lost response is not proof that a write failed.
3. **Prefer iteration; justify recursion.** Recursive code needs an enforced depth
   budget with a documented stack/capacity basis, cycle handling where applicable,
   and a total work/input budget. Iteration alone does not bound a growing queue.
   Preserve valid language semantics when moving traversal to an explicit stack.
4. **Propagate cancellation across external operations.** Network calls, commands,
   authentication and deployment need operation-appropriate deadlines and bounded
   response/output handling. Include child processes, inherited pipes and cleanup
   in cancellation design. A socket timeout alone does not bound a whole operation.
5. **Give services an explicit lifecycle.** A service loop may intentionally run
   indefinitely. Name its owner, cancellation signal, per-iteration resource
   limits and bounded shutdown/drain behavior. Interactive applications and
   long-running workflows need controlled shutdown, not an arbitrary lifetime cap.
6. **Fail clearly on exhaustion or invalid input.** Report which contract failed
   without exposing credentials. Never return a silently truncated result as
   success. Deliberately bounded diagnostics may truncate with a visible marker;
   structured result bodies must remain complete or fail.
7. **Handle outcomes deliberately.** Preserve success, cancellation and failure
   through the established language idiom (`error`/context in Go, explicit result
   types where useful). Handle expected errors at their owning boundary. Do not
   swallow errors, turn cancellation into success or add wrappers without value.
8. **Own and release resources.** Pair acquisition with cleanup on every exit;
   bound cleanup that can block. Validate a replacement before publishing it,
   use atomic state transitions where supported, and retain recoverable previous
   state until success. Report meaningful cleanup failures without losing the
   original cause.
9. **Keep one authoritative implementation of each invariant.** Reuse common
   boundary validation. Independent checks at trust transitions may be necessary;
   copying the same business logic twice is not reliability redundancy.
10. **Prove failure behavior.** Test relevant limit edges, cancellation, invalid
    input, partial failure, restart/retry and cleanup. Use bounded fixtures rather
    than exhausting the machine. Distinguish source review, local tests, native
    host evidence and live provider acceptance. Scale validation to risk.

During review, identify the resource contract, authority boundary, cancellation
owner, failure outcome and smallest credible test. Track older gaps in the
[reliability audit](agents/tasks/recurring-codebase-hygiene/reliability-audit.md),
then implement separately scoped repairs.

## PipeLang foundation

The [foundation contract](concepts/pipelang-foundation-contracts.md#c2--functions-modules-callable-values-and-control-flow)
allows recursion under declared logical call-depth and step budgets. Keep this
contract: do not prohibit recursion or substitute a host timeout for deterministic
logical exhaustion. The foundation document is a design/delivery contract;
implementation must be established for the specific compiler, target and execution
path under review. Do not describe planned budget semantics as implemented merely
because the contract exists. Host containment remains an independent layer.

## Inspiration and scope

This standard adapts the emphasis on reviewable control flow and resource bounds
from Gerard J. Holzmann's JPL-era
[The Power of Ten (2006)](https://spinroot.com/gerard/pdf/Power_of_Ten.pdf)
and NASA's [Coding Standards guidance](https://swehb.nasa.gov/spaces/SWEHBVD/pages/102695794/9.03+Coding+Standards).
It is tailored to Dockpipe's Go, C++, scripts and language runtime. It does not
adopt every flight-software C restriction or claim NASA/JPL certification.
