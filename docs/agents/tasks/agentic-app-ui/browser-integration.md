# Optional DorkPipe Browser Integration

## Status and ownership

Deferred TASK-008 scope, recorded 2026-10-17. This records the user's future
browser-assistance direction; it does not authorize AI implementation now.
Ordinary browser settings, navigation, downloads, extensions and page tools are
DockPipe functionality and continue independently of this backlog. Password
storage and autofill use user-selected compatible browser extensions; DockPipe
is not adding a password vault. Extension compatibility requires separate proof.

DorkPipe is the optional package owner for AI behavior, model/provider selection,
context assembly and execution. DockPipe owns reusable browser/session operations,
contribution registration and authorization. Reuse the built-in browser component
and its spatial Live page integration; do not duplicate their browser logic or
make either depend on AI. [The canonical Browser guide](../../../../src/app/tooling/dockpipe-launcher/README.md#browser)
describes the currently available ordinary browser.

## Intended capabilities

- Summarize or ask questions about an explicitly selected page, selection or
  user-approved capture, with source/page identity and visible context scope.
- Send an annotated page capture and the user's request to DorkPipe. Show exactly
  which image, annotations and text will be included before submission.
- Perform browser actions through shared operations: navigate, inspect, scroll,
  click, type and manage explicitly selected tabs where supported. Page content
  and model suggestions do not authorize actions or widen site access.
- Offer explicit, scoped history access when the user requests it. History is
  separate from current-page access; do not send every tab, prior visits, cookies,
  authentication secrets or stored extension credentials as ambient context.
- Contribute optional browser actions or panels through package contribution
  contracts. Disabling or uninstalling DorkPipe removes AI controls, cancels its
  outstanding work and revokes its session access while ordinary browsing,
  downloads, extensions and spatial worlds remain usable.
- Contribute separate, arrangeable chat-input and output widgets to custom
  dashboards through the same package contribution contract. Reuse the desktop's
  retained canvas/tiling widget lifecycle. Ordinary dashboard widgets and window
  hosting must remain useful with DorkPipe absent; no implicit page or window
  capture is granted by placing a chat widget next to them.

## Future Send input targets

The ordinary [Terminal and Send input widgets](../../../../src/app/tooling/dockpipe-launcher/README.md#terminal)
belong to DockPipe. Send input currently addresses one selected terminal session
and sends text followed by Enter. It does not broadcast, and this ordinary shell
input path does not imply an AI package API or authorize AI execution.

Future package contributions may register other explicitly selected destinations,
including DorkPipe conversations or runs. Keep target identity, supported input
types, destination preview, authorization, cancellation and delivery outcomes in
a reusable contribution contract. Do not hardcode DorkPipe execution or provider
selection into the input widget, reinterpret shell input as an AI request, or
silently fall back to another target after session removal. A failed or revoked
destination must produce a visible outcome without resending elsewhere.

Package removal must remove its targets and cancel pending delivery while normal
terminal input remains available. Adding an AI destination does not grant it
terminal output, browser pages, history, captures or nearby dashboard content;
those require separately scoped access. This is deferred TASK-008 work, with no
AI controls or generic package target registration implemented by this record.

## Authorization and runtime boundaries

Core runtime and CLI/API operations remain authoritative; desktop and unified
MCP consume the same operations. A headless installation must be able to use
supported browser automation through those shared contracts without requiring
the desktop UI. Inventory and qualify actual backends before exposing commands;
today's Qt BrowserWidget is not proof of a headless browser-control API.

Require explicit site/origin approvals and an identified browser session/tab
before AI reads page content or performs actions. Bound grants by capability,
source, duration and selected package identity/version. Check redirects, new
windows, cross-origin frames and later navigation against the applicable grant;
access to one page cannot silently expand to another site or all tabs.

Keep identity, capability permission and approval for a consequential action
separate. A logged-in website, a selected model or an enabled package is not an
approval to submit forms, send messages, purchase, publish, upload or delete.
Reuse existing review and execution approval contracts for those actions. Browser
content, downloaded material and extension output remain untrusted input.

History/capture requests must preview their scope and destination provider.
Apply existing source permissions, secret-reference rules, retention/export
choices and cancellation. Do not silently turn the browser's session-only data
into persistent AI history or upload it to a model provider. Persisted knowledge
or decisions belong to DORK-001 through its separately authorized operations.

## Missing reusable contracts

The current BrowserWidget composition shares ordinary UI tools and selected-view
lifecycle with spatial browsing. It does not yet provide a supported package API
for browser contributions, authorized page access or headless automation.

Coordinate the smallest reusable seams with [TASK-039](../unified-mcp-gateway/overview.md):

- Browser session/tab identity, supported operations, capability discovery and
  bounded results, including an honest unsupported outcome for backend gaps.
- Package-owned action/panel registration, selected-version binding, enable/disable
  cleanup, cancellation and removal without embedding DorkPipe logic in core.
- Site grants, scoped context/capture/history access, operation results and
  approval/audit records enforced identically through CLI/API, UI and MCP.
- Safe active-tab/session replacement: cancel stale work before page disposal,
  preserve user navigation and keep the single spatial projection/media lifecycle.

Use existing runtime/network/credential/package services wherever available.
Do not introduce a second configuration store, independent UI execution path,
mandatory AI daemon or new browser framework merely to host the contribution.
This work follows the existing core/MCP/knowledge delivery sequence and does not
block ordinary browser settings or the current staging checkpoint.

## Acceptance before delivery

Prove ordinary and spatial browsing still work with DorkPipe absent, disabled or
removed. Verify denied or revoked site grants, redirects and tab replacement
cannot leak page/history content or continue stale actions. Exercise annotated
request preview/cancellation, independently granted history access, bounded
results, provider failure and teardown while an action is pending. Qualify one
supported headless backend against the same permission and cancellation checks;
UI-only evidence is insufficient. Do not claim any of these APIs or AI features
are implemented from this backlog record.
