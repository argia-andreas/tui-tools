# Product: Explicitly scoped Service Bus MCP access

Gate 1 proposal — **NOT APPROVED**. **No MCP server exists.** MCP was deferred from the approved Explorer UI scope and requires its own approval. This document proposes user-visible permissions and consequences only, not an implementation design.

## Problem

Andreas wants an agent to inspect specifically permitted Service Bus messages, especially a particular topic subscription's dead-letter queue, without giving that agent the interactive user's entire reach. If remediation is enabled later, the user must independently control which DLQ messages may be deleted and which DLQ sources may send copies to which queues.

## Success metric

Proposed acceptance target, not a measured result: inspect 1 explicitly allowed subscription DLQ end to end, with **0 unauthorized accesses or effects** across default-deny, same-name/different-resource, ungranted Active/DLQ, and independent read/delete/resend permission checks. The user can identify the exact source and any permitted destination before enabling access. No performance target is claimed at this gate.

## Announcement — proposed user experience

Let your agent inspect only the Service Bus resources you deliberately share. Allow reading a particular queue or topic subscription and explicitly choose whether its active messages, DLQ, or both are available. Optional remediation permissions are separate: deleting a DLQ original is not the same as sending a copy to an approved queue. Your dashboard and interactive sign-in never enable agent access automatically. Read-only launch is recommended while the consequences of acquiring messages for remediation remain under review.

## Product boundaries

- **Default deny:** no agent resource access until explicitly allowlisted. Every request must respect the grant; hiding an action is not authorization. Azure permissions remain necessary but never override the MCP restrictions.
- **Exact read scope:** identify the namespace and full queue identity, or namespace + topic + subscription identity, including the applicable connection/profile context. Active and DLQ access must be explicit, not inferred from one another. Display names, topic names alone and same-named entities elsewhere are not equivalent grants; no wildcard or namespace-wide inheritance is proposed.
- **Read means inspection:** permitted counts/metadata and peeked message bodies/properties only within that scope. It never means receiving, acquiring locks, settling or sending messages. Do not disclose unallowlisted resources through discovery.
- **Only possible write permissions:** separately allowlist deletion of a message from an exact source DLQ, and/or resend from an exact source DLQ to an explicitly scoped **QUEUE** destination. A source may be a queue DLQ or a specific topic subscription's DLQ; its identity and destination identity are both part of the resend grant. Read access alone authorizes neither write; write grants do not imply broader read access.
- **Resend is a copy, not a move:** it never implies deletion of the original. Any deletion, including cleanup after a confirmed resend, requires separate deletion permission and an explicit action. Unknown send outcomes must not be described as failure or invite automatic retry; sending and deleting cannot be promised as atomic or exactly-once.
- **No inherited agent access:** watchlist inclusion, Explorer settings, interactive credentials and broad Azure roles do not constitute MCP grants. Enabling sharing must make clear that message content may reach the agent's model provider; message text/properties are untrusted data, never instructions or permission changes. Credentials must never be exposed to the agent.
- **Excluded:** active-queue/subscription writes, bulk operations, topic resend, arbitrary destinations and general administration. A subscription-DLQ resend to an approved queue is not replay to its original topic. No FTP changes or coordination with the FTP thread.

## Supported-contract constraint: inspection is not acquisition

An arbitrary message selected by peek in a DLQ **cannot be locked by its sequence number under the supported contract**. Do not promise “delete/resend this peek-selected row.” Receiving the next DLQ message may return a different message and changes lock/delivery state; releasing or expiring that lock can affect delivery count. It cannot be hidden inside a read request, permission check or preview.

The Explorer UI's approved receive-next/review interaction does not authorize the same behavior for an agent. Before any MCP locking or writes, separately approve who may request acquisition, its exact DLQ scope, how the actual acquired identity is authorized/reviewed, and what happens on cancellation, expiry or uncertain outcomes. No receive-and-scan workaround or automatic replacement acquisition is proposed.

## Decisions required for approval

1. **Launch scope — essential ambiguity:** approve a read-only first release (recommended), leaving all acquisition/deletion/resend disabled until an explicit acquisition policy is approved; or defer launch until that policy is resolved? Read-only first delivers useful inspection without quietly importing the UI's lock-changing workflow.
2. **If remediation is required at launch:** what explicit acquisition authority and review are required before receive-next changes state, and how is consent tied to the actual received message rather than the peeked one? An allowed delete or resend alone must not silently authorize acquisition.
3. **Grant and action control:** who may enable/revoke exact resource grants, and is an additional human confirmation required for each acquisition, deletion and resend? These choices remain open, not implied by possessing interactive credentials.

## Screens

No UI proposed at this gate. Grant-management experience, transport, audit behavior, limits and technical contracts await later gates after product scope approval.

## Context

Read alongside the Explorer's `01-product.md`, `03-program-design.md` and `auth-and-mcp-notes.md`. Those documents provide context, not MCP approval. Their broader interactive topic-replay behavior does not expand this proposal's queue-only resend boundary.
