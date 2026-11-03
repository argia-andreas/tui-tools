# QE architecture and safety contracts

Maintained guidance for the unified inspector, saved environments and their
shared-terminal-UI integration. This replaces the completed plan groups linked
below; it is not a new execution plan or authorization to ship or touch Azure.
See the [app README](../README.md) for current commands and keys and the
[click inventory](ui-click-inventory.md) for paired keyboard/pointer behavior.

## Runtime and ownership

QE (currently the `queue` executable with a `QUEUE` runtime header) is an
independent Go/Bubble Tea terminal process, not an HTTP service or broker bridge.
The UI uses explorer contracts backed by deterministic fixtures,
the fixed disposable emulator, or an explicitly selected read-only Azure
connection. There is no silent fallback between modes or credentials.

- `internal/explorer` owns resource, message, count, replay/delivery and
  `ScopeProvider` contracts. Azure SDK objects and full replay content stay in
  the backend, not rendering state. Fixtures implement the same user-facing flow.
- `internal/azure` owns direct Service Bus/management access, exact-message
  reads, resource/connection validation and broker lock lifetimes.
- `internal/config` owns validated nonsecret preferences, deep save snapshots
  and private atomic replacement. Malformed/unsupported files fail startup,
  never silently reset. No credentials, message bodies or grants are persisted.
- `internal/ui` owns environments, staged edits, inclusion, request generations,
  inspector selection, review phases, visible refusal reasons and outcomes.
- `lib/terminalui` owns shared surface chrome, palette, Help, geometry, focus
  traversal and semantic input intents. Domain effects remain app-owned; there
  is no generic environment manager, form engine or provider adapter in the lib.

The executable is `bin/queue`, with `cmd/queue-lab` as the local lab helper.
The module remains `servicebusexplorer`; preferences remain at
`os.UserConfigDir()/service-bus-explorer/preferences.json`. Existing `SBX_*`
variables, lab owner markers and private state identifiers are not alternate
executable names and must not be reset merely to change branding.

## Resource identity, discovery and environments

Azure subscription → resource group → Service Bus namespace → queue or topic →
topic subscription is the hierarchy. `Resource.Subscription` means a **topic
subscription**, not an Azure subscription. A topic groups subscriptions; it has
no independent DLQ. Resource identity includes profile/connection, namespace and
queue or topic/subscription, never just a display name.

An environment has a stable ID, unique user name, one connection context, exact
namespace identities, and its own All/Selected dashboard inclusion. Names such
as Production are user labels, not permission or safety guarantees. Multiple
Azure subscriptions in one tenant are supported; cross-tenant aggregation and
sovereign clouds are not. Local fixture/emulator environments do not mix with
cloud inventories.

`ScopeProvider` exposes explicit subscription/namespace metadata queries, known
namespace resolution and per-namespace discovery results. Discovery receives a
concrete namespace slice, never resource-group rules. All inventory, browsing,
counts and background refresh stay inside the active environment. Azure startup
without an environment performs no global entity inventory scan. The editor
loads namespace metadata only for explicitly opened Azure subscriptions.

Resource groups are batch-selection aids: a complete successful listing produces
a deduplicated, reviewable snapshot of exact namespaces. Failed/partial listings
cannot claim complete group membership. **Future namespaces never join
automatically.** Known namespace entry accepts a full public-Azure address or
ARM identity without requiring subscription-wide enumeration. Unknown metadata
remains unknown; knowing an address does not grant inventory/count/peek access.
Namespace metadata permissions and message permissions are operation-specific,
not interchangeable. Do not simplify Azure data roles into having no management
permissions, or promise child-scoped grants make broad listing succeed.

Dashboard inclusion is separate from discovery. One per-environment list controls
both grid and table; unwatched entities remain browsable inside saved namespaces.
New environments start Selected/empty. Filtered bulk selection affects only the
current matches. Explicit All mode warns that future entities **inside saved
namespaces** will be included; it never adds namespaces. Returning to Selected
restores explicit picks. Removed namespaces retain picks as outside scope, not
accessible/countable; re-adding the exact identity restores them. A same-named
resource elsewhere never inherits them. Appearance remains global.

