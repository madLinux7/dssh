package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/madLinux7/dssh/internal/model"
)

var (
	// ErrInvalidGroupName is returned when a group name is blank or too long.
	ErrInvalidGroupName = errors.New("group name must contain 1 to 64 characters")
	// ErrDuplicateGroupName is returned when a case-insensitive name already exists.
	ErrDuplicateGroupName = errors.New("a group with that name already exists")
)

// NormalizeGroupName trims a group name and returns its comparison key.
func NormalizeGroupName(name string) (displayName, normalizedName string, err error) {
	displayName = strings.TrimSpace(name)
	if displayName == "" || utf8.RuneCountInString(displayName) > 64 {
		return "", "", ErrInvalidGroupName
	}
	return displayName, strings.ToLower(displayName), nil
}

// CreateGroup creates a uniquely named connection group.
func CreateGroup(d *sql.DB, name string) (*model.Group, error) {
	displayName, normalizedName, err := NormalizeGroupName(name)
	if err != nil {
		return nil, err
	}

	result, err := d.Exec(`
		INSERT INTO connection_groups (name, normalized_name)
		VALUES (?, ?)`, displayName, normalizedName)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateGroupName, displayName)
		}
		return nil, fmt.Errorf("create group: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read new group id: %w", err)
	}
	return &model.Group{ID: id, Name: displayName}, nil
}

// ListGroups returns all groups ordered case-insensitively from Z to A.
func ListGroups(d *sql.DB) ([]model.Group, error) {
	rows, err := d.Query(`
		SELECT id, name
		FROM connection_groups
		ORDER BY normalized_name DESC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []model.Group
	for rows.Next() {
		var group model.Group
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

// GetGroupByName returns a group by its case-insensitive name.
func GetGroupByName(d *sql.DB, name string) (*model.Group, error) {
	_, normalizedName, err := NormalizeGroupName(name)
	if err != nil {
		return nil, err
	}
	group := &model.Group{}
	err = d.QueryRow(`SELECT id, name FROM connection_groups WHERE normalized_name = ?`, normalizedName).
		Scan(&group.ID, &group.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: group %q", ErrNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("get group %q: %w", name, err)
	}
	return group, nil
}

// GroupIDsByNames resolves every supplied group name before a caller mutates
// anything. Repeated names resolve to one ID.
func GroupIDsByNames(d *sql.DB, names []string) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	seen := make(map[int64]struct{}, len(names))
	for _, name := range names {
		group, err := GetGroupByName(d, name)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[group.ID]; ok {
			continue
		}
		seen[group.ID] = struct{}{}
		ids = append(ids, group.ID)
	}
	return ids, nil
}

// RenameGroup changes a group name while preserving its identity and memberships.
func RenameGroup(d *sql.DB, id int64, name string) error {
	displayName, normalizedName, err := NormalizeGroupName(name)
	if err != nil {
		return err
	}
	result, err := d.Exec(`
		UPDATE connection_groups
		SET name = ?, normalized_name = ?, updated_at = datetime('now')
		WHERE id = ?`, displayName, normalizedName, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: %q", ErrDuplicateGroupName, displayName)
		}
		return fmt.Errorf("rename group: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%w: group %d", ErrNotFound, id)
	}
	return nil
}

// DeleteGroup removes a group and cascades its assignments.
func DeleteGroup(d *sql.DB, id int64) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin delete group: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM connection_group_memberships WHERE group_id = ?", id); err != nil {
		return fmt.Errorf("delete group assignments: %w", err)
	}
	result, err := tx.Exec("DELETE FROM connection_groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%w: group %d", ErrNotFound, id)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete group: %w", err)
	}
	return nil
}

// SetConnectionGroups replaces all group assignments for a connection.
func SetConnectionGroups(d *sql.DB, ref model.ConnectionRef, groupIDs []int64) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin group assignment: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := setConnectionGroupsTx(tx, ref, groupIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit group assignments: %w", err)
	}
	return nil
}

// AssignConnections adds a group membership for each reference atomically.
// It is deliberately idempotent, and validates the group before starting the
// write transaction.
func AssignConnections(d *sql.DB, groupID int64, refs []model.ConnectionRef) error {
	return changeConnectionMemberships(d, groupID, refs, true)
}

// UnassignConnections removes a group membership for each reference atomically.
// Removing an absent membership is a successful no-op.
func UnassignConnections(d *sql.DB, groupID int64, refs []model.ConnectionRef) error {
	return changeConnectionMemberships(d, groupID, refs, false)
}

func changeConnectionMemberships(d *sql.DB, groupID int64, refs []model.ConnectionRef, assign bool) error {
	var exists int
	if err := d.QueryRow("SELECT 1 FROM connection_groups WHERE id = ?", groupID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: group %d", ErrNotFound, groupID)
		}
		return fmt.Errorf("validate group: %w", err)
	}
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin membership change: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, ref := range uniqueConnectionRefs(refs) {
		if assign {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO connection_group_memberships
				(group_id, source, source_path, connection_name) VALUES (?, ?, ?, ?)`, groupID, ref.Source, ref.SourcePath, ref.Name); err != nil {
				return fmt.Errorf("assign connection to group: %w", err)
			}
		} else if _, err := tx.Exec(`DELETE FROM connection_group_memberships
			WHERE group_id = ? AND source = ? AND source_path = ? AND connection_name = ?`, groupID, ref.Source, ref.SourcePath, ref.Name); err != nil {
			return fmt.Errorf("unassign connection from group: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit membership change: %w", err)
	}
	return nil
}

