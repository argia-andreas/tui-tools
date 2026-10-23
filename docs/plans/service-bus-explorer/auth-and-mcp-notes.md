# Authentication exploration — 2026-10-23

Research and proposed design only; no implementation, credentials or Azure access. Reviewed published product documentation, not ViewBus source code or a running app.

## What ViewBus publishes

ViewBus is made by Håkon Lia, an individual developer in Norway. Its privacy page describes OAuth sign-in through the default browser and direct Microsoft tenant enumeration so users can select an active tenant. Its homepage says Azure sign-in discovers namespaces visible to that account, and connection strings are also supported (including namespaces outside the tenant). The architecture page says tokens and connection strings use macOS Keychain / Windows Credential Manager, not its local index or logs. These are vendor statements, not independently audited behavior. The pages do not establish the exact OAuth grant, broker integration or a guarantee of silent SSO.

Sources:
- https://viewbus.app/
- https://viewbus.app/privacy/
- https://viewbus.app/why-viewbus/

## Suggested terminal-app model

1. Default: Sign in with Microsoft (Entra ID). Proposed system-browser authorization-code flow with PKCE for a public native client; no embedded client secret. Existing Microsoft browser sessions may simplify sign-in, but MFA, consent, tenant policy and Conditional Access still apply. Show account and tenant explicitly, support account switching, and assess device-code fallback for headless terminals subject to tenant policy. Registration/consent requirements need validation during implementation.
2. Optional explicit developer profile: use an existing Azure CLI login, if supported by the selected SDK. Avoid silent credential-chain fallback that changes identity unexpectedly.
3. Fallback: add a namespace/entity using a SAS connection string. Secret input masked; OS credential vault storage when available, session-only otherwise. Never silently fall back to plaintext, logs or exported dashboard configuration. Avoid broad RootManageSharedAccessKey as the recommended setup. SAS-disabled namespaces require Entra ID.
4. Namespace discovery is distinct from message authorization. Separate management/discovery errors from data access errors; allow a known namespace to be added manually when discovery is unavailable. Do not promise all queue counts or bodies solely because sign-in succeeded.
5. Use narrowly scoped roles. Data Receiver for receiving-side access, Data Sender for send-side access; resend plus cleanup needs receive-side permissions on the source and send-side permissions on the destination. Do not require Data Owner as the default. Azure Receive permissions are not a peek-only security boundary: default read-only app mode and confirmations are safeguards, not a stronger cloud role.
6. Counts/dashboard configuration need no mutation prompts; message actions must remain scoped and reviewed. Credentials stay on the user's machine, requests go directly to Microsoft/Service Bus; no app-operated authentication relay is proposed.

Microsoft sources:
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/service-bus-authentication-and-authorization
- https://learn.microsoft.com/en-us/azure/service-bus-messaging/authenticate-application

Open decisions: SDK/runtime, OS vault support including Linux, app registration audience and consent, token-cache implementation, exact permissions for discovery/count APIs, and whether to include Azure CLI/device-code profiles in the initial version.

## Future agent MCP — Andreas's requirement

Note for future planning only; no MCP implementation or access authorized here.

- Locked down by default: no agent resource access until the user explicitly allowlists it.
- Independently allowlist read operations for specific queues or topic subscriptions, including their DLQs. Agent should be able to inspect dead-letter messages for a specific topic subscription when permitted. Topic subscriptions, not topics themselves, own these DLQs.
- Independently allowlist DLQ resubmission; read permission never implies resend permission. Treat source DLQ, destination queue/topic and operation as explicit policy scope, identified by full resource identity rather than display name alone.
- Dashboard inclusion and interactive-app access must not automatically grant MCP access. Azure credentials/roles remain necessary but do not override the application's MCP restrictions. Enforce policy server-side for every tool request, not just by hiding tools.
- Resubmission should remain deny-by-default and is an optional capability to decide later. Sending a copy and removing the DLQ original are separate effects; resend permission must not silently grant deletion/cleanup. Other mutations remain denied unless separately approved in future requirements.
- Message bodies/properties are untrusted data, not agent instructions. Reading via an agent may transmit content to that agent's model provider; make that boundary clear when enabling access. Never expose credentials through MCP.

Transport, policy UI, audit records, limits and confirmation mechanics remain design decisions. No broad wildcard or inherited namespace-wide grants are assumed.
