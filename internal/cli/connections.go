package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
)

// listConnections returns connections from the active source(s) per runtimeCfg.
func listConnections() ([]model.Connection, error) {
	switch runtimeCfg.ParseMode {
	case model.ParseModeSSHConfigOnly:
		return listSSHConfigConnections()
	case model.ParseModeBoth:
		sqlConns, err := listSQLiteConnections()
		if err != nil {
			return nil, err
		}
		sshConns, err := listSSHConfigConnections()
		if err != nil {
			return nil, err
		}
		all := append(sqlConns, sshConns...)
		sort.Slice(all, func(i, j int) bool {
			return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
		})
		return all, nil
	default: // sqlite_only
		return listSQLiteConnections()
	}
}

// getConnectionByName finds a connection by name across active source(s).
func getConnectionByName(name string) (*model.Connection, error) {
	switch runtimeCfg.ParseMode {
	case model.ParseModeSSHConfigOnly:
		p, err := sshConfigPath()
		if err != nil {
			return nil, err
		}
		return sshconfig.GetByName(p, name)
	case model.ParseModeBoth:
		// Search SQLite first, then ssh_config.
		conn, err := db.GetByName(sharedDB, name)
		if err == nil {
			conn.Source = model.SourceSQLite
			return conn, nil
		}
		p, err2 := sshConfigPath()
		if err2 != nil {
			return nil, fmt.Errorf("sqlite: %w; ssh_config path: %w", err, err2)
		}
		return sshconfig.GetByName(p, name)
	default: // sqlite_only
		conn, err := db.GetByName(sharedDB, name)
		if err != nil {
			return nil, err
		}
		conn.Source = model.SourceSQLite
		return conn, nil
	}
}

// getConnectionSources checks both sources for a connection name.
// Either return value may be nil.
func getConnectionSources(name string) (sqliteConn *model.Connection, sshcfgConn *model.Connection, err error) {
	sqliteConn, _ = db.GetByName(sharedDB, name)
	if sqliteConn != nil {
		sqliteConn.Source = model.SourceSQLite
	}

	p, pathErr := sshConfigPath()
	if pathErr == nil {
		sshcfgConn, _ = sshconfig.GetByName(p, name)
	}

	return sqliteConn, sshcfgConn, nil
}

// sshConfigPath returns the ssh_config file path based on the current config target.
func sshConfigPath() (string, error) {
	if runtimeCfg == nil {
		return sshconfig.MainFilePath()
	}
	return sshconfig.FilePath(runtimeCfg.SSHConfigTarget)
}

func listSQLiteConnections() ([]model.Connection, error) {
	conns, err := db.List(sharedDB)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		conns[i].Source = model.SourceSQLite
	}
	return conns, nil
}

func listSSHConfigConnections() ([]model.Connection, error) {
	p, err := sshConfigPath()
	if err != nil {
		return nil, err
	}
	return sshconfig.List(p)
}
