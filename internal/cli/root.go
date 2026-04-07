// Package cli implements the Cobra command tree and orchestrates the flow
// between the TUI, database, crypto, and SSH layers.
package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/ssh"
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
)

// Package-level shared state — initialised in PersistentPreRunE.
var (
	sharedDB   *sql.DB
	runtimeCfg *model.RuntimeConfig

	// Flag overrides (one-shot, not persisted).
	flagSQLite    bool
	flagSSHConfig bool
	flagBoth      bool
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
		PersistentPreRunE:  persistentPreRun,
		PersistentPostRunE: persistentPostRun,
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

	// Mode override flags (one-shot, not persisted).
	root.PersistentFlags().BoolVar(&flagSQLite, "sqlite", false, "use SQLite mode for this session")
	root.PersistentFlags().BoolVar(&flagSSHConfig, "sshconfig", false, "use ssh_config mode for this session")
	root.PersistentFlags().BoolVar(&flagBoth, "both", false, "use both mode for this session")

	root.AddCommand(
		newAddCmd(),
		newRmCmd(),
		newListCmd(),
		newCreateCmd(),
		newEditCmd(),
		newDeleteCmd(),
		newResetCmd(),
		newConfigCmd(),
	)

	return root
}

// persistentPreRun opens the DB and loads (or bootstraps) the runtime config.
func persistentPreRun(cmd *cobra.Command, args []string) error {
	// Reset command manages the DB file itself — skip shared setup.
	if cmd.Name() == "reset" {
		return nil
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	sharedDB = d

	// Check flag overrides.
	if flagSQLite || flagSSHConfig || flagBoth {
		runtimeCfg = &model.RuntimeConfig{FlagOverride: true}
		switch {
		case flagSQLite:
			runtimeCfg.ParseMode = model.ParseModeSQLiteOnly
		case flagSSHConfig:
			runtimeCfg.ParseMode = model.ParseModeSSHConfigOnly
		case flagBoth:
			runtimeCfg.ParseMode = model.ParseModeBoth
			runtimeCfg.BothMode = model.BothModeSeparate
			runtimeCfg.DefaultSaveTarget = model.SaveTargetSQLite
		}
		return nil
	}

	cfg, err := loadRuntimeConfig(sharedDB)
	if err != nil {
		return err
	}

	if cfg == nil {
		// First run — show config dialog (unless this IS the config command).
		if cmd.Name() == "config" || (cmd.Parent() != nil && cmd.Parent().Name() == "config") {
			return nil
		}
		cfg = tui.RunConfigDialog()
		if cfg == nil {
			fmt.Println("Setup cancelled.")
			os.Exit(0)
		}
		if err := saveRuntimeConfig(sharedDB, cfg); err != nil {
			return err
		}
	}

	runtimeCfg = cfg
	return nil
}

func persistentPostRun(cmd *cobra.Command, args []string) error {
	if sharedDB != nil {
		sharedDB.Close()
		sharedDB = nil
	}
	return nil
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
	conns, err := listConnections()
	if err != nil {
		return err
	}

	initialTab := tui.TabConnect
	if len(conns) == 0 {
		initialTab = tui.TabCreate
	}

	result := tui.Run(conns, sharedDB, initialTab, runtimeCfg)
	if result == nil {
		return nil
	}

	switch result.Action {
	case tui.ActionConnect:
		return connect(result.Connection, extraArgs)
	case tui.ActionCreated:
		return savePasswordAuth(sharedDB, result.WizardResult)
	}
	return nil
}

func connectByName(name string, extraArgs []string) error {
	conn, err := getConnectionByName(name)
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
	password, err := decryptPassword(sharedDB, conn)
	if err != nil {
		return err
	}
	return ssh.ConnectWithPassword(conn, password, extraArgs)
}
