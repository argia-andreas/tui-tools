# Maintainer documentation

User setup, connection requirements, and supported operations live in the app
READMEs: [Transfer](../apps/transfer/README.md), [Logs](../apps/logs/README.md),
[QE](../apps/queue/README.md), and [Streams](../apps/streams/README.md).

## Maintained architecture, safety, and verification

- [Transfer architecture](../apps/transfer/docs/architecture.md): pane-local
  connections, opening workflow, rename safety, and historical verification.
  See also [connections](../apps/transfer/docs/connections.md),
  [alpha limitations](../apps/transfer/docs/alpha.md), and
  [MCP permissions](../apps/transfer/docs/mcp.md).
- [QE architecture](../apps/queue/docs/architecture.md): environments and scoped
  reads, inspector/replay safety, shared UI integration, and verification limits.
- [Ordinary panel rendering](architecture/panel-rendering.md): shared renderer
  ownership, safe-input/presentation contract, app differences, and historical
  migration evidence.

Historical verification records describe what was exercised at that time, not
fresh test runs or proof of universal live-service compatibility. Retained
mockups are design studies, not runtime evidence. Ignored local screenshots and
recordings are not distributed with the repository.

## Plans and acceptance work retained separately

- [Shared terminal UI](plans/shared-terminal-ui/00-status.md): engineering slices
  completed, with human/native-terminal acceptance still outstanding.
- [Streams](plans/streams/00-status.md) and its
  [validation limits](plans/streams/05-validation.md): implementation complete,
  acceptance partially verified.
- [Logs](plans/log-viewer/00-status.md): unfinished and partly superseded scope,
  including context/follow and MCP work; reconcile status before implementing.
- [QE application plan](plans/service-bus-explorer/00-status.md): retains portable
  handoff/polish backlog alongside historical work.
- [QE MCP proposal](plans/service-bus-mcp/00-status.md): Product gate unapproved;
  no server implementation. Interactive permissions do not grant MCP access.

The completed pane-source, opening-screen, rename, DLQ-inspector, Service Bus
scopes/UI-integration, and panel-rendering execution plans have been consolidated
into the maintained guidance above. Their directories retain short pointers and
any existing visual studies. Other plans and visual studies have not been
retired by this cleanup. Historical names in retained plans, runtime identifiers,
and provenance are not instructions to rename stored data or grant permissions.
