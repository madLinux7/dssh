package model

import (
	"errors"
	"testing"
)

func TestValidateNameRejectsReserved(t *testing.T) {
	reserved := []string{"add", "rm", "list", "ls", "new", "create", "edit", "delete", "reset", "help", "config", "group", "connect"}
	for _, name := range reserved {
		if err := ValidateName(name); err == nil {
			t.Errorf("expected error for reserved name %q", name)
		} else if !errors.Is(err, ErrReservedName) {
			t.Errorf("expected ErrReservedName for %q, got: %v", name, err)
		}
	}
}

func TestValidateNameAcceptsNormal(t *testing.T) {
	names := []string{"myserver", "dev1", "prod-db", "staging_web"}
	for _, name := range names {
		if err := ValidateName(name); err != nil {
			t.Errorf("unexpected error for %q: %v", name, err)
		}
	}
}

func TestValidateNameCaseInsensitive(t *testing.T) {
	if err := ValidateName("ADD"); err == nil {
		t.Error("expected error for uppercase reserved name")
	}
	if err := ValidateName("Config"); err == nil {
		t.Error("expected error for mixed-case reserved name")
	}
}

func TestParseModeLabel(t *testing.T) {
	tests := []struct {
		mode ParseMode
		want string
	}{
		{ParseModeSQLiteOnly, "SQLite"},
		{ParseModeSSHConfigOnly, "ssh_config"},
		{ParseModeBoth, "SQLite + ssh_config"},
	}
	for _, tt := range tests {
		got := ParseModeLabel(tt.mode)
		if got != tt.want {
			t.Errorf("ParseModeLabel(%q) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestConnectionDisplayLabel(t *testing.T) {
	c := Connection{Name: "dev", User: "admin", Host: "example.com", Port: 22}
	if got := c.DisplayLabel(); got != "dev — admin@example.com" {
		t.Errorf("got %q", got)
	}

	c.Port = 2222
	if got := c.DisplayLabel(); got != "dev — admin@example.com:2222" {
		t.Errorf("got %q", got)
	}
}

func TestConnectionSSHTarget(t *testing.T) {
	c := Connection{User: "root", Host: "10.0.0.1"}
	if got := c.SSHTarget(); got != "root@10.0.0.1" {
		t.Errorf("got %q", got)
	}
}
