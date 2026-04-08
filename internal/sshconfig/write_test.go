package sshconfig

import (
	"os"
	"strings"
	"testing"

	"github.com/madLinux7/dssh/internal/model"
)

func TestInsertToEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/config"
	os.WriteFile(p, []byte(""), 0600)

	conn := &model.Connection{
		Name: "test1",
		Host: "example.com",
		User: "admin",
		Port: 22,
	}
	if err := Insert(p, conn); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(p)
	content := string(data)
	if !strings.Contains(content, "Host test1") {
		t.Errorf("expected Host test1 in output:\n%s", content)
	}
	if !strings.Contains(content, "    HostName example.com") {
		t.Errorf("expected indented HostName:\n%s", content)
	}
}

func TestInsertAfterExistingHosts(t *testing.T) {
	initial := "Host existing\n    HostName exist.com\n    User root\n\nHost *\n    ServerAliveInterval 60\n"
	p := writeTempConfig(t, initial)

	conn := &model.Connection{
		Name: "new1",
		Host: "new.example.com",
		User: "deploy",
		Port: 2222,
	}
	if err := Insert(p, conn); err != nil {
		t.Fatal(err)
	}

	// Verify the new block is between 'existing' and '*'
	data, _ := os.ReadFile(p)
	content := string(data)
	existingIdx := strings.Index(content, "Host existing")
	newIdx := strings.Index(content, "Host new1")
	wildcardIdx := strings.Index(content, "Host *")

	if newIdx < existingIdx || newIdx > wildcardIdx {
		t.Errorf("new host should be between existing and wildcard:\n%s", content)
	}

	// Port should be present since != 22
	if !strings.Contains(content, "    Port 2222") {
		t.Errorf("expected Port 2222:\n%s", content)
	}
}

func TestInsertWithDirectory(t *testing.T) {
	p := writeTempConfig(t, "")
	conn := &model.Connection{
		Name:      "dirhost",
		Host:      "dir.example.com",
		User:      "root",
		Port:      22,
		Directory: "/data/music",
	}
	if err := Insert(p, conn); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(p)
	content := string(data)
	if !strings.Contains(content, "RemoteCommand cd '/data/music' && exec $SHELL -l") {
		t.Errorf("expected RemoteCommand:\n%s", content)
	}
	if !strings.Contains(content, "RequestTTY force") {
		t.Errorf("expected RequestTTY:\n%s", content)
	}
}

func TestInsertDuplicateNameErrors(t *testing.T) {
	p := writeTempConfig(t, "Host dup\n    HostName dup.com\n    User root\n")
	conn := &model.Connection{Name: "dup", Host: "other.com", User: "root", Port: 22}
	err := Insert(p, conn)
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestInsertRoundTrip(t *testing.T) {
	p := writeTempConfig(t, "")
	conn := &model.Connection{
		Name:         "rt",
		Host:         "rt.example.com",
		User:         "admin",
		Port:         3322,
		IdentityFile: "~/.ssh/mykey",
		Directory:    "/opt/app",
	}
	if err := Insert(p, conn); err != nil {
		t.Fatal(err)
	}

	// Re-parse
	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	c := conns[0]
	if c.Name != "rt" || c.Host != "rt.example.com" || c.User != "admin" || c.Port != 3322 {
		t.Errorf("round-trip mismatch: %+v", c)
	}
	if c.IdentityFile != "~/.ssh/mykey" {
		t.Errorf("identity file mismatch: %q", c.IdentityFile)
	}
	if c.Directory != "/opt/app" {
		t.Errorf("directory mismatch: %q", c.Directory)
	}
}

func TestUpdateBasic(t *testing.T) {
	initial := "Host target\n    HostName old.com\n    User old\n    Port 22\n\nHost other\n    HostName other.com\n    User root\n"
	p := writeTempConfig(t, initial)

	conn := &model.Connection{
		Name: "target",
		Host: "new.com",
		User: "newuser",
		Port: 3322,
	}
	if err := Update(p, "target", conn); err != nil {
		t.Fatal(err)
	}

	// Verify updated
	c, err := GetByName(p, "target")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "new.com" || c.User != "newuser" || c.Port != 3322 {
		t.Errorf("update mismatch: %+v", c)
	}

	// Verify other host preserved
	c, err = GetByName(p, "other")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "other.com" {
		t.Errorf("other host should be preserved: %+v", c)
	}
}

func TestUpdateAddDirectory(t *testing.T) {
	initial := "Host myhost\n    HostName myhost.com\n    User root\n"
	p := writeTempConfig(t, initial)

	conn := &model.Connection{
		Name:      "myhost",
		Host:      "myhost.com",
		User:      "root",
		Port:      22,
		Directory: "/var/www",
	}
	if err := Update(p, "myhost", conn); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(p)
	content := string(data)
	if !strings.Contains(content, "RemoteCommand") || !strings.Contains(content, "RequestTTY") {
		t.Errorf("expected RemoteCommand and RequestTTY after adding directory:\n%s", content)
	}
}

func TestUpdateRemoveDirectory(t *testing.T) {
	initial := "Host myhost\n    HostName myhost.com\n    User root\n    RemoteCommand cd '/data' && exec $SHELL -l\n    RequestTTY force\n"
	p := writeTempConfig(t, initial)

	conn := &model.Connection{
		Name: "myhost",
		Host: "myhost.com",
		User: "root",
		Port: 22,
	}
	if err := Update(p, "myhost", conn); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(p)
	content := string(data)
	if strings.Contains(content, "RemoteCommand") || strings.Contains(content, "RequestTTY") {
		t.Errorf("expected RemoteCommand/RequestTTY removed:\n%s", content)
	}
}

func TestUpdateRename(t *testing.T) {
	initial := "Host oldname\n    HostName host.com\n    User root\n"
	p := writeTempConfig(t, initial)

	conn := &model.Connection{
		Name: "newname",
		Host: "host.com",
		User: "root",
		Port: 22,
	}
	if err := Update(p, "oldname", conn); err != nil {
		t.Fatal(err)
	}

	exists, _ := Exists(p, "oldname")
	if exists {
		t.Error("old name should not exist")
	}
	exists, _ = Exists(p, "newname")
	if !exists {
		t.Error("new name should exist")
	}
}

func TestDeleteBasic(t *testing.T) {
	initial := "Host first\n    HostName first.com\n    User root\n\nHost second\n    HostName second.com\n    User root\n\nHost third\n    HostName third.com\n    User root\n"
	p := writeTempConfig(t, initial)

	if err := Delete(p, "second"); err != nil {
		t.Fatal(err)
	}

	conns, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections after delete, got %d", len(conns))
	}

	names := make([]string, len(conns))
	for i, c := range conns {
		names[i] = c.Name
	}
	if names[0] != "first" || names[1] != "third" {
		t.Errorf("unexpected remaining hosts: %v", names)
	}
}

