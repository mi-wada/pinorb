# pinorb

`pinorb` pins the [CircleCI orbs](https://circleci.com/docs/orb-intro/) in your
`.circleci/config.yml` to an exact patch version — the CircleCI counterpart to
[pinact](https://github.com/suzuki-shunsuke/pinact), which does the same for
GitHub Actions.

When an orb is referenced with only a major (`@3`) or major.minor (`@5.1`)
version, CircleCI silently resolves it to the latest matching release at build
time. `pinorb` resolves it once, up front, and rewrites the file so the version
is reproducible.

## Install

Homebrew:

```sh
brew install mi-wada/tap/pinorb
```

Go:

```sh
go install github.com/mi-wada/pinorb@latest
```

Or download a prebuilt binary from the [releases page](https://github.com/mi-wada/pinorb/releases).

## Usage

```sh
# Search the .circleci/ directory (the default).
pinorb run

# Pin specific files or directories (directories are searched recursively).
pinorb run path/to/config.yml path/to/dir

# Verify only — don't write; exit non-zero if anything is unpinned (CI-friendly).
pinorb run --check
```

Each path may be a file or a directory; directories are searched recursively
for `*.yml` / `*.yaml`. With no path, `pinorb` searches `.circleci/` (falling
back to `.circleci/config.yml`), so both single-file configs and split
[setup-workflow](https://circleci.com/docs/dynamic-config/) configs
(`.circleci/config/*.yml`) are handled automatically.

### What it does

For each orb declaration inside an `orbs:` block:

| Reference                       | Action                                      |
| ------------------------------- | ------------------------------------------- |
| `circleci/aws-cli@5.1`          | → `@5.1.4` (latest patch within `5.1`)      |
| `circleci/path-filtering@3`     | → `@3.0.0` (latest patch within major `3`)  |
| `circleci/node@latest`          | → latest released version                   |
| `circleci/slack@4.13.3`         | left unchanged (already pinned)             |
| `circleci/continuation@volatile`| left unchanged (non-numeric tag)            |

Comments, formatting, and everything outside `orbs:` blocks are preserved.

## Authentication

`pinorb` queries the CircleCI orb registry. Public orbs need no credentials.
Private orbs (those in your own org's namespace) — and heavier usage that hits
rate limits — require a [CircleCI API token](https://app.circleci.com/settings/user/tokens):

```sh
pinorb run --token "$CIRCLE_TOKEN"
# or via environment:
export CIRCLE_TOKEN=...   # CIRCLECI_TOKEN is also accepted
pinorb run
```
