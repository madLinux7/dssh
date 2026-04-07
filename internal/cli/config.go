package cli

import (
	"fmt"

	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/tui"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure dssh connection mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := tui.RunConfigDialog(appVersion)
			if cfg == nil {
				fmt.Println("Configuration cancelled.")
				return nil
			}

			if err := saveRuntimeConfig(sharedDB, cfg); err != nil {
				return err
			}

			runtimeCfg = cfg
			success("Mode set to %s", model.ParseModeLabel(cfg.ParseMode))

			if cfg.SSHConfigDest != "" {
				success("ssh_config file set to: %s", cfg.SSHConfigDest)
			}
			return nil
		},
	}

	cmd.AddCommand(newConfigGetCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get",
		Aliases: []string{"show"},
		Short:   "Show the current configuration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtimeCfg == nil {
				fmt.Println("Not configured yet. Run 'dssh config' to set up.")
				return nil
			}

			fmt.Printf("parse_mode:                      %s\n", runtimeCfg.ParseMode)
			if runtimeCfg.SSHConfigDest != "" {
				
				fmt.Printf("ssh_config_parse_destination:    %s\n", runtimeCfg.SSHConfigDest)
			}
			if runtimeCfg.ParseMode == model.ParseModeBoth {
				fmt.Printf("parse_both_view_mode:            %s\n", runtimeCfg.BothViewMode)
				fmt.Printf("parse_both_default_save_target:  %s\n", runtimeCfg.DefaultSaveTarget)
			}

			return nil
		},
	}
}
