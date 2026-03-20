# WSFold

WSFold is a local-first Go CLI for composing trusted and external Git repositories around one active workspace.

In v1 it supports:

- `wsfold summon <repo-ref>` for trusted repository attachment
- `wsfold summon-untrusted <repo-ref>` for external repository visibility without symlink embedding
- `wsfold dismiss <repo-ref>` for removing a repo from the current composition
- deterministic `.wsfold/manifest.yaml` state
- deterministic `wsfold.code-workspace` generation for VS Code multi-root workspaces

## Usage

Run commands from the primary repository or worktree root:

```bash
wsfold summon acme/service
wsfold summon-untrusted other/legacy-tool
wsfold dismiss acme/service
```

`repo-ref` is slug-first:

- `owner/name`
- short repo name when the local repo index makes it unambiguous

`summon` only works for repos classified as trusted.
`summon-untrusted` only works for repos classified as external.

## Environment

WSFold reads trust policy from environment variables:

```bash
export WSFOLD_TRUSTED_DIR="$HOME/wsfold/trusted"
export WSFOLD_EXTERNAL_DIR="$HOME/wsfold/external"
export WSFOLD_TRUSTED_GITHUB_ORGS="acme,platform-team"
```

Rules:

- repos under `WSFOLD_TRUSTED_DIR` are eligible for `./refs/<name>` symlink mounting
- repos under `WSFOLD_EXTERNAL_DIR` are never symlinked into the workspace tree
- missing GitHub repos from trusted orgs clone into `WSFOLD_TRUSTED_DIR`
- all other clone targets default to `WSFOLD_EXTERNAL_DIR`

## Generated Files

WSFold writes task-local state into the active workspace:

- `./.wsfold/manifest.yaml`
- `./wsfold.code-workspace`
- `./refs/<name>` for trusted attachments only

Trusted repos are both symlinked under `refs/` and added as VS Code roots.
External repos are added only as VS Code roots.

## Development

Run the automated suite with:

```bash
go test ./...
```
