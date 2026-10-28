# Local FTP / Blob / S3 lab

Run inside a Linux amd64 Amp orb; no Docker daemon or cloud account is needed.
See the [suite README](../../../README.md) for common setup. From the repository
root:

```sh
./.agents/setup
cd apps/lazyftp
make lab-up lab-seed
make lab-check
make test-integration
make run-lab
make run-lab LAB_RIGHT=lab-s3:/files/assets
make lab-down
```

All subsequent commands and lab paths are relative to `apps/lazyftp`.

Setup installs rclone 1.70.3, pyftpdlib 2.1.0, Azurite 3.35.0 and the official
SeaweedFS 4.49 standalone binary. Downloads use pinned SHA256 checksums; npm uses
the committed lockfile. `python3 scripts/lab.py install-s3` installs just SeaweedFS.
The pinned S3 archive is
[`linux_amd64.tar.gz`](https://github.com/seaweedfs/seaweedfs/releases/download/4.49/linux_amd64.tar.gz),
SHA256 `3d10545b2317adc1587be02594f97e63420fe1aa7a5ba2af9c9760804574e397`
(also published as the release asset's SHA256 digest by GitHub).

## Why SeaweedFS?

The official MinIO standalone archive URL checked during implementation returned
HTTP 410. SeaweedFS has maintained, official standalone GitHub release binaries
and an S3 gateway in the same process as its local backing services. We use
`weed server -filer -s3`, not `weed mini`: mini starts admin services even with
its admin UI disabled. The server command avoids those extra listeners.

## Remotes and fixtures

The private, generated `.lab/rclone.conf` contains only public development
credentials and explicit local endpoints. It does not read personal remotes.

| Remote | Starting path | Service |
| --- | --- | --- |
| `lab-ftp` | `/incoming` | `lazyftp-lab-ftp`, port 2121 |
| `lab-blob` | `/files/assets` | `lazyftp-lab-blob`, port 10000 |
| `lab-s3` | `/files/assets` | `lazyftp-lab-s3`, port 8333 |

SeaweedFS also needs loopback-only internal HTTP/gRPC ports 8888/18888,
9333/19333, 9340/19340 and 18333. WebDAV, IAM API, Iceberg, Lance and telemetry
are disabled; no public storage portal is created. Internal services are not
production-hardened and must never be exposed. S3 authentication uses explicit
development keys, not ambient AWS credentials. Filer metadata and object data
remain under `.lab/seaweedfs` across restarts.

`lab-seed` adds missing files only; it never resets edited fixtures or deletes
extra files. The fixtures include differing `shared.txt` contents per backend,
backend-only names, empty objects, Unicode/spaces, nested paths and an asymmetric
4233-byte binary S3 object. The local side lives under `.lab/local`.

`run-lab` preserves FTP as the default right pane and Blob in navigation.
Override `LAB_RIGHT` as above to open S3, or with an existing build:

```sh
./bin/lazyftp --live --rclone-config .lab/rclone.conf \
  --left "$PWD/.lab/local" --right lab-s3:/files/assets
```

## Lifecycle and verification

Services run under `amp orb service`, survive orb pause/resume, and listen only
on loopback. Repeating `lab-up` is safe. Occupied ports belonging to another
service are rejected. `check` checks pinned tool versions, config endpoints and
permissions, listening ports and authenticated rclone listings on all backends.
Inspect a service with `amp orb service logs lazyftp-lab-s3` (or `-ftp`/`-blob`).
`lab-down` stops only those three exact service names and preserves all data.
The ownership marker and symlink checks prevent accidentally using unrelated
paths. There is deliberately no destructive reset command.

Normal `make test`/`make check` do not need emulators. `make test-integration`
requires all three seeded services and exercises the real rclone adapter:
byte-verified uploads/downloads (including empty/binary/Unicode files), nested
listings, equal-size conflict refusal and explicit overwrite in both directions,
renamed copies, missing paths, directory rejection and cancellation. Tests use
random unique prefixes and only purge those prefixes, never the seeded roots.

This verifies local S3 compatibility, not real AWS authentication, IAM policies,
TLS, large multipart transfers, production behavior or every S3-compatible
provider. FTP and Azurite remain development-only services too.
