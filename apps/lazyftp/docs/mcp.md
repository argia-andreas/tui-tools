# LazyFTP / Tandem MCP server

This is the FTP project's own MCP server, **not** the Service Bus explorer.
It exposes three tools over newline-delimited JSON-RPC on **stdio only**:

| Tool | Required arguments | Result |
| --- | --- | --- |
| `list_files` | `remote`, `path` | `entries`: immediate children with `Name`, `IsDir`, `Size`, `Modified` |
| `upload_file` | `remote`, `path`, `local_path` | `copied: true` after local → remote copy |
| `download_file` | `remote`, `path`, `local_path` | `copied: true` after remote → local copy |

Paths identify full filenames for transfers, directories for listings. Listing
is not recursive. There is no local-listing, remote-to-remote, local-to-local,
delete, move, execute, credential, or permission-management tool. Transfers use
the existing `storage.Rclone` Browser/Transferer, not a separate implementation.
Errors are MCP tool errors (`isError: true`); backend error details are redacted.
The tool catalog remains visible even when all operations are denied.

## Build and connect

Requires the repository's Go version and rclone on the server's trusted `PATH`.
Run build, verification and lab commands below from `apps/lazyftp`.

```sh
go build -o /tmp/lazyftp-mcp ./cmd/lazyftp-mcp
```

Example MCP client configuration (replace paths with your own absolute paths):

```json
{
  "mcpServers": {
    "lazyftp": {
      "command": "/tmp/lazyftp-mcp",
      "args": [
        "--policy", "/home/operator/.config/lazyftp/mcp-policy.json",
        "--rclone-config", "/home/operator/.config/rclone/mcp.conf"
      ]
    }
  }
}
```

The client launches a subprocess, performs MCP initialization, and closes stdin
to shut it down. No port, HTTP service, login, or interactive UI is involved.
Stdout contains protocol messages only; startup diagnostics go to stderr.
The official Go MCP SDK handles initialization, schema validation, notifications
and cancellation. The stdio tests exercise the 2025-11-25 protocol lifecycle.

## Explicit permission policy

**No policy flag means deny everything.** `{}`, `{"grants":[]}`, and
`{"grants":null}` also deny everything. Supplying an unreadable, malformed or
invalid policy stops startup, rather than silently falling back to permissions.
Unknown fields, trailing JSON and files larger than 1 MiB are rejected.
There are no wildcard or inherited grants, and rclone credentials, client roots,
environment variables and interactive-app state never create MCP permissions.

Create dedicated staging directories first. A useful asymmetric policy is:

```json
{
  "grants": [
    {
      "operation": "list",
      "remote": "team-ftp",
      "root": "/incoming"
    },
    {
      "operation": "upload",
      "remote": "team-ftp",
      "root": "/incoming/agent-uploads",
      "local_roots": ["/home/operator/mcp-staging/outbox"]
    },
    {
      "operation": "download",
      "remote": "team-ftp",
      "root": "/incoming/reports",
      "local_roots": ["/home/operator/mcp-staging/inbox"]
    }
  ]
}
```

Each grant authorizes exactly one operation on one **case-sensitive, exact**
named remote and that remote root or its descendants. Upload grants permit
local reads; download grants permit local writes. List grants must not contain
`local_roots`; transfer grants must have at least one existing local directory.
One complete grant must match: a remote root from one grant cannot be combined
with local roots from another. Multiple complete grants are additive. Listing
permission does not authorize transfers, and transfers do not authorize the
listing tool (the storage adapter may internally list to perform a transfer).
Changes to policy require restarting the MCP server; it snapshots grants at
startup and offers no tool that broadens them.

With any grants, an explicit existing absolute `--rclone-config` file is required.
Its path must have no symlink components and must be outside **all** granted
local roots, including upload roots, so it cannot be uploaded as an ordinary
file. Use a minimal dedicated config, not your personal all-remotes config.
Neither config contents nor backend stderr are returned to the agent. Keep the
policy and config operator-owned, outside directories agents can write.

Tool calls for the example policy:

```json
{"name":"list_files","arguments":{"remote":"team-ftp","path":"/incoming/reports"}}
{"name":"upload_file","arguments":{"remote":"team-ftp","path":"/incoming/agent-uploads/new.txt","local_path":"/home/operator/mcp-staging/outbox/new.txt"}}
{"name":"download_file","arguments":{"remote":"team-ftp","path":"/incoming/reports/daily.txt","local_path":"/home/operator/mcp-staging/inbox/daily.txt"}}
```

