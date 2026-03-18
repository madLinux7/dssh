package cli

import (
	"github.com/madLinux7/dssh-launcher/internal/db"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a saved connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()

			if err := db.Delete(d, args[0]); err != nil {
				return err
			}

			success("Removed connection %q", args[0])
			return nil
		},
	}
}