The editor stages changes until Apply. Apply changes the session; **Save
preferences persists all applied changes**, including active environment and
inclusion. Cancel discards the form only. Save failure remains visibly unsaved.
Dirty quit offers Save / Discard / Cancel; save-and-quit waits for a successful
save of the current revision, not an old completion or unapplied draft.
Rename retains identity/inclusion; duplicate requires a new name. Name-confirmed
delete removes local preferences only. Deleting the active environment leaves
no active inventory, never silently selects another. Local watchlists migrate
once, preserving absent-means-All versus explicit-empty-means-none; deleted
defaults are not recreated.

Switching returns to the destination Dashboard, clears old message ownership,
cancels old reads and invalidates scope/count/peek/inspector/clipboard callbacks.
Late results cannot relabel the new environment. Review acquisition, held locks,
submission, release, full-content loading and unacknowledged terminal outcomes
block switching. Changing environment cannot erase an unknown broker outcome.

## Azure identity and network boundary

Live mode explicitly uses an existing modern Azure CLI login (`expires_on`
token JSON), an explicit tenant and public Azure. It does not log in, acquire
roles, store credentials, use a silent credential chain or introduce browser
authentication/vault support. Pin **tenant plus immutable principal object ID**,
not username; changed principals/tenants and wrong token audiences fail closed.
JWT inspection is a consistency check on trusted CLI output, **not signature
validation or an authorization grant**. Actual service authorization still applies.

Management metadata and Service Bus administration/peek paths are read-only.
Cloud receive/send/delete/selected-copy preparation is refused, not enabled by
saved environments. Preserve original ARM identity and validate namespace address
mappings before counts/peek; moved resources require review. Reject redirects,
unsafe/cyclic pagination and continuation links escaping the selected collection,
even on the same origin. Errors must not disclose credentials or copied payloads.

Saved environments and dashboard inclusion grant neither Azure permissions nor
MCP access. No MCP server exists here. Future agent grants must be independently
deny-by-default and checked per full source/destination identity and operation;
read never implies resend, and resend never implies deletion. Bodies/properties
are untrusted data, not agent instructions; enabling future agent reading would
also require explaining the model-provider data boundary. No credentials via MCP.

## Counts and bounded reads

Counts distinguish genuine observed zero, unavailable (`—`) and stale (`~`),
with observation time and error detail. Refresh uses bounded sweeps, at most four
concurrent reads and a 30-second interval after each sweep; it must not block
input or overlap into an uncontrolled poll loop. Failed reads retain last values
as stale. Partial namespace discovery stays visibly partial. Only a complete
successful relevant listing establishes Not found; a failed listing means Could
not verify. Preserve saved missing selections instead of silently dropping them.

Peeks use bounded pages and explicit sequence positioning for next/previous and
refresh; an implicit SDK cursor can otherwise skip prior content. A bounded peek
page is never a total. Emulator 2.0.1 omitted `CountDetails`; observed SDK paths
either rejected it or returned misleading zeroes. **Emulator counts remain
unavailable**, regardless of insertion expectations or SDK zeros. Fixtures prove
presentation, not live counts; accurate Azure totals require separate cloud checks.

## Unified read-only inspector and clipboard

Enter on an Active/DLQ row focuses the existing inspector, never receives/locks
or sends. The passive inspector follows the list; focused text pins its row and
preserves logical cursor/scroll through resize/Help. A new message/page clears
selection. Do not auto-refresh text under an active selection. Temporary reveal
does not change saved visibility. Esc clears selection/pending operator before
returning to the list; explicit Back returns directly. Reader text owns its
grammar (`b` is word-back, not sidebar; list paging/refresh keys do not leak).

The selectable document has labelled metadata, original body and readable
application properties. It is not an editor. `yy`/`V` operate on original logical
lines; `v` has inclusive grapheme-safe endpoints. Preserve exact UTF-8, tabs,
CRLF/newlines and final-newline presence, excluding gutters, soft wraps and chrome.
`Y` copies only the full body; Copy message report produces labelled readable
text, **not a lossless broker export or replay payload**. Non-text/binary text
copy is unavailable; an empty body leaves the clipboard unchanged.

