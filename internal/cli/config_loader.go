package cli

import (
	"database/sql"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

// Setting keys for the RuntimeConfig values.
const (
	settingParseMode              = "parse_mode"
	settingParseBothViewMode      = "parse_both_view_mode"
	settingParseBothDefaultTarget = "parse_both_default_save_target"
	settingSSHConfigDest          = "ssh_config_parse_destination"
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
		DefaultSaveTarget: model.SaveTargetSQLite,
		BothViewMode:      model.SourceSQLite,
	}

	if v, err := db.GetSetting(d, settingParseBothViewMode); err == nil && v != nil {
		cfg.BothViewMode = model.Source(v)
	}
	if v, err := db.GetSetting(d, settingParseBothDefaultTarget); err == nil && v != nil {
		cfg.DefaultSaveTarget = model.SaveTarget(v)
	}
	if v, err := db.GetSetting(d, settingSSHConfigDest); err == nil && v != nil {
		cfg.SSHConfigDest = string(v)
	}

	return cfg, nil
}

// saveRuntimeConfig writes all config keys to the settings table.
func saveRuntimeConfig(d *sql.DB, cfg *model.RuntimeConfig) error {
	pairs := []struct {
		key   string
		value string
	}{
		{settingParseMode, string(cfg.ParseMode)},
		{settingParseBothViewMode, string(cfg.BothViewMode)},
		{settingParseBothDefaultTarget, string(cfg.DefaultSaveTarget)},
		{settingSSHConfigDest, cfg.SSHConfigDest},
	}
	for _, p := range pairs {
		if err := db.SetSetting(d, p.key, []byte(p.value)); err != nil {
			return err
		}
	}
	return nil
}
