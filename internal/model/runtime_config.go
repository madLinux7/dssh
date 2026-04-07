package model

// RuntimeConfig holds the active configuration for the current session.
type RuntimeConfig struct {
	ParseMode         ParseMode
	DefaultSaveTarget SaveTarget
	SSHConfigDest     string // absolute path to the ssh_config file
	BothViewMode      Source // which source list to show initially in "both" mode
	FlagOverride      bool   // true when --sqlite/--sshconfig/--both was used
}