Preview truncation is explicit; fragment copies say preview fragment. Full body
or report copying asks to load full content with Cancel default.
`Store.PeekMessage` reads the exact Active/DLQ sequence without receive/lock;
returned sequence and message ID must match. Missing/changed content never
substitutes a neighbor. Cancelled, stale or failed loads leave clipboard/content
unchanged; successful loading shows full size and requires a second copy command.

Clipboard writes carry immutable text and identity/generation. Native local
`pbcopy`, `wl-copy`, `xclip` or `xsel` success says Copied; remote SSH/Mosh uses
OSC52 and says **Copy requested · terminal confirmation unavailable**. The OSC52
limit is 1 MiB, never silent trimming. No clipboard read-back, content logging,
persistence or third-party copy service. Failed writes retain selection. Terminal
permissions can block requests; tmux requires `set-clipboard on`, and screen
wrapping is supported. Manual terminal copy is an explicit fallback that may
include gutters/wraps. Untrusted terminal control sequences must not execute or
trigger clipboard writes; safe rendering is distinct from exact user-requested
copy bytes. Native macOS clipboard execution and screen-reader usability remain
unverified, not established by screenshots or OSC52 transmission tests.

## Reviewed DLQ mutation: two distinct acquisition paths

Only explicitly selected local modes support these actions. Reading/copying
never starts acquisition. Resend preparation/review/outcome reuse the existing
mutation state machine inside the inspector, not a second broker lifecycle.

**Selected-message resend:** queue DLQs are fixed to that queue's Active target;
subscription DLQs require an explicit same-namespace existing queue picker,
including unwatched queues. No topic destination, arbitrary endpoint or
cross-namespace destination is implied. Cleanup defaults unchecked.

- Copy-only re-peeks the exact full identity/sequence/ID; no receive, scan or lock.
- Cleanup checked acquires the **selected** original via bounded non-session
  PeekLock receive-and-match, prefetch off: at most 100 individual receives or
  five seconds, whichever ends first. Deferred-message receive cannot acquire
  an ordinary DLQ message. Missing/locked/expired/beyond-budget means stop,
  never substitute receive-next or claim the queue empty.
- Nonmatches remain held until scan end to avoid reacquisition, then each gets
  one application Abandon attempt within a shared two-second cleanup context
  independent of caller cancellation. Receiver close has a separate two-second
  bound. Uncertain unrelated release aborts before sending, attempts target
  release too, and reports uncertainty; Close alone does not prove release.
  Search may take five seconds **plus** bounded cleanup/close. Disclose temporary
  unrelated locks and delivery-count effects; no unbounded scan or deferral.

**Receive next DLQ message…:** independently acquires the next available message,
not the highlighted peek row. Review shows the actual held identity. Its replay
goes to the original queue/topic; topic replay warns about fan-out to other
matching subscriptions. Standalone typed `delete` authorizes deletion of that
actual held message, not any selected preview.

Both paths freeze source, destination and cleanup for final review; show the new
message ID, body/representation changes, consequences and lock deadline if held.
Configuration and review start on Cancel; text Enter/visual `y` never confirms.
Back-to-edit releases any held lock first. Cancellation/quit/late acquisition
must account for held locks, not simply discard a result as stale. No automatic
lock renewal or replacement after expiry. Expiry disables send/Complete.

Replay creates a new copy with a new ID, supported body/scalar properties and
correlation ID; original TTL duration becomes a fresh enqueue lifetime, scheduled
enqueue is cleared and broker DLQ reason/description fields are stripped.
Clone mutable data rather than relying on a shallow SDK conversion. Refuse
unsupported sessions and advanced AMQP bodies/IDs instead of silently converting.
Full outgoing content never comes from truncated display text.

**Submit each send/settlement once at application level.** The accepted pinned
azservicebus 1.11.0 behavior includes internal recovery/retry and settlement
fallback that can repeat wire attempts even with negative configured retries.
No SDK patch, single-wire-attempt, atomic move, duplicate-free or exactly-once
guarantee is claimed. Single-flight input blocks repeat submission.