func TestDeleteFirst(t *testing.T) {
	initial := "Host first\n    HostName first.com\n    User root\n\nHost second\n    HostName second.com\n    User root\n"
	p := writeTempConfig(t, initial)

	if err := Delete(p, "first"); err != nil {
		t.Fatal(err)
	}

	conns, _ := List(p)
	if len(conns) != 1 || conns[0].Name != "second" {
		t.Errorf("expected only second: %+v", conns)
	}
}

func TestDeleteLast(t *testing.T) {
	initial := "Host first\n    HostName first.com\n    User root\n\nHost second\n    HostName second.com\n    User root\n"
	p := writeTempConfig(t, initial)

	if err := Delete(p, "second"); err != nil {
		t.Fatal(err)
	}

	conns, _ := List(p)
	if len(conns) != 1 || conns[0].Name != "first" {
		t.Errorf("expected only first: %+v", conns)
	}
}

func TestDeleteNotFoundErrors(t *testing.T) {
	p := writeTempConfig(t, "Host only\n    HostName only.com\n    User root\n")
	err := Delete(p, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent host")
	}
}

func TestDeletePortNotWrittenForDefault(t *testing.T) {
	p := writeTempConfig(t, "")
	conn := &model.Connection{
		Name: "noport",
		Host: "noport.com",
		User: "root",
		Port: 22,
	}
	Insert(p, conn)

	data, _ := os.ReadFile(p)
	if strings.Contains(string(data), "Port") {
		t.Errorf("Port 22 should not be written explicitly:\n%s", string(data))
	}
}

func TestDeleteIdentityFileDefaultNotWritten(t *testing.T) {
	p := writeTempConfig(t, "")
	conn := &model.Connection{
		Name:         "defkey",
		Host:         "defkey.com",
		User:         "root",
		Port:         22,
		IdentityFile: "default",
	}
	Insert(p, conn)

	data, _ := os.ReadFile(p)
	if strings.Contains(string(data), "IdentityFile") {
		t.Errorf("IdentityFile 'default' should not be written:\n%s", string(data))
	}
}

func TestDirectiveInsertAndDetect(t *testing.T) {
	dir := t.TempDir()
	mainPath := dir + "/config"
	os.WriteFile(mainPath, []byte("Host existing\n    HostName test.com\n"), 0600)

	// Simulate prependIncludeDirective
	err := prependIncludeDirective(mainPath)
	if err != nil {
		t.Fatal(err)
	}

	has, err := hasIncludeDirective(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected Include directive to be detected")
	}

	// Verify existing content preserved
	data, _ := os.ReadFile(mainPath)
	if !strings.Contains(string(data), "Host existing") {
		t.Error("existing content should be preserved")
	}
}
