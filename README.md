# lfs-zstd-filter

A git-lfs **extension** that transparently compresses files with
[zstd](https://facebook.github.io/zstd/) before they are stored in LFS object
storage, and decompresses them again on checkout.

```
working tree  ──clean──▶  zstd-compressed LFS object  ──smudge──▶  working tree
  (original)                 (stored & uploaded as-is)                (original)
```

## How it works

git-lfs has an [extension system](git-lfs/docs/extensions.md) that lets you
register a subprocess as a stdin→stdout transformer between git and the LFS
object store. `lfs-zstd-filter` plugs in there:

- **clean** (`git add`): receives raw file bytes on stdin, writes
  zstd-compressed bytes to stdout. git-lfs stores the compressed bytes as the
  LFS object and uploads them to the server.
- **smudge** (`git checkout`): receives compressed bytes from the local LFS
  object store on stdin, writes decompressed bytes to stdout. git writes those
  to the working tree.

Stock LFS servers are completely unaware of compression. They store and serve
the compressed bytes as ordinary LFS objects.

The pointer file stored in the git repository records the SHA-256 of the
compressed object (managed entirely by git-lfs), plus an `ext-0-zstd` line
with the SHA-256 of the original file — allowing git-lfs to verify integrity
after decompression.

```
version https://git-lfs.github.com/spec/v1
ext-0-zstd sha256:<sha256-of-original-file>
oid sha256:<sha256-of-compressed-file>
size <compressed-size>
```

## Build

```sh
go build -o lfs-zstd-filter ./cmd/lfs-zstd-filter
```

Place the resulting binary somewhere on your `PATH` (e.g.
`~/.local/bin/lfs-zstd-filter` or alongside `git-lfs`).

## Configuration

### 1. Register the extension in git config

Because this is not a widespread extension, it is highly recommended to install
it locally (per-repository) so you don't inadvertently require it for other
repositories you clone.

Run this inside your repository:

```sh
lfs-zstd-filter install
```

This will automatically register the extension in your repository's local `.git/config`.

*(If you really want to enable it for all repositories on your machine, you can run `lfs-zstd-filter install --global`, but note that any other users cloning those repositories will also need this tool installed).*

### 2. Track files with git-lfs as usual

The extension is triggered automatically whenever git-lfs processes a file.
Set up LFS tracking in `.gitattributes` as normal:

```
*.bin filter=lfs diff=lfs merge=lfs -text
*.dat filter=lfs diff=lfs merge=lfs -text
```

> **Note**: compression is applied to *all* files processed by git-lfs on
> machines where the extension is registered. If you commit files on a machine
> without the extension, they are stored uncompressed — and the smudge
> pass-through will still serve them correctly on checkout.

## Behaviour details

`clean` is unconditional — it always wraps the input in a custom zstd skippable frame (`0x50 0x2A 0x4D 0x18 0x04 0x00 0x00 0x00 L F S Z`) followed by a standard zstd compressed frame, regardless of the file's existing format. A `.zst`, `.gz`, or `.jpg` file is stored as `[LFSZ skippable] + zstd(original-bytes)`. On smudge, the zstd decoder naturally skips the custom frame, the outer zstd frame is stripped, and the original file — still in its original format — is restored to the working tree.

`smudge` checks the first 12 bytes for our custom `LFSZ` skippable frame. If absent, the bytes are copied through unchanged.

| Scenario | clean | smudge |
|---|---|---|
| Any file with extension active | Compresses unconditionally (with `LFSZ` skippable) | Decompresses (skipping `LFSZ`) |
| Pre-extension object (non-zstd bytes) | — | Passes through unchanged |
| Pre-extension object (zstd-magic bytes, e.g. old `.zst`) | — | Passes through unchanged |
| Empty file | Empty output (contains `LFSZ` skippable) | Empty output |

### Migration from an uncompressed repository

If you enable the extension on an existing repository, LFS objects already
stored on the server were committed without compression. On checkout those
objects are fed to `smudge`: because they do not start with our custom `LFSZ`
skippable frame, they are safely passed through unchanged.

This includes pre-existing `.zst` files that were pushed to LFS before the
extension was enabled. Because they start with the standard zstd magic and not
our custom `LFSZ` skippable frame, `smudge` correctly identifies them as raw
pre-extension objects and passes them through without accidentally decompressing
them. After a `git add` with the extension enabled those files will be
re-committed as `[LFSZ skippable] + zstd(original-.zst-bytes)` and will continue
to round-trip correctly.

## Limitations

- The LFS extension system is marked **experimental** by the git-lfs project.
  Test thoroughly before relying on it with production data.
- Every contributor who checks out tracked files must have `lfs-zstd-filter`
  on their `PATH` and the extension registered in their git config; otherwise
  smudge will fail with *"extension 'zstd' is not configured"*.
- Priority `0` means this runs first. If you chain multiple LFS extensions,
  adjust the priority accordingly.
