package cli

import (
	"database/sql"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

func TestFilterConnectionsByGroupsUsesAnyGroupAndScopedMemberships(t *testing.T) {
	conns := []model.Connection{{Name: "api", Source: model.SourceSQLite}, {Name: "api", Source: model.SourceSSHConfig}, {Name: "worker", Source: model.SourceSQLite}}
	memberships := map[string][]string{connectionScopeKey(conns[0]): {"Production"}, connectionScopeKey(conns[1]): {"Staging"}}
	filtered := filterConnectionsByGroups(conns, memberships, []string{"production", "staging"}, false)
	if !reflect.DeepEqual(filtered, conns[:2]) {
		t.Fatalf("any-group filter = %#v, want %#v", filtered, conns[:2])
	}
	ungrouped := filterConnectionsByGroups(conns, memberships, nil, true)
	if !reflect.DeepEqual(ungrouped, conns[2:]) {
		t.Fatalf("ungrouped filter = %#v, want %#v", ungrouped, conns[2:])
	}
}

func TestListWithUnknownGroupPrintsNothing(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Initialize(database); err != nil {
		t.Fatal(err)
	}

	previousDB, previousCfg := sharedDB, runtimeCfg
	sharedDB = database
	runtimeCfg = &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly}
	t.Cleanup(func() { sharedDB, runtimeCfg = previousDB, previousCfg })

	output := captureStdout(t, func() {
		cmd := newListCmd()
		cmd.SetArgs([]string{"--group", "NonexistantGroup"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if output != "" {
		t.Fatalf("unknown group output = %q, want empty", output)
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = previous
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
