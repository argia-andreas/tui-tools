# streams validation — 2026-10-30

This records implementation acceptance, not a deployment or production-hardening
claim. Streams remains an experimental read-only viewer.

## Executed acceptance

| Check | Result |
| --- | --- |
| `make check` | Formatting, vet, independent module race tests and both Python lab safety suites passed. |
| `./build` | All four app binaries and transfer MCP built. |
| `make test-terminal` | All four actual app PTY suites passed; streams includes mouse/color and mouse-off/NO_COLOR. |
| streams public UI tests | Investigation, seek validation, draft preservation, held late open after Pause, latest catalog wins, connection-switch operation isolation passed. |
| streams output/profile tests | Exact bytes/null/empty/headers, safe controls, JSON/raw/hex bounds, config validation, private settings and credential construction passed. |
| Redpanda app integration | Read/replay/resume/noncreation and seeded workflows passed with **viewer** credentials. |
| Kafka app integration | Same app contracts passed on Apache Kafka loopback plaintext. |
| Independent lab checks | Both brokers passed; Redpanda additionally enforced negative write/create/commit/describe, invalid credentials and untrusted CA cases. |
| Rendered actual app | Inspected wide topics/partitions/events, group lag, seek error, compact inspector; recorded mouse and keyboard workflow against real Redpanda. |

The opt-in application tests are `TestRealBrokerReadOnlyContract` and
`TestSeededLabWorkflows` in `apps/streams/internal/kafka/integration_test.go`.
They require explicit configuration and do not seed or mutate the broker. Run
with `STREAMS_SEEDED_LAB=1`, `STREAMS_KAFKA_TEST_CONFIG` set to the lab's absolute
`connections.json`, `STREAMS_KAFKA_TEST_CONNECTION=redpanda-viewer` (then
`kafka-local`), `STREAMS_KAFKA_TEST_TOPIC=orders.created`, and
`STREAMS_KAFKA_TEST_GROUP=orders-reader`:

```sh
GOWORK=off go -C apps/streams test -count=1 -race ./internal/kafka \
  -run 'TestRealBroker|TestSeeded' -v
```

Decisive output on each real broker:

```text
read 4 owned records (237 raw bytes); replay/resume and noncreation verified
31 committed records, exact binary/null/headers, offset/time seeks, follow/close and unchanged group positions verified
PASS
```

The 31 records span two three-partition topics: 3/5/7 base records each plus
one committed transaction in `orders.created`. The independent lab client proves
aborted/open records really exist with a read-uncommitted control. The app reads
committed only. Group commits remain 1/2/4 on both topics. Timestamp seek uses an
asymmetric p2 timestamp (+202 seconds → offset 2); exact offset seek uses offset 3.

## Issues caught during acceptance

- Actual terminal Space events were initially dropped by seek input; corrected
  and exercised by the actual binary PTY, not just synthetic rune messages.
- Lab CA certificates initially had public file modes, contrary to the profile
  contract. Installation now makes referenced certificate files private too.
- The real Follow contract initially rejected the UI's `Mode=follow, Follow=true`
  request. The live integration failed on both brokers before correction and
  passed after correction.
- Review found connection-switch overlap and out-of-order catalog refresh races.
  Backend operations are blocked while switching, batches carry connection
  identity, connection installation invalidates reads, and catalogs have tickets.
  Public UI regression tests exercise held and reordered results.
- ttyd's canvas/WebGL rendering mis-scaled at DPR2; the review preview uses its
  DOM renderer. This was a preview problem, not accepted as an app screenshot.

## Compatibility and remaining limits

| Runtime / feature | Evidence |
| --- | --- |
| Redpanda v26.2.5 | Real app + independent lab; TLS/SCRAM-SHA-256, viewer ACLs, transactions, seeks, follow/close, groups. |
| Apache Kafka 4.3.2 | Real app + independent lab; local plaintext, transactions, seeks, follow/close, groups. No broker ACL claim. |
| mTLS / PLAIN / SCRAM-SHA-512 | Configuration/client construction covered; live handshakes **not verified**. |
| Compaction/retention races, huge broker batches | Logic/limit tests exist; full live stress/race matrix **not verified**. |
| External clipboard / human terminal usability | Safe byte payload and explicit transport outcomes covered; actual OS clipboard receipt/human acceptance **not verified**. |
| Azure Event Hubs / other managed brokers | **Not verified**, no vendor login/administration integration. |

Historical reads can return every visible record but remain explicitly incomplete
at invisible trailing transaction/control gaps. This is intentional: no fabricated
next-scanned cursor, watermark jump or silent data loss. Broker batch soft limits
and decoded allocations mean the limits are not an RSS guarantee. Topic recreation
detection depends on usable broker topic IDs. No under-60-second human workflow
metric was measured. See app/lab READMEs for security and runtime details.

The local lab and preview services are opt-in, isolated to loopback/private state,
and were left running for review. Brokers are not exposed through portals.

## Approved UI refinement — 2026-10-31

Andreas selected A (Compact) as default and B (Comfortable) as optional, clarified
that keyboard row selection—not mouse hover—drives the inspector, and approved
implementation. Both layouts and the Enter payload modal are implemented with
the existing shared actions/navigation/surface library.

The actual-binary streams PTY now checks Compact default, j/k preview identity and
payload, modal input capture/scroll/raw/Esc, distinct rendered JSON token colours,
uniform pane backgrounds through nested SGR resets, density change, explicit
save/restart persistence, existing seek/help/follow/group flows, and mouse-off /
NO_COLOR. Both scenarios pass. Public UI tests also check select-only mouse
motion, Comfortable's second-line hit target, and modal close after 60×16 resize.

Inspected 2× ttyd DOM-rendered fixture captures: Compact list + inspector,
Comfortable list + inspector, floating JSON payload modal, ordered duplicate
headers, and narrow modal. A short real-pointer/keyboard recording shows selection,
Enter and Esc without pointer-hover selection. Artifacts are under
`.amp/in/artifacts/streams/redesign-*`; fictional fixture data is labelled.
These are actual application captures, not generated mockups.

The redesign does not change Kafka protocol behaviour; previous real-broker
acceptance and its explicit limitations above remain applicable. No new live
authentication or stress-matrix claim is made by these visual checks.

### Comfortable card correction

The initial implementation incorrectly turned B into spaced table rows. After
Andreas pointed this out, the original approved A/B/C image was re-read and
Comfortable corrected to bordered event cards with metadata left, key/payload
right, a cyan selected outline retaining syntax colours, and a topic header
switcher instead of the permanent sidebar. Compact remains default and unchanged.

`go vet ./...`, `go test -race ./...` and `make test-terminal` pass from
`apps/streams`. The public UI regression checks bordered cards, absent topic
sidebar, second-content-line select-only hit targets, topic switching and retained
selection across the payload modal/resize. Both PTY colour/mouse scenarios and
save/restart checks still pass. Final rendered evidence is
`comfortable-cards.png`, `comfortable-cards-narrow.png` and
`comfortable-cards-flow.mp4` under `.amp/in/artifacts/streams/`.