// InsertWithGroups stores a SQLite connection and its assignments atomically.
func InsertWithGroups(d *sql.DB, connection *model.Connection, ref model.ConnectionRef, groupIDs []int64) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin connection save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertConnection(tx, connection); err != nil {
		return err
	}
	if err := setConnectionGroupsTx(tx, ref, groupIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit connection save: %w", err)
	}
	return nil
}

// UpdateWithGroups stores a SQLite edit and its assignment changes atomically.
func UpdateWithGroups(d *sql.DB, connection *model.Connection, oldRef, newRef model.ConnectionRef, groupIDs []int64) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin connection update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := updateConnection(tx, connection); err != nil {
		return err
	}
	if oldRef != newRef {
		if err := deleteConnectionMembershipsWith(tx, oldRef); err != nil {
			return fmt.Errorf("remove old connection assignments: %w", err)
		}
	}
	if err := setConnectionGroupsTx(tx, newRef, groupIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit connection update: %w", err)
	}
	return nil
}

func setConnectionGroupsTx(tx *sql.Tx, ref model.ConnectionRef, groupIDs []int64) error {
	if err := deleteConnectionMembershipsWith(tx, ref); err != nil {
		return fmt.Errorf("clear group assignments: %w", err)
	}
	for _, groupID := range uniqueGroupIDs(groupIDs) {
		var exists int
		if err := tx.QueryRow("SELECT 1 FROM connection_groups WHERE id = ?", groupID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: group %d", ErrNotFound, groupID)
			}
			return fmt.Errorf("validate group assignment: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO connection_group_memberships
				(group_id, source, source_path, connection_name)
			VALUES (?, ?, ?, ?)`, groupID, ref.Source, ref.SourcePath, ref.Name); err != nil {
			return fmt.Errorf("assign connection to group: %w", err)
		}
	}
	return nil
}

// ReplaceConnectionGroups applies a connection rename and assignment draft atomically.
func ReplaceConnectionGroups(d *sql.DB, oldRef, newRef model.ConnectionRef, groupIDs []int64) error {
	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin assignment update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if oldRef != newRef {
		if err := deleteConnectionMembershipsWith(tx, oldRef); err != nil {
			return fmt.Errorf("remove old connection assignments: %w", err)
		}
	}
	if err := setConnectionGroupsTx(tx, newRef, groupIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit assignment update: %w", err)
	}
	return nil
}

func deleteConnectionMembershipsWith(execer connectionExecer, ref model.ConnectionRef) error {
	_, err := execer.Exec(`
		DELETE FROM connection_group_memberships
		WHERE source = ? AND source_path = ? AND connection_name = ?`,
		ref.Source, ref.SourcePath, ref.Name)
	return err
}

// DeleteConnectionMemberships removes every assignment for a connection.
func DeleteConnectionMemberships(d *sql.DB, ref model.ConnectionRef) error {
	if err := deleteConnectionMembershipsWith(d, ref); err != nil {
		return fmt.Errorf("delete connection memberships: %w", err)
	}
	return nil
}

// ReconcileConnectionMemberships removes assignments for connections that no
// longer exist in the currently inspected backend scope.
func ReconcileConnectionMemberships(d *sql.DB, source model.Source, sourcePath string, existingNames []string) error {
	existing := make(map[string]struct{}, len(existingNames))
	for _, name := range existingNames {
		existing[name] = struct{}{}
	}

	rows, err := d.Query(`
		SELECT DISTINCT connection_name
		FROM connection_group_memberships
		WHERE source = ? AND source_path = ?`, source, sourcePath)
	if err != nil {
		return fmt.Errorf("inspect connection memberships: %w", err)
	}
	var stale []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan connection membership: %w", err)
		}
		if _, ok := existing[name]; !ok {
			stale = append(stale, name)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close membership scan: %w", err)
	}

	for _, name := range stale {
		if _, err := d.Exec(`
			DELETE FROM connection_group_memberships
			WHERE source = ? AND source_path = ? AND connection_name = ?`,
			source, sourcePath, name); err != nil {
			return fmt.Errorf("remove stale connection membership: %w", err)
		}
	}
	return nil
}

// GroupIDsForConnection returns a connection's assigned group IDs.
func GroupIDsForConnection(d *sql.DB, ref model.ConnectionRef) ([]int64, error) {
	rows, err := d.Query(`
		SELECT m.group_id
		FROM connection_group_memberships m
		JOIN connection_groups g ON g.id = m.group_id
		WHERE m.source = ? AND m.source_path = ? AND m.connection_name = ?
		ORDER BY g.normalized_name DESC, g.id ASC`, ref.Source, ref.SourcePath, ref.Name)
	if err != nil {
		return nil, fmt.Errorf("list connection groups: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan connection group: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListGroupsWithCounts returns every group with counts scoped to one backend.
func ListGroupsWithCounts(d *sql.DB, source model.Source, sourcePath string) ([]model.GroupWithCount, error) {
	rows, err := d.Query(`
		SELECT g.id, g.name, COUNT(m.connection_name)
		FROM connection_groups g
		LEFT JOIN connection_group_memberships m
			ON m.group_id = g.id AND m.source = ? AND m.source_path = ?
		GROUP BY g.id, g.name, g.normalized_name
		ORDER BY g.normalized_name DESC, g.id ASC`, source, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("list groups with counts: %w", err)
	}
	defer rows.Close()

	var groups []model.GroupWithCount
	for rows.Next() {
		var group model.GroupWithCount
		if err := rows.Scan(&group.ID, &group.Name, &group.ConnectionCount); err != nil {
			return nil, fmt.Errorf("scan group count: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// ConnectionNamesForGroup returns the connection names assigned to a group in a scope.
func ConnectionNamesForGroup(d *sql.DB, groupID int64, source model.Source, sourcePath string) ([]string, error) {
	rows, err := d.Query(`
		SELECT connection_name
		FROM connection_group_memberships
		WHERE group_id = ? AND source = ? AND source_path = ?
		ORDER BY lower(connection_name), connection_name`, groupID, source, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("list grouped connections: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan grouped connection: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// GroupNamesForConnections returns source-scoped group names by connection.
// Connections with no memberships are absent from the returned map.
func GroupNamesForConnections(d *sql.DB, source model.Source, sourcePath string) (map[string][]string, error) {
	rows, err := d.Query(`SELECT m.connection_name, g.name
		FROM connection_group_memberships m
		JOIN connection_groups g ON g.id = m.group_id
		WHERE m.source = ? AND m.source_path = ?
		ORDER BY g.normalized_name DESC, g.id ASC`, source, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("list scoped connection groups: %w", err)
	}
	defer rows.Close()
	groups := make(map[string][]string)
	for rows.Next() {
		var connectionName, groupName string
		if err := rows.Scan(&connectionName, &groupName); err != nil {
			return nil, fmt.Errorf("scan scoped connection group: %w", err)
		}
		groups[connectionName] = append(groups[connectionName], groupName)
	}
	return groups, rows.Err()
}

func uniqueGroupIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func uniqueConnectionRefs(refs []model.ConnectionRef) []model.ConnectionRef {
	seen := make(map[model.ConnectionRef]struct{}, len(refs))
	unique := make([]model.ConnectionRef, 0, len(refs))
	for _, ref := range refs {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		unique = append(unique, ref)
	}
	return unique
}
