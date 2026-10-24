# Live startup and saved connections

Start with `go run ./cmd/lazyftp`: a centered **Open a connection** chooser shows
bookmarks, existing remotes and **New connection**. It discovers **names only**
using `rclone listremotes`, without dumping configuration or testing
authentication. No remote is accessed until Connect, or an explicit read-only
test in the new-connection wizard. Esc opens local browsing; from the file
workspace, **Connect a remote** reopens the chooser.

The picker uses the configured theme (Midnight by default): arrows select a
connection, Tab/Shift+Tab move between controls, and Enter selects or activates.
Selecting an existing connection opens its details: remote root, local folder,
bookmark name, **Connect** and **Save bookmark**. Esc returns from details to the
chooser, then to local files. Click rows and controls; the wheel scrolls
connections. Type to append in fields, Backspace to remove a character, Ctrl+U
to replace the whole field. Root and local folder are editable paths, not
directory-browser dialogs. The local folder must already exist.
The supported picker size is at least 60 columns × 21 rows.

## CLI compatibility and precedence

```sh
# Existing explicit startup: no picker, no bookmark reads/writes or discovery
go run ./cmd/lazyftp --live --left ./downloads --right work-ftp:/incoming

# Any mix of existing rclone backends, not protocol-specific flags
go run ./cmd/lazyftp --live --right work-ftp:/incoming \
  --remote work-blob:/container/assets --remote work-s3:/bucket/prefix

# Connection chooser with a chosen config and separate bookmarks
go run ./cmd/lazyftp --live --rclone-config /private/rclone.conf \
  --connections /private/bookmarks.json

# Explicit fixture-only workspace; never accesses actual files or remotes
go run ./cmd/lazyftp --demo
```

- `--right remote:path` bypasses the picker. It is not overridden by bookmarks.
- Repeatable `--remote remote:path` adds navigation roots after the initial right
  location. `--blob remote:path` remains a compatible additional root (before
  `--remote` roots); despite its name it is not restricted to Azure.
- Without `--right`, startup opens the chooser and stays disconnected until
  Connect. Saved bookmarks appear first, then sorted discovered remote names.
  A bookmark restores its root and local folder. An explicitly supplied
  `--left` overrides that local default; it can still be edited in the picker.
- `--rclone-config` is passed to discovery and live storage. Otherwise rclone's
  normal environment/config-file rules apply, including `RCLONE_CONFIG` and
  environment-defined remotes. No app-specific credential precedence exists.
- `--config` remains theme/icons JSON only. Existing precedence is unchanged:
  `--theme` overrides its preset, JSON colors override preset roles, and explicit
  `--icons` overrides the JSON icon preference.
- `--demo` and `--live` together are rejected. `--live` is now optional for
  real mode; `--demo` is the only fixture-mode switch.

## Create a connection in-app

Choose **New connection** in the picker, select a storage type, then enter only
the relevant fields. FTP defaults to explicit FTPS; choosing plain FTP displays
an unencrypted-transport warning. Azure offers an existing Azure CLI session,
managed identity, SAS URL or account key. CLI mode requires `az` and a prior
`az login`; there is no embedded browser authentication. S3 supports an existing
AWS profile/environment or explicit keys, plus a custom endpoint for compatible
services. Cloud credentials still need appropriate provider permissions.

**Test connection** only lists the chosen root. Successful listing does not
verify write access or prove that an object-store prefix already exists: an
unused prefix inside an accessible bucket/container can list successfully as
empty. Editing details invalidates verification. Canceling or closing a test
cancels the process, and stale results cannot connect the app.

By default **Connect for this session** uses a temporary private 0600 rclone
config, removed on normal exit. It contains a snapshot of existing remotes plus
the proposed connection, so existing navigation roots keep working. A crash can
leave a `lazyftp-session-*.conf` file in the OS temporary directory; treat it as
sensitive and remove it after the process has stopped.

Only the unchecked **Remember connection and credentials** option permits
**Save and connect** to append the new remote to the selected rclone config.
Existing names are rejected, not replaced. The config is a private file, **not a
keychain and not encrypted at rest**; FTP password obscuring is reversible.
Encrypted configs and symlink config paths are not modified by this wizard;
choose a dedicated ordinary config file or manage those remotes with rclone.
Cooperating saves use an exclusive sidecar lock and atomic replacement; an
external writer that ignores the lock can still race the final rename.

Saving a later connection never persists earlier session-only connections.
Bookmarks remain separate and contain no credentials. New interactive
connections never create or broaden [MCP grants](mcp.md).

## Explicit, non-secret bookmarks

Enter a bookmark name and activate **Save bookmark**. This saves but does not
connect. Connect does not save. Saving the same name explicitly replaces it and
retains other bookmarks. Open the picker again to reuse a saved bookmark. There
is no automatic recent-history file. Credential setup is the separate wizard
above, not the bookmark fields.

