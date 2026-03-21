# WSFold

WSFold is a local-first Go CLI for composing trusted and external Git repositories around one active workspace.

In v1 it supports:

- `wsfold init` for initializing a workspace in the current directory
- `wsfold summon [repo-ref]` for trusted repository attachment, with a remote-aware picker when no ref is provided
- `wsfold reindex` for refreshing the trusted GitHub remote cache
- `wsfold summon-external [repo-ref]` for external repository visibility without symlink embedding
- `wsfold dismiss [repo-ref]` for removing a repo from the current composition
- deterministic `.wsfold/manifest.yaml` state
- deterministic `<workspace-dirname>.code-workspace` generation for VS Code multi-root workspaces

## Usage

Initialize once from the directory you want to treat as the primary workspace:

```bash
wsfold init
```

After that, run commands from anywhere inside that workspace tree:

```bash
wsfold summon acme/service
wsfold reindex
wsfold summon-external other/legacy-tool
wsfold dismiss acme/service
```

If you omit `repo-ref`, `wsfold` opens an interactive Bubble Tea picker with live fuzzy filtering:

```bash
wsfold summon
wsfold summon-external
wsfold dismiss
```

`wsfold summon` picker behavior:

- always shows local trusted repos immediately
- includes cached remote repos from `WSFOLD_TRUSTED_GITHUB_ORGS` when available
- refreshes trusted remote metadata in the background with `gh` and live-updates the open picker
- clones trusted remote repos with `gh repo clone`
- keeps shell completion local-only by design

Zsh completion:

```bash
eval "$(wsfold completion zsh)"
```

This local completion currently suggests:

- trusted local repos for `wsfold summon`
- external local repos for `wsfold summon-external`
- repos already attached in the current workspace for `wsfold dismiss`

`repo-ref` is slug-first:

- `owner/name`
- short repo name when the local repo index makes it unambiguous

`summon` only works for repos classified as trusted.
`summon-external` only works for repos classified as external.

## Environment

WSFold reads trust policy from environment variables:

```bash
export WSFOLD_TRUSTED_DIR="$HOME/wsfold/trusted"
export WSFOLD_EXTERNAL_DIR="$HOME/wsfold/external"
export WSFOLD_TRUSTED_GITHUB_ORGS="acme,platform-team"
```

Rules:

- repos under `WSFOLD_TRUSTED_DIR` are eligible for `./_prj/<name>` symlink mounting by default
- repos under `WSFOLD_EXTERNAL_DIR` are never symlinked into the workspace tree
- missing GitHub repos from trusted orgs may clone into `WSFOLD_TRUSTED_DIR` via `wsfold summon`
- `wsfold summon` without a ref reads cached trusted GitHub repos from the user cache directory and refreshes them with `gh`
- trusted remote summon clones use `gh repo clone`, following the user’s `gh` git protocol settings
- `wsfold reindex` performs a blocking refresh of the trusted GitHub cache
- run `gh auth login` to enable trusted remote refresh
- `wsfold summon-external` does not clone from remote; it only attaches repos already present under `WSFOLD_EXTERNAL_DIR`
- `WSFOLD_PROJECTS_DIR` optionally overrides the trusted mount directory name; default is `_prj`

## Generated Files

WSFold writes task-local state into the active workspace:

- `./.wsfold/manifest.yaml`
- `./<workspace-dirname>.code-workspace`
- `./_prj/<name>` for trusted attachments only by default

Trusted repos are both symlinked under `_prj/` and added as VS Code roots.
External repos are added only as VS Code roots.

## Development

Run the automated suite with:

```bash
go test ./...
```

## Build and Release

WSFold ships prebuilt release binaries for:

- macOS Apple Silicon (`darwin/arm64`)
- macOS Intel (`darwin/amd64`)
- Linux ARM64 (`linux/arm64`)
- Linux x86_64 (`linux/amd64`)

Local developer commands:

```bash
make test
make build
make release-check
make release-snapshot
```

What they do:

- `make build` builds the current platform binary into `./dist/wsfold`
- `make release-check` validates `.goreleaser.yml`
- `make release-snapshot` creates the same multi-platform archives locally that GitHub Releases will publish
- the release targets bootstrap a pinned `goreleaser` binary automatically into `./.bin/`, so no global `goreleaser` install is required

Release flow:

1. Push a SemVer tag with `v` prefix, for example `v0.0.1`.
2. GitHub Actions runs tests and publishes release assets through GoReleaser.
3. The release will contain:
   - `wsfold_Darwin_x86_64.tar.gz`
   - `wsfold_Darwin_arm64.tar.gz`
   - `wsfold_Linux_x86_64.tar.gz`
   - `wsfold_Linux_arm64.tar.gz`
   - `checksums.txt`
4. Update the Homebrew tap repo `wsfold/homebrew-wsfold` with the new version and checksums.
