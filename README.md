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
# Clone all your personal repos to ~/Desktop
userclone clone --token ghp_xxx

# Include all org repos too
userclone clone --token ghp_xxx --with-orgs

# Clone only repos from specific orgs
userclone clone --token ghp_xxx --org cj-ways --org another-org

# Skip personal repos, only org repos
userclone clone --token ghp_xxx --only-orgs --with-orgs

# GitLab support
userclone clone --token glpat_xxx --gitlab

# Dry run to preview
userclone clone --token ghp_xxx --with-orgs --dry-run
```

## Config

Generate a config file:

```bash
userclone init
```

This creates `~/.userclone.yml`:

```yaml
default_dest: ~/Desktop
default_platform: github
with_orgs: false

github:
  token: ghp_xxx

gitlab:
  token: glpat_xxx
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
