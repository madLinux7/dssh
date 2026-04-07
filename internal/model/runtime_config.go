package model

// RuntimeConfig holds the active configuration for the current session.
type RuntimeConfig struct {
	ParseMode         ParseMode
	BothMode          BothMode
	DefaultSaveTarget SaveTarget
	SSHConfigTarget   SSHConfigTarget
	FlagOverride      bool // true when --sqlite/--sshconfig/--both was used
}
