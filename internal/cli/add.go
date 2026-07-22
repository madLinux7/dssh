package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var port int
	var directory string
	var proxyJump string
	var addToSQLite bool
	var addToSSHConfig bool
	var groupNames []string

	cmd := &cobra.Command{
		Use:   "add [-p PORT] [-d DIR] [-J JUMP] NAME target [password]",
		Short: "Add a new SSH connection",
		Long: `Add a new SSH connection. Target can be user@host or ssh://user@host:port.
If a password is provided, the connection uses password authentication
and the password is encrypted with your master passphrase.`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := model.ValidateName(name); err != nil {
				return err
			}
			target := args[1]
			hasPassword := len(args) == 3

			user, host, parsedPort, err := parseTarget(target)
			if err != nil {
				return err
			}

			// -p flag takes precedence over URI port.
			if cmd.Flags().Changed("port") {
				parsedPort = port
			} else if parsedPort == 0 {
				parsedPort = 22
			}

			conn := &model.Connection{
				Name:      name,
				User:      user,
				Host:      host,
				Port:      parsedPort,
				Directory: directory,
				ProxyJump: proxyJump,
				AuthType:  model.AuthKey,
			}

			// Determine save target.
			saveTarget := resolveAddTarget(addToSQLite, addToSSHConfig, hasPassword)
			if saveTarget == "" {
				return nil // user aborted
			}
			// Resolve every group before creating a connection in either backend.
			groupIDs, err := db.GroupIDsByNames(sharedDB, groupNames)
			if err != nil {
				return err
			}

			// Password handling.
			if hasPassword {
				if saveTarget == model.SaveTargetSSHConfig {
					errMsg("Passwords can only be saved to SQLite. Use %s or %s mode.",
						magentaText("--sqlite"), magentaText("sqlite_only"))
					return nil
				}
				password := args[2]
				conn.AuthType = model.AuthPassword
				encPass, nonce, err := encryptPassword(sharedDB, password)
				if err != nil {
					return err
				}
				conn.EncryptedPass = encPass
				conn.PassNonce = nonce
			}

			// Cross-source duplicate check in "both" mode.
			if runtimeCfg.ParseMode == model.ParseModeBoth {
				if err := checkCrossSourceDuplicate(name, saveTarget); err != nil {
					return err
				}
			}

			// Save to the chosen target.
			if saveTarget == model.SaveTargetSSHConfig {
				p, err := sshConfigPath()
				if err != nil {
					return err
				}
				if err := sshconfig.Insert(p, conn); err != nil {
					return err
				}
				if len(groupIDs) > 0 {
					ref, err := sshConfigMembershipRef(name)
					if err != nil {
						if rollbackErr := sshconfig.Delete(p, name); rollbackErr != nil {
							return fmt.Errorf("resolve ssh_config group assignment: %w (connection rollback failed: %v)", err, rollbackErr)
						}
						return err
					}
					if err := db.SetConnectionGroups(sharedDB, ref, groupIDs); err != nil {
						if rollbackErr := sshconfig.Delete(p, name); rollbackErr != nil {
							return fmt.Errorf("assign groups to ssh_config connection: %w (connection rollback failed: %v)", err, rollbackErr)
						}
						return fmt.Errorf("assign groups to ssh_config connection: %w", err)
					}
				}
				success("Added connection %q to ssh_config (%s@%s:%d)", name, user, host, parsedPort)
			} else {
				ref := model.ConnectionRef{Source: model.SourceSQLite, Name: name}
				if err := db.InsertWithGroups(sharedDB, conn, ref, groupIDs); err != nil {
					return err
				}
				success("Added connection %q to SQLite (%s@%s:%d)", name, user, host, parsedPort)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 22, "SSH port")
	cmd.Flags().StringVarP(&directory, "directory", "d", "", "Remote directory to cd into on connect")
	cmd.Flags().StringVar(&directory, "cd", "", "Alias for --directory")
	cmd.Flags().MarkHidden("cd")
	cmd.Flags().StringVarP(&proxyJump, "proxy-jump", "J", "", "ProxyJump host (user@bastion / host1,host2)")
	cmd.Flags().BoolVar(&addToSQLite, "sqlite", false, "Save to SQLite")
	cmd.Flags().BoolVar(&addToSSHConfig, "sshconfig", false, "Save to ssh_config")
	cmd.Flags().StringSliceVar(&groupNames, "group", nil, "Assign to an existing group (repeatable)")
	return cmd
}

