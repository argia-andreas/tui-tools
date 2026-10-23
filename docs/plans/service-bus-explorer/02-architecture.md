# Architecture: Service Bus Explorer terminal UI

Gate 2 approved 2026-10-23: independent app/lab, direct SDK access and layered testing. The user also agreed to independent-first theme/layout reuse, with shared-library extraction only after contracts stabilize. All design gates are approved. Slices 1–2 now provide the fixture workspace, read-only Go emulator backend and reusable repository lab tooling. The earlier emulator capability experiment remains separate.

## Fit and isolation

- Build an independent Go application in `service-bus-explorer/` with its own module, dependency lock, build commands, lab and configuration. Do not change LazyFTP's module, commands, internal packages, Makefile, setup scripts, documentation or lab.
- Recommend Bubble Tea/Lip Gloss for a keyboard-first terminal UI and the official Azure Service Bus Go SDK for messaging/administration. The repository already uses this terminal toolkit, but that is familiarity, not authorization to import LazyFTP application code. Pin versions and validate SDK behavior in program design.
- Prefer direct SDK calls over a separate .NET broker bridge. The initial JavaScript/.NET capability experiment did not validate Go; Slice 2 subsequently executed the Go SDK discovery/peek path and independent receipt check against the pinned emulator. This does not establish cloud authentication or accurate runtime counters.
- Keep terminal rendering, explorer actions and Azure access separate enough that deterministic in-memory fixtures can drive the same user-facing flows. Do not build a generic plugin system or share storage abstractions with FTP.

## Endpoints

No application HTTP server or backend. Initially one local terminal process communicates directly with the selected namespace.

- Messaging: Azure Service Bus AMQP/TLS in future cloud mode; explicitly selected emulator mode uses local AMQP/TCP on host port 5673 (container 5672), separate from the older spike.
- Discovery/runtime properties: namespace administration endpoint in future cloud mode. The reusable emulator lab publishes HTTP management at host port 5301 (container 5300); the exact-loopback read-only transport rejects redirects and all other authorities. It is never a cloud transport.
- Browser Microsoft sign-in is a later delivery step. No embedded client secret or application-operated auth relay.
- MCP is deferred; do not expose a listener or stdio tool surface in the first implementation.

## Data and permissions

- Local preferences: theme, sidebar state, grid/table choice, named namespace profiles and watched full resource identities. No database required initially.
- Store persisted credentials only through a suitable OS credential vault. If unavailable, offer session-only access rather than plaintext fallback. Exact Linux/macOS providers and credential dependencies require Gate 3 review.
- Do not persist message bodies by default. Bounded message pages live in memory; no background whole-namespace body indexing.
- Represent unavailable, stale and failed counts distinctly from genuine zero. Dashboard watchlist membership grants no Azure or MCP permissions.
- Queue and subscription backlogs have separate Active and DLQ views. Topic nodes group subscriptions; do not synthesize topic DLQ totals.

## Flow

1. Load nonsecret preferences and select an explicit connection profile; begin development with connection strings.
2. Discover queues/topics/subscriptions; return understandable access errors without pretending sign-in/discovery proves message access.
3. Fetch runtime counts for watched entities asynchronously and with bounded concurrency. Retain last-known data with a stale marker on failure; do not tightly poll every entity or block keyboard input on requests. Exact intervals/limits are Gate 3 decisions.
4. Selecting Active/DLQ peeks a bounded page, with explicit sequence positioning for refresh/pagination. A probe showed that implicit peek cursors can advance within a client; refreshing must not silently skip the messages already viewed.
5. Inspector shows the selected message body and properties without receiving it. Message actions require explicit scope and a separate review/confirmation.
6. The user-approved mutation flow receives/locks the next available DLQ message, then reviews that actual message before resend/delete. It does not acquire a peek-selected message by sequence. Resend sends a new copy to the original queue/topic, and only then performs separately authorized optional cleanup. Distinguish send rejection, confirmed send with failed cleanup, and unknown outcomes. Do not auto-retry an ambiguous send as though it were safe. Topic confirmation warns about other matching subscriptions.

Supported Go SDK contracts do not acquire an arbitrary active DLQ message by its peek sequence number; deferred-message receive is not a substitute. The user chose receive-next/review rather than receive-and-scan. Cancellation/release can affect delivery count. Gate 3 specifies lock lifecycle, message conversion and unsupported-session behavior before mutation implementation.

## Local development and verification

Propose a standalone Compose lab inside this project's implementation directory, not the FTP lab. Pin emulator and SQL image digests; bind published ports only to loopback. Require acceptance of Microsoft's development license terms and generate local-only SQL credentials outside Git.

Provide project-local commands for start/stop, readiness, seed/reset, send, bounded peek, dead-letter, drain and integration checks. These commands must target only the disposable emulator and reject cloud connections. Seed at least 12 queues plus a topic with two subscriptions, asymmetric active/DLQ states, empty queues, JSON/text bodies and meaningful DLQ reasons. Reset/seed must be deterministic and repeatable.

Testing layers:

1. Instant fixture-backed domain/UI tests without Docker or Azure; represent counts as available, unavailable and stale. Real PTY checks for keyboard focus, palette, sidebars, themes and resize. Mock data must be visibly identified.
2. Serial seeded emulator integration checks for discovery, send, peek without removal, dead-letter reason/body, queue replay with separate cleanup, topic fan-out and drain. Restart/reset between scenarios as needed. Validate public results, not SDK call order.
3. Explicit opt-in cloud checks for Entra/RBAC, accurate runtime counts and emulator gaps. Use a disposable authorized namespace, never production by default. These checks must not be implied by a passing emulator suite.

### Verified emulator result and count limitation

The x86_64 orb successfully ran Docker and the two-container emulator. Messaging/discovery checks passed, including topic replay reaching both subscriptions. Current ~4 GB orb memory was sufficient; no resize is required for the tested lab. See `local-development-notes.md` for exact versions and results.

Runtime counter reads are not validated: emulator 2.0.1 administration responses omitted `CountDetails`; JavaScript SDK 7.10.0 rejected the response. .NET SDK 7.20.1 returned zero active/DLQ counts even after an actual send/dead-letter. No fabricated count fallback is acceptable. Fixture tests can validate dashboard presentation; emulator mode must report counts unavailable if the selected SDK observes this gap. Accurate cloud counts remain an explicit cloud validation requirement. Do not count a bounded peek page as an authoritative total.

Apple Silicon execution has not been tested. The SQL dependency's multi-architecture manifest contains an older ARM child; SQL Edge is retired. Document this development-only dependency risk, validate macOS in its own environment, and keep the lab replaceable. The app itself should not depend on the emulator's SQL implementation.

## External dependencies

- Go, Bubble Tea/Lip Gloss, Azure Service Bus SDK; future Azure Identity and OS credential-vault integration.
- Docker/Compose, Microsoft Service Bus emulator and its local SQL dependency for integration testing. No production database.
- Microsoft identity/management/Service Bus endpoints only for authorized cloud access. Exact environment variable names and packages belong in Gate 3, not guessed here.

## Approval scope

Approving Gate 2 selects the independent Go/TUI/direct-SDK shape, isolated reproducible lab, layered testing and honest unavailable-count handling. It does not approve program signatures, implementation slices, Azure writes, deployments or changes to LazyFTP. Gate 3 must specify the file boundaries, operations, resource limits and tests before implementation.