Default storage is `lazyftp/connections.json` inside the OS user configuration
directory (`$XDG_CONFIG_HOME`, otherwise `~/.config` on Linux). `--connections`
chooses another path. The file contains only these four fields:

```json
[
  {
    "name": "Incoming files",
    "remote": "work-ftp",
    "root": "/incoming",
    "local": "/home/me/downloads"
  }
]
```

Names and paths are potentially private metadata: keep the file out of source
control. New directories/files use 0700/0600 permissions on Unix; saves replace
the file with a same-directory rename. Invalid JSON is reported, not overwritten.
Concurrent writers are not merged; use one picker per bookmark file. Edit the
JSON while the app is closed to remove or rename bookmarks. Bookmarks refer to
remote names in the **currently selected rclone config**, not a stored config
path; deleting/renaming a remote or switching configs can invalidate a bookmark.
Discovery errors leave saved bookmarks available, but successful discovery or
bookmark loading is not proof the target exists or that credentials work.

**Never paste passwords, access keys, connection strings, SAS/signed URLs or
inline backend definitions into bookmark fields or CLI locations.** Only use
named rclone remotes and ordinary paths. The app cannot recognize every secret
someone puts in a path. Backend stderr is not displayed by discovery, since it
can contain credential-bearing URLs. Unlock encrypted rclone configurations
using rclone's supported environment/password-command mechanism before startup;
the app does not prompt for an rclone config password.

## Configure rclone safely

Run `rclone config` in a private terminal, or `rclone --config /private/rclone.conf
config` for a separate file. Use meaningful, non-secret names such as `work-ftp`,
`work-blob`, `work-s3`. Follow the installed rclone version's prompts; do not put
credential arguments into shell history, scripts, screenshots, or this repository.
Protect the rclone config and credential files. rclone's password **obscuring is
reversible, not encryption**; use rclone config encryption and secure unlock
handling where appropriate. Prefer short-lived, narrowly scoped credentials.
Keep TLS verification enabled. Authentication options vary by rclone version.

### FTP / FTPS

Choose backend `ftp`, then the host, user and password in rclone. Prefer explicit
FTPS (`explicit_tls`, normally port 21) or implicit FTPS (`tls`, normally 990) when
the server supports it; choose one, not both. Plain FTP sends credentials and data
without TLS and is unsuitable across untrusted networks. Do not bypass certificate
verification to make a connection work. SFTP is a different protocol/backend
(`sftp`), not a TLS option for FTP. Set the picker root to `/incoming` or another
authorized directory. See [rclone FTP documentation](https://rclone.org/ftp/).

### Azure Blob Storage

Choose `azureblob`. Prefer an appropriate identity-based flow: `env_auth` can use
environment service-principal credentials, managed identity in Azure, or Azure
CLI credentials. A version supporting `use_az` can select Azure CLI specifically;
do not enable it alongside `env_auth`. Configure the account and grant only the
required Blob data permissions at the appropriate container scope (read-only for
browsing, write only when needed). An ordinary Azure management-plane role alone
may not grant Blob data access.

If identity-based access is unavailable, rclone supports account/shared key,
service-principal credentials, or limited/expiring SAS URLs. Keep these in rclone's
private configuration or supported credential sources, **never in a bookmark
root**. For container-level SAS, set the actual SAS URL in rclone's `sas_url`
setting; use only `/container/prefix` in lazyftp. Account keys are broad access;
do not use them just for convenience when narrower access is available. See
[rclone Azure authentication](https://rclone.org/azureblob/#authentication).

### S3 / S3-compatible storage

Choose `s3`, the correct provider and region, plus the provider's HTTPS endpoint
for non-AWS services. Prefer `env_auth = true` with temporary credentials, a
shared credential profile, or a workload/instance role supported by rclone.
`AWS_PROFILE` or rclone's `profile` selects a named profile; keep actual keys and
session tokens in the supported credential source, not lazyftp arguments.
Static access keys, if necessary, belong in protected rclone configuration.
Restrict IAM/policies to the needed bucket/prefix and operations. A scoped user
may lack account-wide bucket listing: start at `/bucket/prefix` instead of `/`.
See [rclone S3 authentication](https://rclone.org/s3/#authentication).

## Validation boundaries

Fast tests require no network, rclone binary, Docker or accounts:
`go test ./cmd/lazyftp ./internal/connections`. They cover remote parsing,
discovery's executable contract and stderr suppression, explicit save/reuse/
replacement, CLI local override, keyboard/mouse selection and bounded rendering.
Discovery does not list remote files. Connecting hands the chosen paths to the
existing live browser; single-file copy still requires its existing confirmation.
No production connection is configured or authorized for this slice. **Real FTP,
Azure and AWS account authentication/permission checks remain pending**; local
fixtures or emulator mechanics do not establish production-account validation.
