package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
	"github.com/spf13/cobra"
)

type groupJSON struct {
	Name           string `json:"name"`
	SQLiteCount    *int   `json:"sqlite_count"`
	SSHConfigCount *int   `json:"ssh_config_count"`
}

func newGroupCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "group", Short: "Manage connection groups"}
	cmd.AddCommand(newGroupListCmd(), newGroupCreateCmd(), newGroupRenameCmd(), newGroupDeleteCmd(), newGroupAssignCmd("assign", true), newGroupAssignCmd("unassign", false))
	return cmd
}

func newGroupListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List groups and source-scoped membership counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			groups, err := db.ListGroups(sharedDB)
			if err != nil {
				return err
			}
			result := make([]groupJSON, len(groups))
			for i, group := range groups {
				result[i].Name = group.Name
			}
			if runtimeCfg.ParseMode != model.ParseModeSSHConfigOnly {
				counts, err := db.ListGroupsWithCounts(sharedDB, model.SourceSQLite, "")
				if err != nil {
					return err
				}
				for i := range result {
					value := countForGroup(counts, groups[i].ID)
					result[i].SQLiteCount = &value
				}
			}
			if runtimeCfg.ParseMode != model.ParseModeSQLiteOnly {
				path, err := sshConfigMembershipRef("")
				if err != nil {
					return err
				}
				counts, err := db.ListGroupsWithCounts(sharedDB, model.SourceSSHConfig, path.SourcePath)
				if err != nil {
					return err
				}
				for i := range result {
					value := countForGroup(counts, groups[i].ID)
					result[i].SSHConfigCount = &value
				}
			}
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(result)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if runtimeCfg.ParseMode == model.ParseModeBoth {
				fmt.Fprintln(w, "NAME\tSQLITE\tSSH_CONFIG")
			} else if runtimeCfg.ParseMode == model.ParseModeSSHConfigOnly {
				fmt.Fprintln(w, "NAME\tSSH_CONFIG")
			} else {
				fmt.Fprintln(w, "NAME\tSQLITE")
			}
			for _, group := range result {
				switch runtimeCfg.ParseMode {
				case model.ParseModeBoth:
					fmt.Fprintf(w, "%s\t%d\t%d\n", group.Name, *group.SQLiteCount, *group.SSHConfigCount)
				case model.ParseModeSSHConfigOnly:
					fmt.Fprintf(w, "%s\t%d\n", group.Name, *group.SSHConfigCount)
				default:
					fmt.Fprintf(w, "%s\t%d\n", group.Name, *group.SQLiteCount)
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}

func countForGroup(groups []model.GroupWithCount, id int64) int {
	for _, group := range groups {
		if group.ID == id {
			return group.ConnectionCount
		}
	}
	return 0
}

func newGroupCreateCmd() *cobra.Command {
	return &cobra.Command{Use: "create NAME", Short: "Create a group", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		group, err := db.CreateGroup(sharedDB, args[0])
		if err != nil {
			return err
		}
		success("Created group %q", group.Name)
		return nil
	}}
}

func newGroupRenameCmd() *cobra.Command {
	return &cobra.Command{Use: "rename NAME NEW_NAME", Short: "Rename a group", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		group, err := db.GetGroupByName(sharedDB, args[0])
		if err != nil {
			return err
		}
		if err := db.RenameGroup(sharedDB, group.ID, args[1]); err != nil {
			return err
		}
		success("Renamed group %q to %q", group.Name, strings.TrimSpace(args[1]))
		return nil
	}}
}

func newGroupDeleteCmd() *cobra.Command {
	return &cobra.Command{Use: "delete NAME", Short: "Delete a group and all of its memberships", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		group, err := db.GetGroupByName(sharedDB, args[0])
		if err != nil {
			return err
		}
		if err := db.DeleteGroup(sharedDB, group.ID); err != nil {
			return err
		}
		success("Deleted group %q", group.Name)
		return nil
	}}
}

func newGroupAssignCmd(name string, assign bool) *cobra.Command {
	verb := "Assign"
	if !assign {
		verb = "Remove"
	}
	return &cobra.Command{Use: name + " GROUP CONNECTION...", Short: verb + " connections " + map[bool]string{true: "to", false: "from"}[assign] + " a group", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := groupWriteRef(cmd)
		if err != nil {
			return err
		}
		group, err := db.GetGroupByName(sharedDB, args[0])
		if err != nil {
			return err
		}
		refs := make([]model.ConnectionRef, 0, len(args)-1)
		seen := make(map[string]struct{})
		for _, connectionName := range args[1:] {
			if _, ok := seen[connectionName]; ok {
				continue
			}
			seen[connectionName] = struct{}{}
			if err := connectionExistsInScope(connectionName, ref.Source); err != nil {
				return err
			}
			connectionRef := ref
			connectionRef.Name = connectionName
			refs = append(refs, connectionRef)
		}
		if assign {
			err = db.AssignConnections(sharedDB, group.ID, refs)
		} else {
			err = db.UnassignConnections(sharedDB, group.ID, refs)
		}
		if err != nil {
			return err
		}
		action := "Assigned"
		if !assign {
			action = "Removed"
		}
		success("%s %d connection(s) %s group %q", action, len(refs), map[bool]string{true: "to", false: "from"}[assign], group.Name)
		return nil
	}}
}

func groupWriteRef(cmd *cobra.Command) (model.ConnectionRef, error) {
	if runtimeCfg.ParseMode == model.ParseModeBoth {
		return model.ConnectionRef{}, fmt.Errorf("group membership writes in both mode require exactly one of --sqlite or --sshconfig")
	}
	if runtimeCfg.ParseMode == model.ParseModeSSHConfigOnly {
		return sshConfigMembershipRef("")
	}
	return model.ConnectionRef{Source: model.SourceSQLite}, nil
}

func connectionExistsInScope(name string, source model.Source) error {
	if source == model.SourceSQLite {
		_, err := db.GetByName(sharedDB, name)
		return err
	}
	path, err := sshConfigPath()
	if err != nil {
		return err
	}
	_, err = sshconfig.GetByName(path, name)
	return err
}
