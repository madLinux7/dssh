package cli

import (
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing connection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			conns, err := listConnections()
			if err != nil {
				return err
			}

			result := tui.Run(conns, sharedDB, tui.TabEdit, runtimeCfg)
			if result == nil || result.Action == tui.ActionNone {
				return nil
			}

			switch result.Action {
			case tui.ActionConnect:
				return connect(result.Connection, nil)
			case tui.ActionCreated:
				return savePasswordAuth(sharedDB, result.WizardResult)
			}
			return nil
		},
	}
}