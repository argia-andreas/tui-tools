# Streams

A small, read-only terminal workbench for Kafka-compatible event logs. Find a
topic, inspect exact event bytes, seek by partition/offset or time, follow new
events, and observe consumer-group lag without joining a group or committing
offsets. Experimental; see the compatibility limits below before real use.

## Start offline

From the repository root, with Go from `.go-version`:

```sh
./build streams
./apps/streams/bin/streams --fixture
```

Fixture mode is explicitly fictional and makes no network connections. Real
mode never silently falls back to fixtures. `--mouse=false` leaves selection to
the terminal. `Ctrl+P` opens Actions; `F1` opens Help.

## Connections

Profiles are explicit JSON, not broker URLs pasted into the application:

```json
{
  "connections": [
    {
      "name": "my-cluster",
      "brokers": ["broker.example:9093"],
      "tls": true,
      "ca_file": "ca.pem",
      "sasl": "SCRAM-SHA-256",
      "username": "viewer",
      "password_file": "viewer.password"
    }
  ]
}
```

```sh
./apps/streams/bin/streams --config /path/to/connections.json --connection my-cluster
```

Use broker-enforced read-only ACLs as defense in depth. The app has no producer,
topic administration, offset mutation, consumer-group membership or automatic
topic creation. Reads still use broker bandwidth. Only one connection/topic read
is active at a time; Switch connection in Actions selects an exact saved name.

TLS verifies certificates and hostnames; omitting `ca_file` uses system roots.
Optional `cert_file` and `key_file` enable a client certificate. SASL supports
`PLAIN`, `SCRAM-SHA-256`, and `SCRAM-SHA-512`, always over TLS. OAuth and cloud
credential acquisition are not supported. Referenced certificate/key/password
files resolve relative to the profile file, must be regular owner-only files
(for example mode 0600), and cannot have symlink path components. Passwords are
not interpolated; one final LF/CRLF is removed. Unknown/duplicate JSON fields,
null values and duplicate profile names are rejected. Errors do not echo secrets.

For an explicitly local test broker only, use `"local_plaintext": true` instead
of `tls`; all bootstrap and advertised addresses must be loopback. Credentials
are refused in plaintext mode. There is no insecure TLS bypass.

## Workbench

| Control | Result |
| --- | --- |
| `t` / `g` | Topics / consumer groups; explicit `r` refresh |
| `j/k`, arrows, PgUp/PgDn | Move in the focused pane; event selection updates the inspector immediately |
| Tab / Shift+Tab | Visible controls and panes |
| Enter / Esc | Open selected topic/group or event payload modal / return without losing selection |
| `s` | Seek editor, with explicit partitions and position |
| `F` | Deliberately jump to latest and follow from now |
| Space / `c` | Stop fetching / resume unread delivery positions |
| `/`, `n` | Filter topic names, or literal find/next in loaded event keys/values |
| `v` / `y` | JSON/raw/hex mode / copy full selected field |
| `h/l`, `[` / `]` | Horizontal content scroll / previous-next 256 KiB byte window |
| `q`, Ctrl+C | Quit and close clients |

Seek grammar (time must have an explicit timezone):

```text
recent all
earliest 0,2
offset 1 6408
time all 2026-10-30T12:00:00Z
```

At most 32 partitions per read. Topics show observed leaders, replicas and ISR
in the wide partition pane. Events show an arrival-order interleave, **not** a
global chronological order. Wide terminals show navigation/list/inspector; narrow
terminals use the payload modal for inspection. Minimum size is 60 columns × 16 rows.

**Compact** is the default one-line event layout. Choose **Comfortable** from the
density control or Actions for bordered event cards: partition/offset/time on the
left, key and payload preview on the right. Comfortable replaces the topic sidebar
with a header topic switcher. Changing density keeps the selected event and read position.
Selection follows the keyboard
or a click, never mouse hover. Enter opens a floating payload modal; Esc returns to
the same event. Tab selects Value, Key or Headers; j/k scrolls the payload or moves
through ordered headers. Valid JSON is syntax-highlighted in previews and inspection;
Compact rows use a uniform blue highlight; selected Comfortable cards use a cyan
outline and retain JSON colours.

