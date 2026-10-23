# Local Service Bus development research

Research and bounded execution checked 2026-10-23. The emulator now runs in this orb and messaging checks passed. Runtime counts have a verified compatibility gap. This informs Gate 2; it is not an approved architecture or completed reusable lab implementation.

## Azurite is not Service Bus

Azurite emulates Azure Storage Blob, Queue and Table services. Azure Storage Queues are a different service from Azure Service Bus queues, topics and subscriptions. Azurite therefore cannot validate this explorer's Service Bus behavior.

## Microsoft Service Bus emulator

Microsoft supplies a separate Service Bus emulator with AMQP messaging and configurable queues, topics and subscriptions. Current documentation describes Linux containers on Linux/macOS/Windows, a two-container setup with a SQL dependency, minimum 2 GB RAM and 5 GB disk, and a static development connection string marked `UseDevelopmentEmulator=true`. No Azure subscription is required for local emulator messaging.

Important boundaries:

- No Microsoft Entra ID integration: browser sign-in/RBAC needs separate cloud validation.
- AMQP TCP only, not AMQP WebSockets; no partitioned entities or JMS.
- Entities and messages do not persist across emulator restarts: deterministic reseeding is necessary, not optional.
- One namespace, up to 50 queue/topic entities and 50 subscriptions per topic; 256 KB messages and 10 concurrent connections according to current quotas. Multi-namespace dashboard scenarios still need fixtures or a separately designed test arrangement.
- Administration is exposed on a separate HTTP endpoint (default port 5300), including health checks. Current docs say .NET natively supports emulator administration; other SDKs need documented workarounds for non-TLS/custom ports. Do not choose the stack solely on successful send/receive: discovery and runtime counts are central explorer behavior and must be spiked too.
- The official installer README and Learn page differ in SQL dependency naming (SQL Edge versus SQL Server Linux). Pin and inspect the actual Compose/images before documenting a reproducible setup. macOS support does not establish native Apple Silicon support for every dependency; test the selected image architecture and any emulation requirements.
- The emulator is development/test software under separate Microsoft license terms, not a production broker or complete cloud replacement. Current docs recommend sequential testing; avoid parallel integration jobs sharing mutable state.

## Observed orb state

The initial missing Docker socket was resolved by starting `dockerd` as supervised orb service `servicebus-docker` with `sudo`, using overlay2. A real `hello-world` container passed. Installed Docker Compose plugin and started the two-container emulator as supervised service `servicebus-lab`. Ports 5672/5300 bind loopback only; no public portal or cloud resources were created.

The live experiment configuration is outside the repository at `/home/user/.local/share/servicebus-explorer-lab/`. It is a capability spike, not the future portable developer command interface. Inspect through `amp orb service status servicebus-lab` / `logs servicebus-lab` and `sudo docker compose -f /home/user/.local/share/servicebus-explorer-lab/compose.yaml ps`. No secret values need to be printed.

Resource observations: x86_64, 3.8 GiB RAM, roughly 60 GiB disk initially free. After exercising the broker, Docker reported approximately 151 MiB for the emulator and 456 MiB for SQL; system available memory was about 2.7 GiB. No orb upgrade is needed for this tested workload. Larger/parallel suites have not been measured.

## Executed capability probe

Used a temporary Node probe with `@azure/service-bus` 7.10.0 and the official sample's isolated HTTP/custom-port administration override. This is not a recommendation to use private SDK overrides in production. A second probe used .NET SDK 10.0.402 and `Azure.Messaging.ServiceBus` 7.20.1 without custom administration overrides.

Verified against emulator 2.0.1 on x86_64:

- Health endpoint returned HTTP 200 and `Service is healthy`.
- Discovered configured queues and both topic subscriptions.
- Sent three known JSON messages and peeked their exact IDs/body data.
- Confirmed peek left those messages available, using independent bounded samples (not aggregate counters).
- Dead-lettered one, verified its ID/reason, and observed the expected two active / one DLQ sample.
- Sent its replay copy before completing the original DLQ message; observed three active / one DLQ before cleanup, three active / zero DLQ after cleanup.
- Completed Billing's original event, dead-lettered Shipping's original, then resent to the topic: both Billing and Shipping received the new copy. This independently demonstrates the agreed replay warning.
- Drained exactly three remaining active messages; repeated from a fresh emulator restart successfully.
- Left illustrative seed data in the running lab: Orders has two active and one dead-letter message; Empty is empty; Billing has one active event and Shipping one dead-letter event. These are seed expectations, not values from a trusted runtime count API.

Important failure: queue runtime response returned HTTP 200 but omitted `CountDetails` and `AccessedAt`. The JavaScript SDK rejected it with `PARSE_ERROR`. Adding .NET-style `enrich=True` and API version 2024-05 did not change the omission. The unmodified .NET administration client returned active=0/DLQ=0 after a real send, and again after that message was dead-lettered. Do not present these zeros as accurate. No authoritative count fallback was implemented. This limits emulator validation of live dashboard totals; use fixtures for presentation and explicit cloud checks for true runtime counts.

Explicit peek sequence positioning matters: repeated implicit peeks on a client can advance its shared management cursor. The probe used an explicit starting sequence for refresh assertions rather than treating an empty subsequent page as an empty queue.

## Image reproducibility and macOS boundary

Tested manifest-list digests:

- Emulator: `mcr.microsoft.com/azure-messaging/servicebus-emulator@sha256:5a96d893b245031740f7d46e0fe5ff282d24b78c4b7d761dd57590f3f010a9b3` (2.0.1).
- SQL: `mcr.microsoft.com/azure-sql-edge@sha256:902628a8be89e35dfb7895ca31d602974c7bafde4d583a0d0873844feb1c42cf` (the upstream `latest` multi-architecture manifest).

Manifest research found amd64 and arm64 emulator children. SQL's `latest` manifest selects a 2.0.0-equivalent amd64 image but an older 1.0.7-equivalent arm64 image; pinning SQL 2.0.0 would not provide a native ARM image. SQL Edge is retired; historical ARM image availability does not prove supported Apple Silicon execution. No macOS device was available to execute this test. Validate the developer's actual Mac before claiming parity.

## Proposed feedback layers

1. Deterministic fixture-backed UI/action tests runnable without containers, including 12-box/large-table dashboards, DLQ errors, stale counts and access failures.
2. Disposable emulator integration on a container-capable machine: health/readiness, seeded topology, send, peek, receive/settle, DLQ, replay and count/discovery checks. Development commands should include start/stop, seed/reset, send, drain and dead-letter, guarded against cloud endpoints.
3. Explicit opt-in real-Azure smoke checks for Entra/RBAC and differences the emulator cannot prove; no production writes implied.

Remaining: approve Gate 2, validate the selected Go SDK path, specify reusable lab tooling in Gate 3, and execute macOS checks on a Mac. Emulator messaging/discovery is proven here; accurate runtime counts, cloud auth and macOS execution are not. Do not add unapproved application implementation to conceal these gaps.

## Sources

- https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/overview-emulator
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/test-locally-with-service-bus-emulator
- https://github.com/Azure/azure-service-bus-emulator-installer
