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
			cfg := tui.RunConfigDialog()
			if cfg == nil {
				fmt.Println("Configuration cancelled.")
				return nil
			}
			if err := saveRuntimeConfig(sharedDB, cfg); err != nil {
				return err
			}
			runtimeCfg = cfg
			success("Mode set to %s", model.ParseModeLabel(cfg.ParseMode))
			return nil
		},
	}

	cmd.AddCommand(newConfigModeCmd())
	return cmd
}

func newConfigModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode",
		Short: "Show the current configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtimeCfg == nil {
				fmt.Println("Not configured yet. Run 'dssh config' to set up.")
				return nil
			}
			fmt.Printf("parse_mode:                    %s\n", runtimeCfg.ParseMode)
			fmt.Printf("ssh_config_parse_target:        %s\n", runtimeCfg.SSHConfigTarget)
			if runtimeCfg.ParseMode == model.ParseModeBoth {
				fmt.Printf("parse_both_mode:               %s\n", runtimeCfg.BothMode)
				fmt.Printf("parse_both_default_save_target: %s\n", runtimeCfg.DefaultSaveTarget)
			}
			return nil
		},
	}
}