// resolveAddTarget determines where to save based on flags and mode.
// Returns empty string if user aborted.
func resolveAddTarget(flagSQL, flagSSH, hasPassword bool) model.SaveTarget {
	mode := runtimeCfg.ParseMode

	// Explicit flags take priority.
	if flagSQL {
		return model.SaveTargetSQLite
	}
	if flagSSH {
		return model.SaveTargetSSHConfig
	}

	switch mode {
	case model.ParseModeSSHConfigOnly:
		return model.SaveTargetSSHConfig
	case model.ParseModeBoth:
		if hasPassword {
			fmt.Println("Passwords can only be saved to SQLite.")
			choice := radioPrompt("Save to SQLite?", []string{"Yes", "Abort"})
			if choice == 0 {
				return model.SaveTargetSQLite
			}
			return ""
		}
		// Prompt with radio buttons, pre-select the default.
		options := []string{"SQLite", "ssh_config"}
		choice := radioPrompt("Save to:", options)
		switch choice {
		case 0:
			return model.SaveTargetSQLite
		case 1:
			return model.SaveTargetSSHConfig
		default:
			return ""
		}
	default: // sqlite_only
		return model.SaveTargetSQLite
	}
}

// checkCrossSourceDuplicate warns if the name exists in the other source.
func checkCrossSourceDuplicate(name string, target model.SaveTarget) error {
	sqlConn, sshConn, err := getConnectionSources(name)
	if err != nil {
		return err
	}
	if sqlConn != nil && sshConn != nil {
		return fmt.Errorf("connection %q already exists in both SQLite and ssh_config", name)
	}
	if target == model.SaveTargetSQLite && sshConn != nil {
		fmt.Printf("Connection %q already exists in ssh_config.\n", name)
		choice := radioPrompt("Save to SQLite anyway?", []string{"Yes", "Abort"})
		if choice != 0 {
			return fmt.Errorf("aborted")
		}
	}
	if target == model.SaveTargetSSHConfig && sqlConn != nil {
		fmt.Printf("Connection %q already exists in SQLite.\n", name)
		choice := radioPrompt("Save to ssh_config anyway?", []string{"Yes", "Abort"})
		if choice != 0 {
			return fmt.Errorf("aborted")
		}
	}
	return nil
}

func magentaText(s string) string {
	return fmt.Sprintf("\033[35m%s\033[0m", s)
}

// parseTarget parses user@host or ssh://user@host:port into components.
func parseTarget(target string) (user, host string, port int, err error) {
	// Try ssh:// URI first.
	if strings.HasPrefix(target, "ssh://") {
		u, parseErr := url.Parse(target)
		if parseErr != nil {
			return "", "", 0, fmt.Errorf("invalid SSH URI: %w", parseErr)
		}
		if u.User == nil || u.User.Username() == "" {
			return "", "", 0, fmt.Errorf("SSH URI must include user (ssh://user@host)")
		}
		user = u.User.Username()
		host = u.Hostname()
		if host == "" {
			return "", "", 0, fmt.Errorf("SSH URI must include host")
		}
		if p := u.Port(); p != "" {
			port, err = strconv.Atoi(p)
			if err != nil {
				return "", "", 0, fmt.Errorf("invalid port in URI: %w", err)
			}
		}
		return user, host, port, nil
	}

	// user@host format.
	parts := strings.SplitN(target, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", 0, fmt.Errorf("target must be user@host or ssh://user@host:port")
	}

	return parts[0], parts[1], 0, nil
}
