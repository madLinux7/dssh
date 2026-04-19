package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/madLinux7/dssh/internal/model"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all saved connections",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			conns, err := listConnections()
			if err != nil {
				return err
			}

			if len(conns) == 0 {
				fmt.Println("No connections saved. Use 'dssh add [name] [user@host]' or 'dssh create' to add one.")
				return nil
			}

			showSource := runtimeCfg != nil && runtimeCfg.ParseMode == model.ParseModeBoth

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if showSource {
				fmt.Fprintln(w, "NAME\tUSER\tHOST\tPORT\tAUTH\tDIR\tJUMP\tSOURCE")
			} else {
				fmt.Fprintln(w, "NAME\tUSER\tHOST\tPORT\tAUTH\tDIR\tJUMP")
			}
			for _, c := range conns {
				dir := c.Directory
				if dir == "" {
					dir = "-"
				}
				jump := c.ProxyJump
				if jump == "" {
					jump = "-"
				}
				if showSource {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n", c.Name, c.User, c.Host, c.Port, c.AuthType, dir, jump, c.Source)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n", c.Name, c.User, c.Host, c.Port, c.AuthType, dir, jump)
				}
			}
			return w.Flush()
		},
	}
}
