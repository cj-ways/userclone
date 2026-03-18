# @cj-ways/userclone

Clone your entire GitHub/GitLab setup — personal repos and org repos — in one command.

## Install

```bash
npm install -g @cj-ways/userclone
```

## Usage

```bash
# Clone all your personal repos
userclone clone --token ghp_xxx

# Include org repos
userclone clone --token ghp_xxx --with-orgs

# Dry run
userclone clone --token ghp_xxx --dry-run

# GitLab
userclone clone --token glpat_xxx --gitlab
```

See full docs at [github.com/cj-ways/userclone](https://github.com/cj-ways/userclone).