Only **confirmed send** permits separately authorized checked cleanup.
**UNKNOWN send → retain original, no cleanup, check destination before retrying.**
Unknown settlement is not confirmed deletion. Confirmed send with failed/unknown
cleanup is a distinct partial result. Cancellation after submission is not broker
rollback. Report release uncertainty separately and require outcome acknowledgement;
an outcome screen cannot resend again. Explicit refresh returns to broker truth.

## Shared interaction and responsive safety

Render current rows/fields/controls and clipped geometry together. Targets use
native environment IDs, namespace ARM context or complete resource/message
identity, never row indexes/display names or cached frames. Key and pointer
routes call the same checked semantic operation, not synthetic keys. Recheck
phase, authorization, busy/expiry state, current identity, dimensions and whole
control visibility immediately before effects.

Rows select only; explicit Open/Use/Choose/checkbox/actions activate. Press
activates once; release/motion never repeats. Wheel affects only the pointed
visible body. Hidden controls leave focus traversal after resize/update.
Dismiss/Esc/close/outside press share app-owned transitions: clear search first,
dirty-form confirmation, invalidate requests, restore the right parent. Refused
outside dismissal is consumed, never forwarded to underlying content.

Always process resize and async domain lifecycle messages (including expiry,
release, save and generation checks) even under Help. Ctrl+C follows safe quit,
not an unconditional shared exit. Help captures input and restores draft/query/
selection/focus while lifecycle work continues. Fields own printable q/j/k/s/?;
F1 still opens Help. Forms block global Actions over staged edits; Ctrl+P is
consumed/refused there, not an alternate previous-row command. Review/full-load
also block Actions. Only an uncaptured owner passes input to the workspace.

Use full-screen stepped environment panels rather than force side-by-side forms.
Scope-enabled panels support 60×18; scope-less reader compatibility supports
60×16. Broker confirmation requires 80×24. Below minima preserve drafts, show
resize guidance and only visible safe quit/cancel options, never hidden mutation,
Apply, Save or discard submission. Compact held-body pages are read-only and
cannot submit hidden confirmation. Labels, named focus and ASCII fallback
supplement theme colors; browser terminal inspection is not native accessibility
acceptance. `--mouse=false` preserves keyboard/native terminal-selection fallback;
mouse reporting does not add application drag selection or prove tmux/SSH parity.

## Disposable lab boundary and platform limits

Use only the pinned development emulator/SQL images and fixed
`sbx-explorer-lab` project. Require explicit Microsoft **EULA acceptance** for
startup/reset/seed and **disposable confirmation** for destructive operations.
Seed destroys/recreates this lab before insertion; do not infer cleanliness from
unreliable counters. Reject cloud endpoints/project overrides; verify fixed
project/images/health/loopback targets before messaging. Neither lab CLI nor TUI
accepts a cloud connection string. Never point lab tooling at Azure.

AMQP is loopback 5673, management loopback 5301; emulator-only HTTP transport
accepts the exact configured authority and refuses redirects/other authorities.
Cloud must never install that transport. Generated SQL credentials remain private
outside Git under `~/.local/share/sbx-explorer-lab/`. Orb services are supervised,
not shell-background daemons. Lab checks run serially after fresh seed, without
other consumers, and mutate disposable data. Drain/dead-letter report actual
acknowledged settlements, not total backlog. Unknown effects must not be blindly
retried. A historical unpaced seed hit throttle 50002 with unknown send and was
reset, not resumed; 100 ms pacing was a measured local workaround, not a quota.

The emulator is not Azurite, not production, and cannot validate Entra/RBAC.
It uses AMQP TCP (not WebSockets); lacks partitioned entities/JMS and loses
entity/message state on restart. Historical documented limits were one namespace,
50 queue/topic entities, 50 subscriptions/topic, 256 KB messages and ten concurrent
connections; consult upstream before relying on quotas. Multi-namespace fixtures
do not prove multi-namespace emulator behavior. SQL Edge is retired; historical
ARM image availability does not prove Apple Silicon support. Executed lab evidence
is x86_64 Linux only. POSIX PTYs require Linux/macOS; native macOS/Windows and
Apple Silicon parity remain unverified, not proved by cross-compilation.

