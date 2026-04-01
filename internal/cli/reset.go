package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Delete all connections and settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dbPath := filepath.Join(home, ".dssh", "dssh.db")

			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				errMsg("nothing to reset — no database found")
				return nil
			}

			reader := bufio.NewReader(os.Stdin)

			fmt.Print("This will delete ALL saved connections, passwords, and settings. Continue? (yes/no): ")
			ans1, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(ans1)) != "yes" {
				fmt.Println("Aborted.")
				return nil
			}

			fmt.Print("Are you sure? This cannot be undone. Type 'reset' to confirm: ")
			ans2, _ := reader.ReadString('\n')
			if strings.TrimSpace(ans2) != "reset" {
				fmt.Println("Aborted.")
				return nil
			}

			if err := os.Remove(dbPath); err != nil {
				return fmt.Errorf("delete database: %w", err)
			}

			success("All data has been reset")
			return nil
		},
	}
}
