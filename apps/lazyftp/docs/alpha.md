# LazyFTP alpha: local packages and limits

This is an experimental terminal application, not a production-ready storage
client. Package creation does not publish, deploy, sign or notarize anything.
Use disposable files and remotes first. An emulator or a successful cross-build
does not establish production authentication, cloud-service parity or native
runtime support on another platform.

## Build packages

From `apps/lazyftp` in a checkout with the Go toolchain required by its `go.mod`,
Bash, Git, `tar` and `sha256sum` (or macOS `shasum`):

```sh
bash scripts/package.sh 0.1.0-alpha.1
# Optional second argument: a new output directory, never an existing one.
bash scripts/package.sh 0.1.0-alpha.1 /tmp/lazyftp-alpha-check
```

The explicit build matrix is Linux amd64/arm64 and macOS amd64/arm64. Windows is
not packaged in this alpha. Cross-builds use `CGO_ENABLED=0` and `-trimpath`;
they are not native runtime tests. Each versioned `.tar.gz` contains `lazyftp`,
`lazyftp-mcp`, its default-deny policy guide as `MCP.md`, these instructions as
`ALPHA.md`, and `BUILD.txt` with the package label, target,
source revision/dirty status and toolchain. The version labels the archive; the
executable has no `--version` flag. Packages use the current working tree, so
commit intended changes first for traceability. A failed build can leave an
incomplete output directory; retry with a new directory.

Output defaults to ignored `bin/alpha/VERSION/`. `SHA256SUMS` covers all four
archives. Checksums detect accidental changes, not publisher identity. Archives
are not promised to be byte-for-byte reproducible or independently audited.

## Verify, extract and run

```sh
cd bin/alpha/0.1.0-alpha.1
sha256sum -c SHA256SUMS
# macOS alternative: shasum -a 256 -c SHA256SUMS
tar -xzf lazyftp_0.1.0-alpha.1_linux_amd64.tar.gz
cd lazyftp_0.1.0-alpha.1_linux_amd64
./lazyftp --help
./lazyftp --demo
```

Choose the archive matching your operating system and architecture. Run in an
interactive terminal. Default startup opens the connection chooser; Esc opens
local browsing, and `--demo` explicitly selects read-only fixtures.
macOS archives are unsigned/unnotarized; follow your organization's security
policy rather than bypassing trust checks. No installer or privileged copy is
required.

## Live mode and preview boundary

Connecting and transferring require an installed `rclone` on `PATH`. Choose
an existing remote or create one with the in-app connection wizard; see
`docs/connections.md` in the source repository for credential-storage limits.
rclone itself and credentials are **not** included. The
local lab pins rclone 1.70.3; compatibility with other versions/backends is not
established by packaging.

```sh
./lazyftp --live --left /path/to/disposable/local \
  --right my-ftp:/incoming --blob my-blob:/container/prefix \
  --rclone-config /path/to/private/rclone.conf
```

Never distribute your rclone config with an archive. Live mode can write files
after explicit confirmation. Files and recursive folders copy local↔remote
through a serial in-memory queue, with individual conflict choices and retry
of failed/canceled files. Empty directories are not created; cancellation may
leave partial files. Synchronization and remote↔remote copying are not supported.
FTP cannot guarantee atomic no-clobber against a concurrent external writer.

`lazyftp-mcp` is a separate stdio server. It denies every operation until an
operator-owned policy grants listing/upload/download for explicit remote and
local roots. It refuses overwrite requests. Read `MCP.md` before enabling permissions;
path checks are not race-proof confinement against hostile filesystem writers.

Storage consumers may type-assert the optional
`storage.Reader` interface: `Read(context.Context, storage.Location, int64)
([]byte, error)`. `*storage.Rclone` returns at most the requested prefix in raw
bytes, including binary/control data; callers must detect binary data and
sanitize terminal output. Negative limits fail, zero returns empty without
I/O, and positive limits bound output and memory to O(limit). Choose a small
limit (for example 64 KiB), not the entire file size. An exact-length prefix
does not distinguish an exact-sized file from a truncated preview. Local
directories, symlinks and other nonregular files are rejected. Remote reads use
context-bound `rclone cat --head`, independently cap captured stdout and discard
stderr; failures are deliberately generic. Context cancellation/deadline errors
are preserved, with a one-minute remote operation ceiling. UI preview behavior
is implemented separately; this storage API alone is not a UI acceptance test.

## Contributor checks

```sh
go test ./...
go test -race ./internal/storage
go vet ./...
bash -n scripts/package.sh
```

Normal tests need no rclone service or cloud account. Controlled process tests
exercise an overproducing backend, cancellation and credential-bearing stderr.
For optional real local FTP/Azurite/S3-compatible tests, prepare the disposable
lab (`make lab-up lab-seed`) and run `make test-integration`. Those checks validate
local mechanics only, not real FTP servers, Azure accounts, production network
conditions or every packaged platform. The combined integration target includes
byte-verified UI folder round trips and MCP subprocess transfers on all three
backends. Terminal behavior and UI preview states also have separate terminal
acceptance checks.