## Historical verification (not current/live proof)

The retired records describe checks executed in their original checkouts. No
runtime test, Azure connection, broker operation or platform validation was run
as part of this documentation consolidation. Old artifact paths below are evidence
locators in their original threads, not guaranteed assets in a fresh checkout.
Mockups are historical design studies with synthetic data, never runtime proof.

| Work and period | Useful recorded evidence | Limits |
| --- | --- | --- |
| Unified inspector, 2026-10-24 | Race tests, vet, ten Python lab guards, real fixture PTY with independently decoded OSC52 line/selection/full-body bytes; serial seeded real-broker integration and emulator PTY, exact 90 KB Active/DLQ reads, neighbor rejection, selected-only cleanup, subscription-to-queue and topic fan-out control. Actual terminal themes/ASCII, 80×24/60×16 and pointer/keyboard recording inspected. | OSC52 transmission is not receiving clipboard acceptance. Native macOS clipboard and screen readers untested. Ambiguous/expired/partial screens use deterministic offline cases, not induced broker failures. Disposable lab mutated; reseed before rerun. |
| Saved environments, 2026-10-24–25 | Race/vet/lab guards plus real fixture workspace/environment PTYs; exact cross-subscription/group snapshots, Apply != Save, private JSON/restart, Selected-empty, isolated inclusion, edit/duplicate/delete, partial/missing/stale and late-result regressions; renders at 160×48, 80×24, 60×18 and undersized quit. | No live Azure account/CLI validation; injected CLI/HTTP/SDK XML/paging tests are not RBAC/network/AMQP proof. Docker daemon/socket unavailable for that change, so no seeded emulator rerun. Partial screen was offline simulation; no 500-entity moderated usability or timing study. |
| Environment/shared-UI integration | Upstream baseline `23feda384c55bc3e77c06b4b9e767c6aa09a64b9` independently built/tested before adaptation. Combined `make check`, `./build`, auxiliary lab build and `make test-terminal` passed; all then-existing three apps plus terminalui checked. Queue workspace/environment PTYs ran mouse-on and `--mouse=false`; saved JSON/restart independently inspected. Added Help-during-save, native-ID, release-ignore, clipping/hidden-target and compact-quit cases. Actual combined fixture terminal and moving-pointer clip inspected. | Isolated branch checks were not accepted as combined proof. No Azure connection, credential change, broker mutation or lab reset/seed in this integration. Native terminals, assistive technology and receiving system clipboard unverified. Historical module/path names differ from today's layout. |

Inspector evidence was indexed under `.amp/in/artifacts/sbx-inspector/` in the
[parent thread](https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937)
and its [proposal thread](https://ampcode.com/threads/T-01a16f9f-2222-700a-9739-28a0f45be4e6).
Environment captures and `integrated-*` follow-up evidence were indexed under
`.amp/in/artifacts/service-bus-environments/` in the
[environment evidence thread](https://ampcode.com/threads/T-01a16fa1-ebdf-72d9-bdfd-b97aa795af08).
Intermediate bad-color/cropped/resize-overlay captures were replaced before
inspection. No expired preview portal or old running-lab claim is a current service
guarantee. Historical shipping approvals are not fresh authorization.

For reproducible current commands use the app README: fixture race/vet/PTY and
guard layers are offline; serial emulator tests require an explicitly authorized
fresh disposable seed; real-Azure smoke checks require separately authorized
resources. None substitutes for the others.

### Retired plans and retained references

- [Unified inspector design studies](../../../docs/plans/service-bus-explorer/dlq-inspector/README.md)
- [Saved-environment design studies](../../../docs/plans/service-bus-scopes/README.md)
- [Completed UI integration](../../../docs/plans/service-bus-ui-integration/README.md)
- [Explorer safety/retry design record](../../../docs/plans/service-bus-explorer/03-program-design.md)
  and [lab research](../../../docs/plans/service-bus-explorer/local-development-notes.md)
  remain historical context; old source paths and prospective statements there
  must not override current mode/safety contracts above.