Inspection preserves null versus empty, tombstones, invalid UTF-8, and ordered
duplicate headers. Terminal controls and bidi controls are escaped, never run.
JSON formatting is bounded; raw and hex expose undecoded data. No Schema Registry,
Avro or Protobuf decoding is implied. Find searches only loaded keys/values,
not broker history or headers. No event data or cursor is persisted.

Native clipboard tools are tried explicitly on copy. OSC52 is opt-in via
`--trust-osc52`; sending it does not verify receipt. Null fields cannot be copied
as bytes; hex mode copies hex. Native terminal selection remains available with
mouse off. Appearance changes are session-local until **Save appearance
preferences** in Actions; only sidebar/inspector/ASCII/density preferences are saved to
the OS config directory under `streams/settings.json` or an explicit `--settings`.

## Bounds and honest positions

- Historical reads: at most **500 records / 4 MiB / 5 seconds**, with captured
  last-stable-offset boundaries. Recent means a recent offset span, not a
  promised number of events.
- Follow batches: **100 records / 1 MiB / 500 ms**, with 250 ms scheduling ticks.
  Pause cancels work and closes the record client. Resume is not a latest jump.
- Retained window: **2,000 records / 16 MiB**. Eviction is reported; the app pauses
  before evicting the selected record. Positions advance only for accepted data.
- Fetches: 4 MiB response target, 1 MiB partition target, 16 MiB wire ceiling,
  8 MiB decompression ceiling. Broker batch exceptions mean these are **not a
  strict process-RSS guarantee**. Oversized data produces a visible limit state.
- Metadata/group lists cap at 10,000; responses can still require broker/client
  allocations before application caps apply.

Only committed transactional records are shown. Open transactions can delay
visibility. Offset gaps are normal with compaction, transactions and control
records; offset distances are not event counts. The public client does not
expose a reliable next-scanned position through every invisible gap. Consequently
a history read may honestly finish **deadline/incomplete** after delivering all
visible records. Continue retains conservative positions rather than inventing
progress or skipping unseen data. Expired/truncated positions require an explicit
seek. Topic replacement is checked where the broker provides usable topic IDs.

Group lag uses independently observed raw commits and high watermarks, not the
transactional last-stable offset. Missing commits, expired/inconsistent positions,
authorization errors and partial totals remain distinct. It is neither an atomic
snapshot nor a count of pending business events. There is no monitoring daemon.

## Real local testing

The opt-in [lab](lab/README.md) provides pinned **Redpanda v26.2.5** as the primary
broker (verified TLS/SCRAM-SHA-256 and read-only ACLs), plus **Apache Kafka 4.3.2**
for loopback plaintext protocol compatibility. It uses a private local Docker
daemon in an Amp orb, fictional events, transactions and separate seeder credentials.
Builds/tests never install or start brokers automatically.

```sh
make -C apps/streams lab-install lab-up lab-seed lab-check
./apps/streams/bin/streams --config apps/streams/.lab/connections.json --connection redpanda-viewer
```

Add `--broker kafka` to the Python lab commands for Kafka; see the lab README.
Down preserves data. Never expose broker/admin endpoints through a public portal.

```sh
make -C apps/streams check
make -C apps/streams test-terminal
STREAMS_SEEDED_LAB=1 STREAMS_KAFKA_TEST_CONFIG="$PWD/apps/streams/.lab/connections.json" \
STREAMS_KAFKA_TEST_CONNECTION=redpanda-viewer \
STREAMS_KAFKA_TEST_TOPIC=orders.created STREAMS_KAFKA_TEST_GROUP=orders-reader \
GOWORK=off go -C apps/streams test -race ./internal/kafka -run 'TestRealBroker|TestSeeded' -v
```

The terminal harness needs the app-local Python environment prepared by
`.agents/setup`. Real-broker tests otherwise skip explicitly. See
[validation](../../docs/plans/streams/05-validation.md) for measured results and
remaining acceptance limits. Configuring mTLS/PLAIN/SCRAM-SHA-512 is supported but
those handshakes have not all been exercised against the live lab. Neither Azure
Event Hubs nor arbitrary managed Kafka products are certified by these checks.

Built with Bubble Tea, Lip Gloss, shared `terminalui`, and franz-go/kadm. No MCP
server is included.
