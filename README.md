# dssh — Dead Simple SSH Launcher

Dead-simple CLI tool to manage and connect to SSH hosts via saved connections.

<!-- TODO: Replace with VHS recording of `dssh` picker + connect flow -->
![dssh demo](assets/demo.gif)

## Features

- **Instant connect** — `dssh myserver` and you're in
- **Interactive picker** — run `dssh` with no args to choose from a list
- **Wizard** 🪄 — TUI form to create connections without memorizing flags
- **Password encryption** — AES-256-GCM + Argon2id, protected by a master passphrase
- **No dependencies** — single static binary, uses your system's `ssh`
- **Cross-platform** — Linux, macOS, Windows, FreeBSD (amd64 + arm64)

## Installation

### From GitHub Releases (recommended)

Download the latest binary for your platform from [Releases](https://github.com/madLinux7/dssh-launcher/releases) and place it in your `$PATH`.

```bash
# Example for Linux amd64
curl -L https://github.com/madLinux7/dssh-launcher/releases/latest/download/dssh-linux-amd64 -o dssh
chmod +x dssh
sudo mv dssh /usr/local/bin/
```

### From source

Requires Go 1.23+.

```bash
go install github.com/madLinux7/dssh-launcher/cmd/dssh@latest
```

### Build locally

```bash
git clone https://github.com/madLinux7/dssh-launcher.git
cd dssh-launcher
make build
```

## Usage

### Quick start - Let's GO!

```bash
# Add a connection (will use default pubkey identity)
dssh add myserver root@192.168.1.10

# Connect
dssh myserver

# You're in!
```

### Add a connection

```bash
# user@host (port 22 by default)
dssh add myserver root@192.168.1.10

# Custom port
dssh add myserver -p 2222 root@192.168.1.10

# SSH URI syntax
dssh add myserver ssh://root@192.168.1.10:2222
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

### Interactive picker

Run `dssh` with no arguments to launch the connection picker.

<!-- TODO: Replace with VHS recording of `dssh` picker -->
![dssh picker](assets/picker.gif)

Navigate with arrow keys, press Enter to connect, Escape or `q` to cancel.

### Wizard

Create a connection interactively with the TUI wizard.

```bash
dssh wizard
```

<!-- TODO: Replace with VHS recording of `dssh wizard` -->
![dssh wizard](assets/wizard.gif)

The wizard supports both key-based and password-based authentication. For password auth, you'll be prompted to create a master passphrase on first use.

### List connections

```bash
dssh list    # or: dssh ls
```

```
NAME       USER   HOST           PORT  AUTH
myserver   root   192.168.1.10   22    key
webbox     deploy web.host       8022  key
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
| `dssh add [-p PORT] <name> <target>` | Save a new connection |
| `dssh rm <name>` | Delete a saved connection |
| `dssh list` | List all saved connections |
| `dssh wizard` | Interactive form to create a connection |
| `dssh reset` | Delete all data (double confirmation) |
| `dssh --version` | Print version |

## How It Works

dssh is a thin wrapper around your system's `ssh` binary:

- **Key auth** — `syscall.Exec` replaces the dssh process with ssh (zero overhead, full terminal control)
- **Password auth** — ssh runs as a child process with `SSH_ASKPASS` to supply the decrypted password (no `sshpass` needed)
- **Data** — connections stored in SQLite at `~/.dssh/dssh.db`, no config files
- **Crypto** — AES-256-GCM encryption with Argon2id key derivation for stored passwords


## Project Structure

```
dssh-launcher/
├── cmd/dssh/main.go           # Entrypoint
├── internal/
│   ├── cli/                   # Cobra commands (add, rm, list, wizard, root)
│   ├── crypto/                # AES-256-GCM + Argon2id
│   ├── db/                    # SQLite (connections + settings)
│   ├── model/                 # Connection struct
│   ├── ssh/                   # ssh exec (key auth) + SSH_ASKPASS (password auth)
│   └── tui/                   # tview picker + wizard
├── .github/workflows/         # CI/CD
├── Makefile
└── go.mod
```

## License

MIT
