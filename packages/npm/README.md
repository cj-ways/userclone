# @cj-ways/userclone

Clone your entire GitHub/GitLab setup — personal repos and org repos — in one command.

## Install

```bash
npm install -g @cj-ways/userclone
```

## Setup

```bash
userclone init                    # creates ~/.userclone.yml
# edit it → paste your GitHub/GitLab token
```

## Usage

```bash
userclone clone                   # clone all personal repos
userclone clone --with-orgs       # include org repos
userclone clone --dry-run         # preview without cloning
userclone clone --gitlab          # use GitLab instead
```

See full docs at [github.com/cj-ways/userclone](https://github.com/cj-ways/userclone).
