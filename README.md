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
userclone clone
```

The interactive wizard walks you through:

1. **Platform** — GitHub or GitLab
2. **Authentication** — auto-detects `gh` CLI token, or OAuth device flow, or paste manually
3. **Destination** — Desktop (default) or custom path
4. **Folder name** — "GitHub User", your display name, or custom
5. **Visibility** — All, public only, or private only
6. **Scope** — Everything, personal only, orgs only, or pick manually
7. **Structure** — Separated (orgs alongside) or Grouped (everything nested)
8. **Summary** — Review and confirm before cloning

### Wizard Example

```
? Which platform? GitHub
  Found token from GitHub CLI (gh)
  Authenticated as Saba Janelidze (@sabajanelidze)

? Where to clone? Desktop (~/Desktop)
? Folder name for your repos? Saba Janelidze
? Which repos? All repos

Fetching repos...
  Found 23 personal repos, 35 repos across 3 orgs

? What to clone? Everything (23 personal + 35 from 3 orgs)
? Folder structure? Separated — orgs alongside, personal in "Saba Janelidze/"

━━ Summary ━━━━━━━━━━━━━━━━━━━
Platform:    GitHub
User:        Saba Janelidze (@sabajanelidze)
Path:        ~/Desktop
Folder:      Saba Janelidze
Repos:       58 (23 personal + 35 from 3 orgs)
Visibility:  All
Structure:   Separated

? Show full repo list? No
? Proceed? Yes
```

## Directory Layout

### Separated (default)

Orgs sit alongside the personal folder:

```
~/Desktop/
├── Saba Janelidze/        <- personal repos
│   ├── repo-1/
│   └── repo-2/
├── cj-ways/               <- org repos
│   └── org-repo-1/
└── another-org/
    └── org-repo-2/
```

### Grouped

Everything under one folder:

```
~/Desktop/Saba Janelidze/
├── repo-1/                <- personal repos
├── repo-2/
├── cj-ways/               <- org repos nested inside
│   └── org-repo-1/
└── another-org/
    └── org-repo-2/
```

## Flags (Shortcuts)

Flags skip wizard steps. Provide all flags for fully non-interactive usage.

| Flag | Short | Description |
|------|-------|-------------|
| `--token` | `-t` | GitHub/GitLab personal access token |
| `--dest` | `-d` | Base destination directory (legacy mode, no folder name) |
| `--with-orgs` | | Clone repos from all orgs |
| `--org` | `-o` | Clone specific org(s) only (repeatable) |
| `--only-orgs` | | Skip personal repos |
| `--exclude` | `-e` | Comma-separated repo names to skip |
| `--skip-archived` | | Skip archived repos |
| `--skip-forks` | | Skip forked repos |
| `--private-only` | | Only private repos |
| `--public-only` | | Only public repos |
| `--pick` | | Interactive checkbox selection |
| `--dry-run` | | Preview without cloning |
| `--flat` | | No org grouping, everything in dest root |
| `--gitlab` | | Use GitLab instead of GitHub |
| `--gitlab-url` | | Self-hosted GitLab URL |

### Non-Interactive Examples

```bash
# Clone personal repos only (old behavior)
userclone clone --token ghp_xxx --dest ~/Desktop

# Clone everything including orgs
userclone clone --token ghp_xxx --dest ~/Desktop --with-orgs

# Only public repos from a specific org
userclone clone --token ghp_xxx --dest ~/Projects --org cj-ways --public-only

# Preview what would be cloned
userclone clone --dry-run
```

## Authentication

Token is resolved in this order:

1. `--token` flag
2. `~/.userclone.yml` config file
3. Environment variable (`GITHUB_TOKEN` / `GITLAB_TOKEN`)
4. `gh` CLI / `glab` CLI (auto-detected)
5. OAuth Device Flow (GitHub — requires registered OAuth App)
6. Manual paste (interactive prompt)

Tokens obtained interactively can be saved to `~/.userclone.yml` for future use.

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

## Token Scopes

**GitHub:** `repo` (full control of private repositories)

**GitLab:** `read_api` (read access to the API)

## License

MIT
