# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build      # CGO_ENABLED=0 static binary with stripped ldflags
make test       # go test ./...
make release    # build + UPX compress
make clean      # remove binary artifacts
```

Version is injected via `-ldflags -X main.version=...` from `git describe --tags`.

## Architecture

**dssh** (Dead Simple SSH) is a cross-platform SSH connection manager. It wraps the system `ssh` binary, storing connections in SQLite at `~/.dssh/dssh.db`.

### Package dependency flow

```
cmd/dssh/main.go → cli.Execute()
                      ↓
              internal/cli/     (Cobra commands: add, rm, list, wizard, reset, root)
              ↙        ↘
   internal/tui/    internal/ssh/
   (Bubble Tea UI)  (ssh exec abstraction)
       ↓                ↓
   internal/db/     internal/crypto/
   (SQLite layer)   (AES-256-GCM + Argon2id)
       ↓
   internal/model/
   (Connection struct, AuthType enum)
```

### Key design decisions

- **Pure Go SQLite** (`modernc.org/sqlite`) — enables `CGO_ENABLED=0` static binaries with zero C dependencies
- **Unix process replacement** — key-auth connections use `syscall.Exec` to replace the dssh process with ssh (zero overhead). Password-auth uses a child process with `SSH_ASKPASS`
- **Platform split** — `ssh/exec_unix.go` and `ssh/exec_windows.go` handle OS-specific exec behavior (syscall.Exec vs exec.Command, Setsid)
- **Master passphrase** — never stored; a verification token (`"dssh-verify"` encrypted with the derived key) validates correctness. Salt stored in settings table

### TUI structure (Bubble Tea)

`AppModel` manages three tabs and a passphrase modal overlay:
- **TabConnect** — list picker for saved connections
- **TabNew** — form wizard with auth-type toggle (key vs password, changes visible fields)
- **TabDelete** — list with triple-confirm mechanism (3 Enters on same item within 1 second)
- **PassphraseModal** — create mode (2 fields) on first password save, enter mode (1 field) thereafter

### Database tables

- `connections` — id, name (unique), user, host, port, auth_type, identity_file, encrypted_pass, pass_nonce, timestamps
- `settings` — key-value store for `argon2_salt` and `passphrase_check`

### Crypto flow

Passphrase → Argon2id (time=1, mem=64KB, lanes=4) + stored salt → 256-bit key → AES-256-GCM encrypt/decrypt. Each password gets its own random nonce stored alongside the ciphertext.