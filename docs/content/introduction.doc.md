# Getting Started

SELECT is an SQL client shaped like an IDE. Schema-aware completion, real-time linting, [git-based](/workspace/git/) team workspaces with granular DB [permissions](/workspace/permissions/), local and [proxified connections](/databases/proxified-connections/).

## Download

### macOS

Download the latest `.dmg` from the [releases page](#) and drag **SELECT** to your Applications folder.

### Linux

```bash
# Debian/Ubuntu
sudo dpkg -i selectdb_latest_amd64.deb

# Arch
yay -S selectdb
```

### Build from source

```bash
git clone https://github.com/select-db/select.git
cd select/app
wails build
```

> Requires **Go 1.25+** and **Node 20+**.
