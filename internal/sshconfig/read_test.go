package sshconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madLinux7/dssh/internal/model"
)

const testConfig = `Host *
    ServerAliveInterval 60

Host dev1
    HostName dev.example.com
    User john
    Port 2322

Host dev2
    HostName dev2.example.com
    User admin
    IdentityFile ~/.ssh/key2.key

Host dev3
    HostName dev3.example.com
    User deploy
    Port 2222
    RemoteCommand cd '/data/music' && exec $SHELL -l
    RequestTTY force

Host *.example.com
    User fallback
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListBasic(t *testing.T) {
	p := writeTempConfig(t, testConfig)
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}

	if len(conns) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(conns))
	}

	// dev1
	c := conns[0]
	if c.Name != "dev1" || c.Host != "dev.example.com" || c.User != "john" || c.Port != 2322 {
		t.Errorf("dev1 mismatch: %+v", c)
	}
	if c.Source != model.SourceSSHConfig {
		t.Errorf("expected Source=ssh_config, got %q", c.Source)
	}

	// dev2
	c = conns[1]
	if c.Name != "dev2" || c.Host != "dev2.example.com" || c.User != "admin" || c.Port != 22 {
		t.Errorf("dev2 mismatch: %+v", c)
	}
	if c.IdentityFile != "~/.ssh/key2.key" {
		t.Errorf("dev2 identity file: got %q", c.IdentityFile)
	}

	// dev3 with directory
	c = conns[2]
	if c.Name != "dev3" || c.Directory != "/data/music" {
		t.Errorf("dev3 mismatch: %+v", c)
	}
	if c.AuthType != model.AuthKey {
		t.Errorf("expected AuthKey, got %q", c.AuthType)
	}
}

func TestListSkipsWildcards(t *testing.T) {
	p := writeTempConfig(t, testConfig)
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range conns {
		if c.Name == "*" || c.Name == "*.example.com" {
			t.Errorf("wildcard host not skipped: %q", c.Name)
		}
	}
}

func TestListDefaultUser(t *testing.T) {
	cfg := "Host nouser\n    HostName example.com\n"
	p := writeTempConfig(t, cfg)
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].User != "root" {
		t.Errorf("expected default user root, got %+v", conns)
	}
}

func TestListRemoteCommandWithoutQuotes(t *testing.T) {
	cfg := "Host test\n    HostName example.com\n    RemoteCommand cd /var/log && exec $SHELL -l\n    RequestTTY force\n"
	p := writeTempConfig(t, cfg)
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Directory != "/var/log" {
		t.Errorf("expected directory /var/log, got %+v", conns)
	}
}

func TestGetByName(t *testing.T) {
	p := writeTempConfig(t, testConfig)
	c, err := GetByName(p, "dev2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "dev2" {
		t.Errorf("expected dev2, got %q", c.Name)
	}
}

func TestGetByNameCaseInsensitive(t *testing.T) {
	p := writeTempConfig(t, testConfig)
	c, err := GetByName(p, "DEV1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "dev1" {
		t.Errorf("expected dev1, got %q", c.Name)
	}
}

func TestGetByNameNotFound(t *testing.T) {
	p := writeTempConfig(t, testConfig)
	_, err := GetByName(p, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent host")
	}
}

func TestExists(t *testing.T) {
	p := writeTempConfig(t, testConfig)

	ok, err := Exists(p, "dev1")
	if err != nil || !ok {
		t.Errorf("dev1 should exist")
	}

	ok, err = Exists(p, "nope")
	if err != nil || ok {
		t.Errorf("nope should not exist")
	}
}

func TestListFileNotFound(t *testing.T) {
	_, err := List("/nonexistent/path/config")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestListEmptyFile(t *testing.T) {
	p := writeTempConfig(t, "")
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 0 {
		t.Errorf("expected 0 connections from empty file, got %d", len(conns))
	}
}

func TestParseKeyValueWithEquals(t *testing.T) {
	cfg := "Host eqtest\n    HostName=eq.example.com\n    User=admin\n    Port=3322\n"
	p := writeTempConfig(t, cfg)
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	c := conns[0]
	if c.Host != "eq.example.com" || c.User != "admin" || c.Port != 3322 {
		t.Errorf("equals-separated config mismatch: %+v", c)
	}
}