## Path and overwrite rules

- Remote names must match `[A-Za-z0-9][A-Za-z0-9_-]{1,63}` (2–64 characters).
  Use a multi-character remote name, not a drive letter. Connection strings,
  inline backends, colons, options and arbitrary remote selection are rejected.
- All paths and policy roots must be canonical absolute paths. Use `/incoming`
  for FTP and `/bucket/prefix` for Blob/S3. No relative paths, `.`/`..`
  components, repeated slashes, trailing slash except `/`, backslashes, colons
  or control characters. Unicode and spaces are supported; paths are not URL
  decoded. This intentionally excludes some otherwise legal filenames.
- Root matching uses a slash boundary: `/incoming-safe` is not under
  `/incoming`. Remote `/` grants the entire remote; avoid it where possible.
- Every local component, including ancestors above the grant root, is checked
  using `Lstat`. Symlink roots, ancestors and final files are refused, even if
  they resolve inside the root. Special files and directories cannot transfer.
  Upload sources must exist. Download parents must already exist; only the final
  filename may be absent. Local roots themselves are not transfer targets.
- **No overwrite permission exists in this version.** `overwrite: true` is
  denied before backend access; omit it or use false. Existing destinations are
  refused by the storage adapter, including equal-content files. Upload and
  download requests from this server are serialized. Remote parent directories
  may be created by rclone within the granted prefix.
- Permission and path checks precede all backend calls; a denied call cannot
  probe remote existence or initiate a transfer. Successful transfer internals
  may enumerate remote ancestor directories using the existing adapter's Stat;
  only the tool's permitted listing or copy result is exposed.

## Security boundary and limitations

This is an authorization boundary for an **untrusted MCP client**, not a sandbox
against hostile processes running as the server's OS user or a hostile backend.
The operator must control the executable, PATH, launch environment (including
rclone/backend variables), policy, config, and the granted filesystem trees.
Named remotes must resolve to the intended network services, not rclone local,
alias, union, or other configurations that redirect paths outside that service.
Remote permissions/chroots must prevent server-side symlink or alias escapes;
the Browser/Transferer contract cannot inspect these reliably.

Local checks are snapshots: **filesystem race-proof confinement is not
implemented**. Rclone later opens filenames by path. Another local process
could replace a checked directory with a symlink, replace files, introduce
hardlinks to sensitive files, or mount a different filesystem. Use dedicated
staging trees with no sensitive contents/links/mounts that untrusted processes
cannot modify, and run under a dedicated least-privilege account. Do not grant
home/config/system directories. If the agent also has unrestricted shell access
as that account, MCP policy alone does not contain it. Stronger adversarial
local confinement needs an OS sandbox/mount namespace or a descriptor-based
transfer boundary, not more pathname checks.

No-clobber is also subject to backend races with external writers; in particular
FTP cannot provide atomic conditional creation through this adapter. A failed
or cancelled transfer may leave a partial destination. The server does not delete
it; clean up explicitly outside MCP. This is not a transactional transfer API.
Large listings are returned in full, and there are no byte quotas or rate limits;
grant small roots and use OS/backend resource limits where necessary. Filenames
are untrusted data and must never be treated as agent instructions.

## Verification

```sh
go test ./internal/mcp ./cmd/lazyftp-mcp
go test -race ./internal/mcp ./cmd/lazyftp-mcp
```

The public-tool tests cover absent grants, separate operation grants, disjoint
grant combinations, traversal, prefix boundaries, hostile remotes, outside local
roots, symlink components, directories, explicit overwrite denial, credential
error redaction, and policy snapshot isolation. Denied requests assert zero
backend calls. A built executable is tested through actual stdin/stdout for
initialize → initialized → ping → tools/list → tools/call → EOF shutdown, default
denial and fail-closed startup.

Optional local lab integration (no production services):

```sh
python3 scripts/lab.py up
python3 scripts/lab.py seed
go test -tags=integration ./cmd/lazyftp-mcp -run TestStdioRcloneLab -count=1 -v
```

This launches the real MCP executable, grants only random unique lab prefixes,
and verifies list/upload/download using rclone against local FTP, Azurite Blob,
and SeaweedFS S3. It byte-compares asymmetric 4233-byte binary/Unicode-space and
empty-file round trips, verifies overwrite refusal, rejects outside-prefix
requests, checks source preservation, and removes only its unique test prefixes.
This does not verify production authentication, TLS, cloud IAM or race-proof
confinement. Lab services remain loopback-only; see the [lab guide](../lab/README.md)
for lifecycle.
