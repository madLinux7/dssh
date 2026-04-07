package cli

import (
	"database/sql"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

// Setting keys for the RuntimeConfig values.
const (
	settingParseMode              = "parse_mode"
	settingParseBothMode          = "parse_both_mode"
	settingParseBothDefaultTarget = "parse_both_default_save_target"
	settingSSHConfigParseTarget   = "ssh_config_parse_target"
)

// loadRuntimeConfig reads the runtime configuration from the settings table.
// Returns nil if parse_mode is not set (signals first-run).
func loadRuntimeConfig(d *sql.DB) (*model.RuntimeConfig, error) {
	raw, err := db.GetSetting(d, settingParseMode)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil // first-run
	}

	cfg := &model.RuntimeConfig{
		ParseMode:         model.ParseMode(raw),
		BothMode:          model.BothModeSeparate,
		DefaultSaveTarget: model.SaveTargetSQLite,
		SSHConfigTarget:   model.SSHConfigTargetMainFile,
	}

	if v, err := db.GetSetting(d, settingParseBothMode); err == nil && v != nil {
		cfg.BothMode = model.BothMode(v)
	}
	if v, err := db.GetSetting(d, settingParseBothDefaultTarget); err == nil && v != nil {
		cfg.DefaultSaveTarget = model.SaveTarget(v)
	}
	if v, err := db.GetSetting(d, settingSSHConfigParseTarget); err == nil && v != nil {
		cfg.SSHConfigTarget = model.SSHConfigTarget(v)
	}

	return cfg, nil
}

// saveRuntimeConfig writes all four config keys to the settings table.
func saveRuntimeConfig(d *sql.DB, cfg *model.RuntimeConfig) error {
	pairs := []struct {
		key   string
		value string
	}{
		{settingParseMode, string(cfg.ParseMode)},
		{settingParseBothMode, string(cfg.BothMode)},
		{settingParseBothDefaultTarget, string(cfg.DefaultSaveTarget)},
		{settingSSHConfigParseTarget, string(cfg.SSHConfigTarget)},
	}
	for _, p := range pairs {
		if err := db.SetSetting(d, p.key, []byte(p.value)); err != nil {
			return err
		}
	}
	return nil
}
