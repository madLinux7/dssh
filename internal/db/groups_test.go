package db

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/madLinux7/dssh/internal/model"
)

func newGroupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if _, err := d.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := d.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return d
}

func TestMembershipsAreScopedAndCountsFollowTheActiveBackend(t *testing.T) {
	d := newGroupTestDB(t)
	production, err := CreateGroup(d, "Production")
	if err != nil {
		t.Fatal(err)
	}
	shared, err := CreateGroup(d, "Shared")
	if err != nil {
		t.Fatal(err)
	}

	sqliteRef := model.ConnectionRef{Source: model.SourceSQLite, Name: "api"}
	sshARef := model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: "/configs/a", Name: "api"}
	sshBRef := model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: "/configs/b", Name: "api"}
	if err := SetConnectionGroups(d, sqliteRef, []int64{production.ID, shared.ID}); err != nil {
		t.Fatal(err)
	}
	if err := SetConnectionGroups(d, sshARef, []int64{shared.ID}); err != nil {
		t.Fatal(err)
	}
	if err := SetConnectionGroups(d, sshBRef, []int64{production.ID}); err != nil {
		t.Fatal(err)
	}

	sqliteGroups, err := ListGroupsWithCounts(d, model.SourceSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := countsByName(sqliteGroups); got["Production"] != 1 || got["Shared"] != 1 {
		t.Fatalf("SQLite counts = %#v, want Production=1 Shared=1", got)
	}

	sshAGroups, err := ListGroupsWithCounts(d, model.SourceSSHConfig, "/configs/a")
	if err != nil {
		t.Fatal(err)
	}
	if got := countsByName(sshAGroups); got["Production"] != 0 || got["Shared"] != 1 {
		t.Fatalf("ssh_config A counts = %#v, want Production=0 Shared=1", got)
	}

	ids, err := GroupIDsForConnection(d, sshBRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != production.ID {
		t.Fatalf("ssh_config B group IDs = %v, want [%d]", ids, production.ID)
	}

	names, err := ConnectionNamesForGroup(d, shared.ID, model.SourceSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "api" {
		t.Fatalf("SQLite names in Shared = %v, want [api]", names)
	}
}

func TestGroupAndConnectionLifecycleKeepsMembershipsConsistent(t *testing.T) {
	d := newGroupTestDB(t)
	group, err := CreateGroup(d, "Old Name")
	if err != nil {
		t.Fatal(err)
	}
	ref := model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: "/configs/a", Name: "old-host"}
	if err := SetConnectionGroups(d, ref, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}

	if err := RenameGroup(d, group.ID, "New Name"); err != nil {
		t.Fatalf("rename group: %v", err)
	}
	groups, err := ListGroups(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "New Name" {
		t.Fatalf("groups after rename = %#v", groups)
	}

	renamedRef := model.ConnectionRef{Source: ref.Source, SourcePath: ref.SourcePath, Name: "new-host"}
	if err := ReplaceConnectionGroups(d, ref, renamedRef, []int64{group.ID}); err != nil {
		t.Fatalf("replace memberships: %v", err)
	}
	ids, err := GroupIDsForConnection(d, renamedRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("renamed connection group IDs = %v, want [%d]", ids, group.ID)
	}

	if err := ReconcileConnectionMemberships(d, model.SourceSSHConfig, "/configs/a", []string{"another-host"}); err != nil {
		t.Fatalf("reconcile memberships: %v", err)
	}
	ids, err = GroupIDsForConnection(d, renamedRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("stale memberships survived reconciliation: %v", ids)
	}

	if err := DeleteGroup(d, group.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	groups, err = ListGroups(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("groups after delete = %#v, want none", groups)
	}
}

func TestSQLiteConnectionAndAssignmentsSaveAtomically(t *testing.T) {
	d := newGroupTestDB(t)
	group, err := CreateGroup(d, "Production")
	if err != nil {
		t.Fatal(err)
	}
	conn := &model.Connection{Name: "api", User: "root", Host: "api.example", Port: 22, AuthType: model.AuthKey}
	ref := model.ConnectionRef{Source: model.SourceSQLite, Name: conn.Name}
	if err := InsertWithGroups(d, conn, ref, []int64{group.ID}); err != nil {
		t.Fatalf("insert with groups: %v", err)
	}
	if _, err := GetByName(d, conn.Name); err != nil {
		t.Fatalf("saved connection missing: %v", err)
	}
	ids, err := GroupIDsForConnection(d, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("saved group IDs = %v, want [%d]", ids, group.ID)
	}

	bad := &model.Connection{Name: "broken", User: "root", Host: "broken.example", Port: 22, AuthType: model.AuthKey}
	badRef := model.ConnectionRef{Source: model.SourceSQLite, Name: bad.Name}
	if err := InsertWithGroups(d, bad, badRef, []int64{99999}); err == nil {
		t.Fatal("insert with unknown group succeeded")
	}
	if _, err := GetByName(d, bad.Name); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partially saved connection error = %v, want ErrNotFound", err)
	}
}

func TestSQLiteEditAndAssignmentChangesSaveAtomically(t *testing.T) {
	d := newGroupTestDB(t)
	oldGroup, _ := CreateGroup(d, "Old")
	newGroup, _ := CreateGroup(d, "New")
	conn := &model.Connection{Name: "api", User: "root", Host: "old.example", Port: 22, AuthType: model.AuthKey}
	oldRef := model.ConnectionRef{Source: model.SourceSQLite, Name: conn.Name}
	if err := InsertWithGroups(d, conn, oldRef, []int64{oldGroup.ID}); err != nil {
		t.Fatal(err)
	}

	conn.Name = "renamed-api"
	conn.Host = "new.example"
	newRef := model.ConnectionRef{Source: model.SourceSQLite, Name: conn.Name}
	if err := UpdateWithGroups(d, conn, oldRef, newRef, []int64{newGroup.ID}); err != nil {
		t.Fatalf("update with groups: %v", err)
	}
	if _, err := GetByName(d, "api"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old name lookup error = %v, want ErrNotFound", err)
	}
	updated, err := GetByName(d, "renamed-api")
	if err != nil || updated.Host != "new.example" {
		t.Fatalf("updated connection = %#v, err %v", updated, err)
	}
	ids, _ := GroupIDsForConnection(d, newRef)
	if len(ids) != 1 || ids[0] != newGroup.ID {
		t.Fatalf("updated group IDs = %v, want [%d]", ids, newGroup.ID)
	}

	conn.Host = "must-not-stick.example"
	if err := UpdateWithGroups(d, conn, newRef, newRef, []int64{99999}); err == nil {
		t.Fatal("update with unknown group succeeded")
	}
	updated, err = GetByName(d, "renamed-api")
	if err != nil || updated.Host != "new.example" {
		t.Fatalf("failed edit was partially saved: %#v, err %v", updated, err)
	}
}

func TestDeletingSQLiteConnectionAlsoDeletesMemberships(t *testing.T) {
	d := newGroupTestDB(t)
	group, _ := CreateGroup(d, "Production")
	connection := &model.Connection{Name: "api", User: "root", Host: "api.example", Port: 22, AuthType: model.AuthKey}
	ref := model.ConnectionRef{Source: model.SourceSQLite, Name: connection.Name}
	if err := InsertWithGroups(d, connection, ref, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}
	if err := Delete(d, connection.Name); err != nil {
		t.Fatal(err)
	}
	ids, err := GroupIDsForConnection(d, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("memberships after connection delete = %v, want none", ids)
	}
}

func countsByName(groups []model.GroupWithCount) map[string]int {
	counts := make(map[string]int, len(groups))
	for _, group := range groups {
		counts[group.Name] = group.ConnectionCount
	}
	return counts
}

func TestGroupsAreValidatedUniqueAndListedDescending(t *testing.T) {
	d := newGroupTestDB(t)

	for _, name := range []string{"staging", "Production", "alpha"} {
		if _, err := CreateGroup(d, name); err != nil {
			t.Fatalf("create group %q: %v", name, err)
		}
	}

	groups, err := ListGroups(d)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []string{"staging", "Production", "alpha"}
	if len(groups) != len(want) {
		t.Fatalf("group count = %d, want %d", len(groups), len(want))
	}
	for i, name := range want {
		if groups[i].Name != name {
			t.Errorf("group %d = %q, want %q", i, groups[i].Name, name)
		}
	}

	if _, err := CreateGroup(d, " production "); !errors.Is(err, ErrDuplicateGroupName) {
		t.Fatalf("case-insensitive duplicate error = %v, want ErrDuplicateGroupName", err)
	}
	if _, err := CreateGroup(d, "   "); !errors.Is(err, ErrInvalidGroupName) {
		t.Fatalf("blank name error = %v, want ErrInvalidGroupName", err)
	}
}
