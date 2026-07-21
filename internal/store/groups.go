package store

import (
	"database/sql"
	"fmt"
)

// ListGroups returns all groups belonging to a user, ordered by sort_order.
func ListGroups(username string) ([]ConnectionGroup, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, user_id, name, parent_id, sort_order
		 FROM connection_groups WHERE user_id = ? ORDER BY sort_order, id`,
		uid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConnectionGroup{}
	for rows.Next() {
		var g ConnectionGroup
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.ParentID, &g.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CreateGroup creates a new group for a user.
func CreateGroup(username, name string, parentID *int, sortOrder int) (*ConnectionGroup, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	res, err := db.Exec(
		`INSERT INTO connection_groups (user_id, name, parent_id, sort_order)
		 VALUES (?, ?, ?, ?)`,
		uid, name, parentID, sortOrder,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ConnectionGroup{
		ID: int(id), UserID: uid, Name: name, ParentID: parentID, SortOrder: sortOrder,
	}, nil
}

// GetGroup fetches a single group by id, enforcing ownership.
func GetGroup(username string, id int) (*ConnectionGroup, error) {
	uid, err := GetUserID(username)
	if err != nil {
		return nil, err
	}
	var g ConnectionGroup
	if err := db.QueryRow(
		`SELECT id, user_id, name, parent_id, sort_order
		 FROM connection_groups WHERE id = ? AND user_id = ?`,
		id, uid,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.ParentID, &g.SortOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, err
	}
	return &g, nil
}

// UpdateGroup applies a partial update to a group using a read-modify-write
// strategy: the stored group is loaded first and only the fields present in
// `fields` overwrite the existing values before writing back.
//
// This is the fix for bug B3 (rename group). Previously GroupsUpdate bound
// SortOrder int from the request; a rename payload {"name":"x"} left sort_order
// at its zero value, so UPDATE SET sort_order = 0 reset the group's order to the
// top. Now sort_order (and parent_id) are preserved unless explicitly sent.
func UpdateGroup(username string, id int, fields map[string]interface{}) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	existing, err := GetGroup(username, id)
	if err != nil {
		return err
	}
	// Defense in depth: ownership is already enforced by GetGroup.
	if existing.UserID != uid {
		return fmt.Errorf("group not found")
	}

	merged := *existing
	if v, ok := fields["name"]; ok {
		merged.Name = toString(v)
	}
	if v, ok := fields["sort_order"]; ok {
		merged.SortOrder = toInt(v)
	}
	if v, ok := fields["parent_id"]; ok {
		merged.ParentID = toIntPtr(v)
	}

	res, err := db.Exec(
		`UPDATE connection_groups SET name = ?, sort_order = ?, parent_id = ?
		 WHERE id = ? AND user_id = ?`,
		merged.Name, merged.SortOrder, merged.ParentID, id, uid,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("group not found")
	}
	return nil
}

// DeleteGroup removes a group; its connections are detached (group_id -> NULL).
func DeleteGroup(username string, id int) error {
	uid, err := GetUserID(username)
	if err != nil {
		return err
	}
	res, err := db.Exec(`DELETE FROM connection_groups WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("group not found")
	}
	return nil
}
