# Explicit local broker lab

This is a **real broker lab**, not fixture mode. Nothing here is invoked by an
ordinary build or test. It requires Linux, sudo without a password, Docker CLI
and `dockerd`, OpenSSL, Python 3, Go 1.27.3, and the Amp orb service supervisor.
No host Java installation, cloud account, production cluster or remote Docker
daemon is used. It was exercised in the 8 GiB Linux orb on 2026-10-30.

## Commands

From the repository root:

```sh
python3 apps/streams/scripts/lab.py install
python3 apps/streams/scripts/lab.py up
python3 apps/streams/scripts/lab.py seed
python3 apps/streams/scripts/lab.py check
python3 apps/streams/scripts/lab.py down
```

These are the explicit lab-install/up/seed/check/down entrypoints. Add
`--broker kafka` to **each** command for the Apache compatibility broker;
Redpanda is the default. Both may run together. `install` only downloads pinned
images and creates private local credentials/configuration, not sample data.
`up` starts the selected broker; readiness means TCP listening, while `check`
proves actual authenticated protocol reads and content. `seed` creates the
disposable topics/group exactly once and starts an explicitly uncommitted
transaction holder. Repeat seed preserves the original records and group offsets.
After down, use **up, seed, check**: seed restarts the transaction holder without
reproducing base records. Down is idempotent and preserves **all** broker data,
credentials, containers and downloaded images. It deliberately leaves the private
Docker daemon running, so it cannot inadvertently stop the other lab broker.

For an explicit destructive reset of only this selected lab's seeded topics and
group (not unknown topics, files or volumes):

```sh
python3 apps/streams/scripts/lab.py reset --confirm-owned-reset
python3 apps/streams/scripts/lab.py seed
```

Reset requires the completed-seed ownership marker, stops the owned transaction
holder, deletes `orders.created`, `orders.failed`, `private.denied` and group
`orders-reader`, waits for observed topic deletion, then removes the seed marker.
There is no implicit reset or recursive data-directory removal. Partial initial
seeding is refused rather than silently overwritten; investigate it manually.

## Runtime, ownership and local endpoints

The orb's installed Docker daemon works with its `vfs` storage driver, host
networking and no bridge, iptables or forwarding changes. This avoids nesting
overlay networking/storage and does not modify the global Docker service or its
socket. All persistent state lives in **`apps/streams/.lab/`**, mode 0700, with a
checked `owner` marker. Its own `.gitignore` excludes everything, including itself.
Do not force-add it. Credentials, private keys, env/config files are mode 0600.
No password is printed or placed in a command-line argument.

| Service | Bound/advertised endpoints | Security |
| --- | --- | --- |
| `streams-lab-docker` | private `.lab/docker.sock` Unix socket | sudo; `.lab/docker` data root |
| `streams-lab-redpanda` | `127.0.0.1:19092` Kafka, `:19644` admin, `:13345` internal RPC | Kafka TLS + SCRAM-SHA-256 |
| `streams-lab-redpanda-open` | none | separate lab-only admin transaction producer |
| `streams-lab-kafka` | `127.0.0.1:29092` Kafka, `:29093` KRaft controller | **explicit local plaintext, no ACL safety** |
| `streams-lab-kafka-open` | none | separate transaction producer |

Every long-running process is started with `amp orb service start`, including the
private daemon and attached Docker clients. Containers have no restart policy or
public port publishing. Services are never detached with shell `&`. Inspect them
with `amp orb service list`, `status NAME`, or `logs NAME`. Do not expose Kafka,
the Docker socket or the admin API as a portal. Redpanda telemetry is disabled.
The loopback admin API is lab-only and is not protected against other orb users.
This is not a multi-tenant security boundary; containers run as root to keep
private bind-mounted lab state writable without changing host ownership.

Commands reject Docker environment overrides and active remote contexts, use an
explicit private Unix socket, verify the daemon data root before Docker actions,
and check container ownership labels plus immutable image pins before reuse or
stop. Unknown existing app state, service names, populated data without its owned
container, symlinks and port collisions are refused. Do not repoint this script at
another broker. Seeder endpoints are constants, not user-provided broker URLs.

## Viewer configuration

Use **`apps/streams/.lab/connections.json`**. The schema matches the backend
contract: `connections` with `name`, `brokers`, `tls`, `local_plaintext`, `ca_file`,
`sasl`, `username`, `password_file`; omitted optional client cert/key fields are
empty. Secret/certificate paths are relative to the config's directory.

* `redpanda-viewer`: verified local CA, TLS, SCRAM-SHA-256; broker-enforced
  read/describe on the two order topics and describe on `orders-reader` only.
