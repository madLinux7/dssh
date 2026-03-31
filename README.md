# dssh — **D**ead **S**imple S**SH**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/madLinux7/dssh?color=7B2FBE&logo=github)](https://github.com/madLinux7/dssh/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows%20%7C%20freebsd-D946EF)](https://github.com/madLinux7/dssh/releases)
[![SQLite](https://img.shields.io/badge/storage-SQLite-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![AES-256-GCM](https://img.shields.io/badge/crypto-AES--256--GCM%20%2B%20Argon2id-22C55E?logo=letsencrypt&logoColor=white)](#password-encryption)

The only SSH connection management tool you'll ever need. Dead-simple and cross-platform for every CLI.

Three core features: **Create, Connect, Delete**.

Passwords are encrypted using a passphrase (_you should consider using pubkeys only tho ;))_.

<!-- TODO: Replace with VHS recording of `dssh` picker + connect flow -->
![dssh demo](demo_1.gif)

## Features

- **Instant connect** 🚀 — `dssh myserver` and you're in
- **Fancy interactive picker** ✨ — run `dssh` with no args to connect, add and delete connections
- **Wizard** 🪄 — create connections without memorizing flags
- **Start in a directory** 📂 — optionally land in a specific remote directory on connect
- **Password encryption** 🔒 — AES-256-GCM + Argon2id, protected by a master passphrase
- **No dependencies** 🗽 — single static binary, uses your system's `ssh`
- **Cross-platform** 💻 — Linux, macOS, Windows, FreeBSD (amd64 + arm64)
- **Dead simple migration** 📦 — moving to a new machine? Just take `~/.dssh/dssh.db` with you. That's it.

## How it works

dssh is a thin wrapper around your system's `ssh` binary:

- **Key auth** — `syscall.Exec` replaces the dssh process with ssh (zero overhead, full terminal control)
- **Password auth** — ssh runs as a child process with `SSH_ASKPASS` to supply the decrypted password (no `sshpass` needed)
- **Data** — connections stored in SQLite at `~/.dssh/dssh.db`, no config files
- **Crypto** — AES-256-GCM encryption with Argon2id key derivation for stored passwords

## Installation

### From GitHub Releases (recommended)

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

## Usage

### Quick start - Let's Go!

```bash
# Add a connection (will use default pubkey identity)
dssh add myserver root@192.168.1.10

# Connect
dssh myserver

# You're in!
```

```bash
# Open TUI where you can basically do anything (Interactive picker)
dssh
```

Run `dssh` with no arguments to launch the connection picker.

Navigate with arrow keys, press Enter to connect, Escape or `q` to cancel.

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

<!-- TODO: Replace with VHS recording of `dssh add` -->
![dssh add](assets/add.gif)

### Connect to a host

```bash
# Direct connect by name
dssh myserver

# Pass extra args to ssh
dssh myserver -- -v -L 8080:localhost:80
```

### Wizard

Create a connection interactively with the TUI wizard.

```bash
dssh wizard
dssh new # alias
```

<!-- TODO: Replace with VHS recording of `dssh wizard` -->
![dssh wizard](assets/wizard.gif)

The wizard supports both key-based and password-based authentication. For password auth, you'll be prompted to create a master passphrase on first use.

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

## Password Encryption

When you save a connection with password authentication, dssh encrypts the password locally:

1. **First time** — you create a master passphrase (prompted twice to confirm)
2. **On save** — password is encrypted with AES-256-GCM; the key is derived from your master passphrase via Argon2id
3. **On connect** — you re-enter the master passphrase to decrypt

The master passphrase is never stored. A random salt is saved in the database to derive the encryption key. Each password gets its own unique nonce.

All data lives in `~/.dssh/dssh.db` (SQLite).

## Command Reference

| Command | Description |
|---|---|
| `dssh` | Launch interactive connection picker |
| `dssh <name>` | Connect to a saved host by name |
| `dssh <name> -- <args>` | Connect with extra args forwarded to ssh |
| `dssh add [-p PORT] [-d DIR] <name> <target> [password]` | Save a new connection |
| `dssh rm <name>` | Delete a saved connection |
| `dssh list` / `dssh ls` | List all saved connections |
| `dssh wizard` / `dssh new` | Interactive form to create a connection |
| `dssh reset` | Delete all data (double confirmation) |
| `dssh --version` | Print version |

## ✨ Acknowledgements ✨

dssh stands on the shoulders of giants. Huge thanks to the maintainers and contributors of these amazing projects:

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** by [Charm](https://charm.sh) — a sick TUI framework that powers the picker, wizard, and delete views
- **[Bubbles](https://github.com/charmbracelet/bubbles)** by [Charm](https://charm.sh) — ready-made TUI components (lists, text inputs) so I don't need to reinvent the wheel
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** by [Charm](https://charm.sh) — style definitions that make the terminal look ✨ pretty ✨
- **[Cobra](https://github.com/spf13/cobra)** by [Steve Francia](https://github.com/spf13) — the CLI framework powering every `dssh` command
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** by [Jan Mercl](https://gitlab.com/cznic) — pure-Go SQLite driver that lets you ship a single static binary with zero CGO
- **[golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)** by the Go team — Argon2id key derivation keeping your passwords safe
- **[golang.org/x/term](https://pkg.go.dev/golang.org/x/term)** by the Go team — secure terminal password reading without echo
- **[UPX](https://upx.github.io/)** by Markus Oberhumer, Laszlo Molnar & John Reiser - Reducing the release binary by an **insane 62%**! (8.4MB _Regular Go binary_ -> 3.2.MB _Release binary_)