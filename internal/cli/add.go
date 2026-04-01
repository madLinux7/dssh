package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var port int
	var directory string

	cmd := &cobra.Command{
		Use:   "add [-p PORT] [-d DIR] NAME target [password]",
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

			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()

			conn := &model.Connection{
				Name:      name,
				User:      user,
				Host:      host,
				Port:      parsedPort,
				Directory: directory,
				AuthType:  model.AuthKey,
			}

			// Optional password argument → password auth.
			if len(args) == 3 {
				password := args[2]
				conn.AuthType = model.AuthPassword

				encPass, nonce, err := encryptPassword(d, password)
				if err != nil {
					return err
				}
				conn.EncryptedPass = encPass
				conn.PassNonce = nonce
			}

			if err := db.Insert(d, conn); err != nil {
				return err
			}

			success("Added connection %q (%s@%s:%d)", name, user, host, parsedPort)
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 22, "SSH port")
	cmd.Flags().StringVarP(&directory, "directory", "d", "", "Remote directory to cd into on connect")
	cmd.Flags().StringVar(&directory, "cd", "", "Alias for --directory")
	cmd.Flags().MarkHidden("cd")
	return cmd
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