* `kafka-local`: loopback plaintext with `local_plaintext: true`, no authentication.
  This proves protocol compatibility, **not** broker-enforced read-only security.

`admin.json` is separate and is **never** the viewer config. The independent
seeder reads `.lab/admin.password`; the app should only use the viewer profile.
Do not print or publish either credential file. Installation does not rotate
existing credentials or overwrite broker data.

## Deterministic data and independent verification

`lab/seed/main.go` is an independent executable using franz-go v1.22.2 and kadm
v1.19.0. It does not import app/backend packages and is never imported by the app.
The Python entrypoint builds with `GOWORK=off` and a **private alternate modfile**
`.lab/tools.mod`; app `go.mod`/`go.sum` are not edited. Go's normal module checksum
verification remains enabled.

Both `orders.created` and `orders.failed` have three partitions. Base record
counts are **3 / 5 / 7**, with offset zero in each partition, fictional JSON keys
`order-P-I`, timestamps starting at **2026-10-01T00:00:00Z + P×100 + I seconds**,
and ordered headers `trace:first`, `trace:second`, `nullable:null`, `empty:empty`.
Within each partition, index modulo 5 selects JSON, null key/value, non-null empty
key/value, binary key/value, and hostile terminal-control text. Topic retention
is explicitly disabled (`retention.ms=-1`) so these fixed historical timestamps
do not cause immediate expiration under the broker's ordinary retention policy.

`orders.created` additionally has a committed transaction in partition 0 and an
aborted transaction in partition 1; a supervised producer leaves a transaction
open in partition 2. The configured transaction timeout is 24 hours. Restarting
its holder fences/aborts the previous open transaction and opens another; thus
transaction/control offsets and HWM/LSO may move, while base data does not.
After 24 hours restart the holder explicitly (down/up/seed). It is intentionally
not an endless load generator. Group `orders-reader` commits are **1 / 2 / 4**
for each order topic. `private.denied` provides a negative permission sample.

`check` independently assigns partitions **without joining a group**, validates
all 31 committed records' raw bytes, null/empty distinctions, ordered duplicate
headers and timestamps, compares group commits before/after, and observes
LSO < HWM on the open transaction. A separate read-uncommitted control finds the
actual aborted/open records, proving they exist rather than merely being absent
from the committed view. On Redpanda it also proves denied topic writes, topic
creation, private-topic describe and group commits, plus rejected invalid SASL
credentials and an untrusted CA. These negative operations are isolated lab tests,
not behavior the viewer performs. Checks have an overall 45-second deadline.

Offline safety/fixture checks, after seed has created the private modfile:

```sh
python3 -B -m unittest discover -s apps/streams/scripts -p lab_test.py -v
# From apps/streams:
GOWORK=off go test -race -mod=readonly -modfile .lab/tools.mod ./lab/seed
GOWORK=off go vet -mod=readonly -modfile .lab/tools.mod ./lab/seed
```

## Verified artifact pins

Registry manifests were retrieved over HTTPS on 2026-10-30. The response
`Docker-Content-Digest` matched SHA-256 of each downloaded manifest byte-for-byte;
Docker then verified downloaded layers. The runtime uses the immutable index
digest, not a mutable version tag or `latest`.

| Release/tag resolved | Immutable index digest | linux/amd64 manifest |
| --- | --- | --- |
| `docker.redpanda.com/redpandadata/redpanda:v26.2.5` | `sha256:badca2d958f81da675c3055a9ecb6ebb590325582bcf16936148ffd65e96e920` | `sha256:298734baa319139e01b641d2f463bed7d00151858ae9d7facbf4558ef9ea4b52` |
| `docker.io/apache/kafka:4.3.2` | `sha256:852a0d49fc4e656b022e7f23785411802c9a088c78a7c5e39f33969de9c227a7` | `sha256:8f325fc00143fed78eabe00b033a86747078bb06edc21345d06651f77ae44f30` |

Executed version probes returned Redpanda **v26.2.5**, revision
`06e1aca30b726c8de9dc9db5c2b83b4da8edf6d0`, and Kafka **4.3.2** via
`/opt/kafka/bin/kafka-topics.sh --version`. No native Java install was necessary.

## Acceptance limits

Both real runtimes and independent transactional/raw/group checks pass in this
orb. The Redpanda TLS/SCRAM/read-only profile is live. Apache TLS/SASL/ACLs, mTLS,
PLAIN, SCRAM-SHA-512, compaction/retention races, oversize limits and the viewer UI
are **not** established by this lab check. App integration must separately prove
its own cursor/seek/cancellation/pause semantics and group-offset nonmutation.
This is not a production-hardening or universal broker-compatibility claim.
