// Package cli implements the Cobra command tree and orchestrates the flow
// between the TUI, database, crypto, and SSH layers.
package cli

import (
	"os"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/ssh"
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
)

// Execute builds and runs the root command.
func Execute(version string) {
	root := newRootCmd(version)
	if err := root.Execute(); err != nil {
		errMsg("%s", err)
		os.Exit(1)
	}
}

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "dssh [NAME] [-- extra-ssh-args...]",
		Short:   "Dead Simple SSH Launcher",
		Version: version,
		// Disable default completion command.
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		// Allow pass-through args after --.
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: false,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Split args at "--" to get connection name and extra ssh args.
			name, extraArgs := splitArgs(args)

			if name == "" {
				return runPicker(extraArgs)
			}
			return connectByName(name, extraArgs)
		},
	}

	// Allow unknown flags to pass through after --.
	root.Flags().SetInterspersed(false)

	root.AddCommand(
		newAddCmd(),
		newRmCmd(),
		newListCmd(),
		newWizardCmd(),
		newResetCmd(),
	)

	return root
}

func splitArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	name := args[0]
	var extra []string
	if len(args) > 1 {
		extra = args[1:]
	}
	return name, extra
}

func runPicker(extraArgs []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	conns, err := db.List(d)
	if err != nil {
		return err
	}

	initialTab := tui.TabConnect
	if len(conns) == 0 {
		initialTab = tui.TabNew
	}

	result := tui.Run(conns, d, initialTab)
	if result == nil {
		return nil
	}

	switch result.Action {
	case tui.ActionConnect:
		return connect(result.Connection, extraArgs)
	case tui.ActionCreated:
		return savePasswordAuth(d, result.WizardResult)
	}
	return nil
}

func connectByName(name string, extraArgs []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	conn, err := db.GetByName(d, name)
	if err != nil {
		return err
	}

	return connect(conn, extraArgs)
}

// connect dispatches to the appropriate SSH method based on auth type.
// Key auth replaces the process via syscall.Exec; password auth runs ssh
// as a child process with SSH_ASKPASS to supply the decrypted password.
func connect(conn *model.Connection, extraArgs []string) error {
	if conn.AuthType == model.AuthPassword {
		return connectPassword(conn, extraArgs)
	}
	// Key auth — exec replaces the process, db gets cleaned up by OS.
	return ssh.ConnectWithKey(conn, extraArgs)
}

func connectPassword(conn *model.Connection, extraArgs []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	password, err := decryptPassword(d, conn)
	if err != nil {
		return err
	}

	return ssh.ConnectWithPassword(conn, password, extraArgs)
}
