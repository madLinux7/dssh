package cli

import (
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing connection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()

			conns, err := db.List(d)
			if err != nil {
				return err
			}

			result := tui.Run(conns, d, tui.TabEdit)
			if result == nil || result.Action == tui.ActionNone {
				return nil
			}

			switch result.Action {
			case tui.ActionConnect:
				return connect(result.Connection, nil)
			case tui.ActionCreated:
				return savePasswordAuth(d, result.WizardResult)
			}
			return nil
		},
	}
}