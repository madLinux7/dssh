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

	cmd := &cobra.Command{
		Use:   "add [-p PORT] NAME target",
		Short: "Add a new SSH connection",
		Long:  `Add a new SSH connection. Target can be user@host or ssh://user@host:port.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
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
				Name:     name,
				User:     user,
				Host:     host,
				Port:     parsedPort,
				AuthType: model.AuthKey,
			}

			if err := db.Insert(d, conn); err != nil {
				return err
			}

			success("Added connection %q (%s@%s:%d)", name, user, host, parsedPort)
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 22, "SSH port")
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
