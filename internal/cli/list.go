package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/spf13/cobra"
)

type connectionJSON struct {
	Name         string         `json:"name"`
	User         string         `json:"user"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	Directory    string         `json:"directory"`
	AuthType     model.AuthType `json:"auth_type"`
	IdentityFile string         `json:"identity_file"`
	ProxyJump    string         `json:"proxy_jump"`
	Source       model.Source   `json:"source"`
	Groups       []string       `json:"groups"`
}

func newListCmd() *cobra.Command {
	var groupFilters []string
	var ungrouped, asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all saved connections",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			conns, err := listConnections()
			if err != nil {
				return err
			}
			if ungrouped && len(groupFilters) > 0 {
				return fmt.Errorf("--ungrouped cannot be used with --group")
			}
			membershipByConnection, err := scopedGroupNames(conns)
			if err != nil {
				return err
			}
			conns = filterConnectionsByGroups(conns, membershipByConnection, groupFilters, ungrouped)

			if asJSON {
				output := make([]connectionJSON, 0, len(conns))
				for _, c := range conns {
					output = append(output, connectionJSON{Name: c.Name, User: c.User, Host: c.Host, Port: c.Port, Directory: c.Directory, AuthType: c.AuthType, IdentityFile: c.IdentityFile, ProxyJump: c.ProxyJump, Source: c.Source, Groups: membershipByConnection[connectionScopeKey(c)]})
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			if len(conns) == 0 {
				if len(groupFilters) > 0 {
					return nil
				}
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
	cmd.Flags().StringSliceVar(&groupFilters, "group", nil, "Only connections in this group (repeatable)")
	cmd.Flags().BoolVar(&ungrouped, "ungrouped", false, "Only connections with no groups")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}

func scopedGroupNames(conns []model.Connection) (map[string][]string, error) {
	result := make(map[string][]string, len(conns))
	needSQLite, needSSH := false, false
	for _, connection := range conns {
		if connection.Source == model.SourceSSHConfig {
			needSSH = true
		} else {
			needSQLite = true
		}
	}
	if needSQLite {
		groups, err := db.GroupNamesForConnections(sharedDB, model.SourceSQLite, "")
		if err != nil {
			return nil, err
		}
		for name, values := range groups {
			result[string(model.SourceSQLite)+"\x00"+name] = values
		}
	}
	if needSSH {
		ref, err := sshConfigMembershipRef("")
		if err != nil {
			return nil, err
		}
		groups, err := db.GroupNamesForConnections(sharedDB, model.SourceSSHConfig, ref.SourcePath)
		if err != nil {
			return nil, err
		}
		for name, values := range groups {
			result[string(model.SourceSSHConfig)+"\x00"+name] = values
		}
	}
	return result, nil
}

func connectionScopeKey(connection model.Connection) string {
	return string(connection.Source) + "\x00" + connection.Name
}

func filterConnectionsByGroups(conns []model.Connection, memberships map[string][]string, filters []string, ungrouped bool) []model.Connection {
	filterSet := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		_, normalized, err := db.NormalizeGroupName(filter)
		if err == nil {
			filterSet[normalized] = struct{}{}
		}
	}
	filtered := make([]model.Connection, 0, len(conns))
	for _, connection := range conns {
		groups := memberships[connectionScopeKey(connection)]
		if ungrouped {
			if len(groups) == 0 {
				filtered = append(filtered, connection)
			}
			continue
		}
		if len(filterSet) == 0 {
			filtered = append(filtered, connection)
			continue
		}
		for _, group := range groups {
			if _, ok := filterSet[strings.ToLower(group)]; ok {
				filtered = append(filtered, connection)
				break
			}
		}
	}
	return filtered
}
