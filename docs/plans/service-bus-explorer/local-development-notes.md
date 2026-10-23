# Local Service Bus development research

Research checked 2026-10-23. No emulator deployment or broker test has been executed. This informs Gate 2; it is not an approved architecture.

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

`docker info` found Docker CLI 29.9.2 but failed because `/var/run/docker.sock` does not exist. Architecture is x86_64; no Podman executable was found. The emulator cannot currently be started through an existing Docker daemon here. This is not proof that a daemon cannot be provisioned; investigate orb/container capabilities before selecting an alternative. Never report fixtures as successful emulator validation.

## Proposed feedback layers

1. Deterministic fixture-backed UI/action tests runnable without containers, including 12-box/large-table dashboards, DLQ errors, stale counts and access failures.
2. Disposable emulator integration on a container-capable machine: health/readiness, seeded topology, send, peek, receive/settle, DLQ, replay and count/discovery checks. Development commands should include start/stop, seed/reset, send, drain and dead-letter, guarded against cloud endpoints.
3. Explicit opt-in real-Azure smoke checks for Entra/RBAC and differences the emulator cannot prove; no production writes implied.

Before Gate 2 approval: validate a viable orb integration environment, SDK count/discovery support, Apple Silicon setup, and emulator DLQ/replay behavior with a bounded capability spike. Do not add unapproved implementation merely to conceal these unknowns.

## Sources

- https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/overview-emulator
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/test-locally-with-service-bus-emulator
- https://github.com/Azure/azure-service-bus-emulator-installer
