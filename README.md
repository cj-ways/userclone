# userclone

Clone your entire GitHub/GitLab setup — personal repos and org repos — in one command.

Perfect for onboarding to a new device.

## Install

```bash
npm install -g @cj-ways/userclone
```

Or download a binary from [Releases](https://github.com/cj-ways/userclone/releases).

## Quick Start

```bash
# 1. Set up your token (one time)
userclone init                          # creates ~/.userclone.yml
# edit ~/.userclone.yml → paste your GitHub token

# 2. Clone everything
userclone clone                         # personal repos → ~/Desktop
userclone clone --with-orgs             # personal + all org repos
userclone clone --org cj-ways           # personal + specific org
userclone clone --only-orgs --with-orgs # org repos only
userclone clone --dry-run               # preview without cloning
userclone clone --gitlab                # use GitLab instead
```

You can also set the token via environment variable (`GITHUB_TOKEN` / `GITLAB_TOKEN`) or pass it directly with `--token`.

## Config

```bash
userclone init
```

Creates `~/.userclone.yml`:

```yaml
default_dest: ~/Desktop
default_platform: github
with_orgs: false

github:
  token: ghp_xxx               # or set GITHUB_TOKEN env var

gitlab:
  token: glpat_xxx              # or set GITLAB_TOKEN env var
  url: https://gitlab.com

exclude:
  - dotfiles
  - old-project

orgs:
  my-org:
    exclude:
      - legacy-repo
```

Set defaults quickly:

```bash
userclone default platform github
userclone default dest ~/Projects
userclone default with-orgs true
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--token` | `-t` | GitHub/GitLab personal access token |
| `--dest` | `-d` | Base destination directory (default `~/Desktop`) |
| `--with-orgs` | | Also clone repos from all orgs |
| `--org` | `-o` | Clone specific org(s) only (repeatable) |
| `--only-orgs` | | Skip personal repos |
| `--exclude` | `-e` | Comma-separated repo names to skip |
| `--skip-archived` | | Skip archived repos |
| `--skip-forks` | | Skip forked repos |
| `--private-only` | | Only private repos |
| `--public-only` | | Only public repos |
| `--pick` | | Interactive selection per group |
| `--dry-run` | | Preview without cloning |
| `--flat` | | No org grouping, everything in dest root |
| `--gitlab` | | Use GitLab instead of GitHub |
| `--gitlab-url` | | Self-hosted GitLab URL |

## Directory Layout

```
~/Desktop/
├── personal-repo-1/
├── personal-repo-2/
├── my-org/
│   ├── org-repo-1/
│   └── org-repo-2/
└── another-org/
    └── repo-3/
```

Use `--flat` to skip org grouping.

## Token Scopes

**GitHub:** `repo` (full control of private repositories)

**GitLab:** `read_api` (read access to the API)

## Priority

Settings are resolved lowest to highest:

1. Code defaults
2. `~/.userclone.yml`
3. Environment variables (`GITHUB_TOKEN` / `GITLAB_TOKEN`)
4. CLI flags

## License

MIT
