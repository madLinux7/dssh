package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/madLinux7/dssh-launcher/internal/db"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all saved connections",
		Args:    cobra.NoArgs,
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

			if len(conns) == 0 {
				fmt.Println("No connections saved. Use 'dssh add' or 'dssh wizard' to create one.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tUSER\tHOST\tPORT\tAUTH")
			for _, c := range conns {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", c.Name, c.User, c.Host, c.Port, c.AuthType)
			}
			return w.Flush()
		},
	}
}
