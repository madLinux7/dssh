package cli

import (
	"fmt"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			switch runtimeCfg.ParseMode {
			case model.ParseModeSSHConfigOnly:
				p, err := sshConfigPath()
				if err != nil {
					return err
				}
				if err := sshconfig.Delete(p, name); err != nil {
					return err
				}
				success("Removed connection %q from ssh_config", name)
				return nil

			case model.ParseModeBoth:
				sqlConn, sshConn, err := getConnectionSources(name)
				if err != nil {
					return err
				}
				if sqlConn == nil && sshConn == nil {
					return fmt.Errorf("connection %q not found", name)
				}
				if sqlConn != nil && sshConn != nil {
					return rmBothPrompt(name)
				}
				if sqlConn != nil {
					if err := db.Delete(sharedDB, name); err != nil {
						return err
					}
					success("Removed connection %q from SQLite", name)
					return nil
				}
				p, err := sshConfigPath()
				if err != nil {
					return err
				}
				if err := sshconfig.Delete(p, name); err != nil {
					return err
				}
				success("Removed connection %q from ssh_config", name)
				return nil

			default: // sqlite_only
				if err := db.Delete(sharedDB, name); err != nil {
					return err
				}
				success("Removed connection %q", name)
				return nil
			}
		},
	}
}

func rmBothPrompt(name string) error {
	choice := radioPrompt(
		"This connection exists in both SQLite & ssh_config.\nWhich one do you want to remove?",
		[]string{"SQLite", "ssh_config", "Both", "Abort"},
	)

	switch choice {
	case 0: // SQLite
		if err := db.Delete(sharedDB, name); err != nil {
			return err
		}
		success("Removed connection %q from SQLite", name)
	case 1: // ssh_config
		p, err := sshConfigPath()
		if err != nil {
			return err
		}
		if err := sshconfig.Delete(p, name); err != nil {
			return err
		}
		success("Removed connection %q from ssh_config", name)
	case 2: // Both
		if err := db.Delete(sharedDB, name); err != nil {
			return err
		}
		p, err := sshConfigPath()
		if err != nil {
			return err
		}
		if err := sshconfig.Delete(p, name); err != nil {
			return err
		}
		success("Removed connection %q from both SQLite and ssh_config", name)
	default: // Abort or cancel
		fmt.Println("Aborted.")
	}
	return nil
}
