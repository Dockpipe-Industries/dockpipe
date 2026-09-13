# PipeLang Qt integration boundary

Status: approved bounded integration continuation, 2026-09-13. The
[C++/Qt pilot](../research/pipelang-cpp-qt-pilot.md) remains completed experimental
proof. This contract applies its findings to a resolver-side reference adapter;
it does not promote C++ to full-language acceptance or implement a production UI.

The [compiler contract](../agents/tasks/pipelang-reactive-application-language/compiler-contract.md)
owns language values, semantic/Core identities and inert analysis.
[TASK-020](../agents/tasks/declarative-application-surfaces-and-target-builders/targets.md)
owns Application IR, bindings, component identities and target capability admission.
The target adapter consumes validated identities and typed outcomes; it cannot
parse source/YAML, redefine language types, or gain host authority from metadata.
Qt objects, signal scheduling and display strings stay outside Core. Future
packaging belongs to resolver-owned assets, not generic engine code.

## Values and presentation

The [reference implementation](../../tests/pipelang-qt-boundary/boundary.hpp)
has an explicitly bounded scalar envelope: signed 64-bit integer, Boolean, owned
UTF-8 text, the existing checked arithmetic overflow error, or adapter failure.
No implicit QVariant, floating-point or JavaScript-number round trip is allowed.
Record/list/Optional/enum envelopes require future typed identity-preserving
mappings and corresponding proof; this envelope does not claim their support.

Text copies cross the boundary using explicit lengths. Initial byte-order marks, embedded NUL, combining
sequences and supplementary scalars round-trip exactly. There is no normalization,
locale conversion or replacement decoding. Invalid UTF-8 and unpaired UTF-16
surrogates refuse. A declared adapter profile bounds text to 1 MiB of UTF-8 bytes;
this is admission policy, not a new PipeLang language limit. Refusal preserves the
error category and cannot silently truncate or repair input.

The typed outcome is authoritative. Display text is a presentation projection,
never input to control flow or a reconstructed language Result. Arithmetic overflow
retains its domain category. Invalid text, unknown domain tags and host failure
remain adapter/infrastructure outcomes. Raw exception details, command output and
secrets are not display text. Adding another domain error requires an explicit
mapping; unknown tags must not become overflow. Allocation/containment failure is
infrastructure failure, not an invented language error.

## Lifetime and queued completion

A delivery object requires an owner on the application's main/GUI thread. Qt parent
ownership supplies deterministic adapter destruction; it does not establish
PipeLang Class/reference semantics or garbage-collection policy. Cross-thread
invocation of mutable delivery state refuses in release builds. GUI-only getters
and request creation enforce thread affinity. The host owns and joins workers;
this reference adapter does not implement language tasks or detached execution.

Each view request gets a monotonically increasing, nonzero identity scoped to its
live delivery object. A new request supersedes the previous presentation request.
Clearing a previous result notifies property bindings. Only a matching pending request may publish one terminal outcome. Late, duplicate,
zero and stale identities do not change current state. Identity exhaustion refuses
instead of wrapping. These are presentation rules; they cannot cancel, roll back
or erase a host operation's recorded outcome.

Workers send an owned Completion value through an explicit `Qt::QueuedConnection`
whose receiver/context is the delivery QObject. Workers must not capture a raw
view/delivery pointer or use context-free callbacks. The host must finish creating
connections before starting workers and join workers before destroying their
sender. Qt removes queued deliveries when the receiver is destroyed. No QPointer
check followed by an unguarded cross-thread dereference is a lifetime protocol.
Notifications run only after the complete typed outcome has been stored on the GUI
thread. Request ordering is scoped to each receiver; no global scheduling order is
claimed.

## Foundation reconciliation and acceptance

This boundary realizes target-side obligations of C1 value identity, C3 owned
resource lifetime and C4 governed outcomes in the
[foundation contracts](pipelang-foundation-contracts.md). It does not settle D1-D6,
implement P15/P16/P17/P20, or bypass their prerequisites. P01/v0.113 remains accepted;
the [dependency plan](pipelang-foundation-delivery.md) remains authoritative for
new language capabilities. Foundation continuation here repairs the existing
zero-argument record transport panic, under current versioned acceptance/refusal
rules, before another P-number is selected.

The deterministic Qt Core event-queue harness uses the main application thread,
a real worker and moc-generated signal machinery, with generated pilot arithmetic.
It checks independent expected values, byte ownership, malformed text, typed errors,
late/duplicate results and receiver destruction. It does not rerun the completed
visible xcb window or claim new widget/layout/platform/packaging proof. Go remains
the accepted backend; no Core, Application IR schema, authored syntax or engine
execution contract changes are implied.

Verified on 2026-09-13 under the existing containment policy: final Qt checks,
99 zero-argument source/HIR/Core/evaluator/Go-generation cases across v0.15-v0.113,
three construction refusals, fresh native and direct compiler cases at v0.30/v0.31/
v0.113, nine inherited record/predicate tests, five selected enum/native/lazy-trace
tests, Application IR and frozen compatibility. Independent acceptance rehashed
16099 current input identities and the accepted artifacts, with process-tree
cleanup verified. The [objective record](../agents/tasks/pipelang-reactive-application-language/qt-boundary-foundation.md)
locates retained receipts, failed preparations and the exact proof limits.
