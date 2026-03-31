# dssh — **D**ead **S**imple S**SH**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/madLinux7/dssh?color=7B2FBE&logo=github)](https://github.com/madLinux7/dssh/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows%20%7C%20freebsd-D946EF)](https://github.com/madLinux7/dssh/releases)
[![SQLite](https://img.shields.io/badge/storage-SQLite-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![AES-256-GCM](https://img.shields.io/badge/crypto-AES--256--GCM%20%2B%20Argon2id-22C55E?logo=letsencrypt&logoColor=white)](#password-encryption)

The only SSH connection management tool you'll ever need. No dependencies, no more editing `/etc/hosts`.

Four core features: **Create, Connect, Edit, Delete**. Dead-simple and cross-platform for every CLI.

Passwords are encrypted using a master passphrase (_you should consider using pubkeys only tho ;))_.

<!-- TODO: Replace with VHS recording of `dssh` picker + connect flow -->
![dssh demo](demo_1.gif)

## Table of Contents

- [Features](#features)
- [How it works](#how-it-works)
- [Usage](#usage)
  - [Command Reference](#command-reference)
  - [Quick start](#quick-start---lets-go)
  - [Add a connection](#add-a-connection)
  - [Connect to a host](#connect-to-a-host)
  - [Create](#create)
  - [Edit a connection](#edit-a-connection)
  - [Delete a connection (TUI)](#delete-a-connection-tui)
  - [List connections](#list-connections)
  - [Remove a connection](#remove-a-connection)
  - [Reset everything](#reset-everything)
- [Installation](#installation)
  - [Install script (recommended)](#install-script-recommended)
  - [From GitHub Releases](#from-github-releases)
  - [From source](#from-source)
  - [Build locally](#build-locally)
- [Acknowledgements](#-acknowledgements-)

## Features

**Core:**

- **Fancy TUI** ✨ — run `dssh` with no args to connect and manage all your connections
- **Instant connect** 🚀 — `dssh myserver` and you're in
- **Create Wizard** 🪄 — easily add new connections without memorizing flags
- **Edit** ✏️ — no need to delete and re-add connections, just edit them

Also:

- **Launch into a directory** 📂 — optionally land in a specific remote directory on connect
- **Password encryption** 🔒 — AES-256-GCM + Argon2id, protected by a master passphrase
- **Cross-platform** 💻 — Linux, macOS, Windows, FreeBSD (amd64 + arm64)
- **Dead simple migration** 📦 — moving to a new machine? Just take `~/.dssh/dssh.db` with you. That's it.

## How it works

dssh is a thin wrapper around your system's `ssh` binary:

- **Key auth** — `syscall.Exec` replaces the dssh process with ssh (zero overhead, full terminal control)
- **Password auth** — ssh runs as a child process with `SSH_ASKPASS` to supply the decrypted password (no `sshpass` needed)
- **Data** — connections stored in SQLite at `~/.dssh/dssh.db`, no config files
- **Crypto** — AES-256-GCM encryption with Argon2id key derivation for stored passwords

## Usage

### Command Reference

| Command | Description |
|---|---|
| `dssh` | Launch interactive connection picker |
| `dssh <name>` | Connect to a saved host by name |
| `dssh <name> -- <args>` | Connect with extra args forwarded to ssh |
| `dssh add [-p PORT] [-d DIR] <name> <target> [password]` | Save a new connection |
| `dssh rm <name>` | Delete a saved connection |
| `dssh list` / `dssh ls` | List all saved connections |
| `dssh create` / `dssh new` | Interactive form to create a connection |
| `dssh reset` | Delete all data (double confirmation) |
| `dssh --version` | Print version |

### Quick start - Let's Go!

```bash
# Add a connection (will use default pubkey identity)
dssh add myserver root@192.168.1.10

# Connect
dssh myserver

# You're in!

# Permanently delete the connection (no confirmation asked)
dssh rm myserver
```

```bash
# Open the TUI where you can basically do anything
dssh
```

Run `dssh` with no arguments to launch the TUI.

Switch between tabs with `Tab` / `Shift+Tab`, navigate lists with arrow keys, press `Enter` to select, `ESC` or `Q` to quit.

### Add a connection

```bash
# user@host (port 22 by default)
dssh add myserver root@192.168.1.10

# Custom port
dssh add myserver -p 2222 root@192.168.1.10

# SSH URI syntax
dssh add myserver ssh://root@192.168.1.10:2222

# Start in a specific remote directory
dssh add myserver -d /var/www root@192.168.1.10

# With password (will prompt for master passphrase)
dssh add myserver root@192.168.1.10 'my-ssh-password'
```

### Connect to a host

```bash
# Direct connect by name
dssh myserver

# Pass extra args to ssh
dssh myserver -- -v -L 8080:localhost:80
```

### Create

Launch the TUI wizard to create a connection interactively.

```bash
dssh create
dssh new # alias
```

<!-- TODO: Replace with VHS recording of `dssh wizard` -->
![dssh wizard](demo_wizard.gif)

The wizard supports both key-based and password-based authentication. For password auth, you'll be prompted to create a master passphrase on first use.

### Edit a connection

Launch the TUI directly on the Edit tab to modify an existing connection.

```bash
dssh edit
```

### Delete a connection (TUI)

Launch the TUI directly on the Delete tab. Requires pressing Enter 3 times on the same item to confirm.

```bash
dssh delete
```

### List connections

```bash
dssh list
dssh ls # alias
```

```
NAME       USER   HOST           PORT  AUTH  DIR
myserver   root   192.168.1.10   22    key   /var/www
webbox     deploy web.host       8022  key   -
```

### Remove a connection

```bash
dssh rm myserver
```

### Reset everything

Wipe all saved connections, encrypted passwords, and settings (deletes the SQLite database). Requires two confirmations to prevent accidents.

```bash
dssh reset
```

```
This will delete ALL saved connections, passwords, and settings. Continue? (yes/no): yes
Are you sure? This cannot be undone. Type 'reset' to confirm: reset
All data has been reset
```

## Installation

### Install script (recommended)

**Linux / macOS / FreeBSD:**

```bash
curl -fsSL https://raw.githubusercontent.com/madLinux7/dssh/main/install.sh | sh
```

Installs to `~/.local/bin` by default. Override with `INSTALL_DIR=/custom/path`.

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/madLinux7/dssh/main/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\dssh` and adds it to your PATH automatically.

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/madLinux7/dssh/releases) and place it in your `$PATH`.

```bash
# Example for Linux amd64
curl -L https://github.com/madLinux7/dssh/releases/latest/download/dssh-linux-amd64 -o dssh
chmod +x dssh
sudo mv dssh /usr/local/bin/
```

### From source

Requires Go 1.26+.

```bash
go install github.com/madLinux7/dssh/cmd/dssh@latest
```

### Build locally

```bash
git clone https://github.com/madLinux7/dssh.git
cd dssh
# Build only with optimized -ldflags
make build
# Build and compress binary (upx needed)
make release
```

## ✨ Acknowledgements ✨

dssh has no need for reinventing the wheel — thanks to the maintainers and contributors of these amazing projects:

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** by [Charm](https://charm.sh) — pretty sick TUI framework
- **[Bubbles](https://github.com/charmbracelet/bubbles)** by [Charm](https://charm.sh) — ready-to-go TUI components (lists, text inputs) so I don't need to reinvent the wheel
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** by [Charm](https://charm.sh) — style definitions that make the terminal look ✨ pretty ✨
- **[Cobra](https://github.com/spf13/cobra)** by [Steve Francia](https://github.com/spf13) — the CLI framework powering every `dssh` command
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** by [Jan Mercl](https://gitlab.com/cznic) — pure-Go SQLite driver that lets you ship a single static binary with zero CGO
- **[golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)** by the Go team — Argon2id key derivation keeping your passwords safe
- **[golang.org/x/term](https://pkg.go.dev/golang.org/x/term)** by the Go team — secure terminal password reading without echo
- **[UPX](https://upx.github.io/)** by Markus Oberhumer, Laszlo Molnar & John Reiser - Reducing the Linux release binary size by an **insane 64%**! (9.0MB -> 3.3MB)