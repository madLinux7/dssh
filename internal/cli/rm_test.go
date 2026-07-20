package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"

	_ "modernc.org/sqlite"
)

func TestDeleteSSHConfigConnectionKeepsEntryWhenMetadataCleanupFails(t *testing.T) {
	previousDB, previousCfg := sharedDB, runtimeCfg
	t.Cleanup(func() {
		sharedDB, runtimeCfg = previousDB, previousCfg
	})

	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	connection := &model.Connection{Name: "api", Host: "api.example", User: "root", Port: 22, AuthType: model.AuthKey}
	if err := sshconfig.Insert(path, connection); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	sharedDB = d
	runtimeCfg = &model.RuntimeConfig{SSHConfigDest: path}

	if err := deleteSSHConfigConnection(path, connection.Name); err == nil {
		t.Fatal("delete succeeded despite metadata cleanup failure")
	}
	if _, err := sshconfig.GetByName(path, connection.Name); err != nil {
		t.Fatalf("SSH entry was removed before metadata cleanup succeeded: %v", err)
	}
}
