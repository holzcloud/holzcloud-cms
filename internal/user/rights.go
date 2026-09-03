package user

import (
	"context"
	"fmt"
	"sort"
)

// What a person may do, beyond their role.
//
// Two roles are right for an installation of this size: administrator for the
// installation itself, editor for the content. What is missing in practice is
// not more roles but two limits inside the one — which websites somebody may
// enter, and whether they may publish or only submit.
//
// Both are properties of a person, not of a role. A club has somebody who looks
// after the committee site and somebody who writes and whose text is read
// before it goes out; with a list of roles that could only be expressed by
// inventing a role per combination.
//
// One rule runs through everything here: **no assignment means every website.**
// Not "none" — otherwise the migration itself would lock everybody out, and an
// operator who never uses this never has to know it exists.

// Rights are one person's limits.
type Rights struct {
	// MayPublish is false for somebody who writes and submits, and whose text
	// somebody else puts online.
	MayPublish bool
	// Websites are the sites this person may enter. Empty means all of them.
	Websites []int64
}

// Everything is the rights of somebody with no limits at all — what an
// administrator has and what a person without an assignment has.
func Everything() Rights { return Rights{MayPublish: true} }

// MayUse reports whether a website is one this person may enter.
func (r Rights) MayUse(websiteID int64) bool {
	if len(r.Websites) == 0 {
		return true
	}
	for _, id := range r.Websites {
		if id == websiteID {
			return true
		}
	}
	return false
}

// Limited reports whether this person is restricted to certain websites, for a
// screen that wants to say so.
func (r Rights) Limited() bool { return len(r.Websites) > 0 }

// Rights loads one person's limits.
//
// An administrator has none: the role is the right to run the installation, and
// a site an administrator may not enter would be a site nobody could repair.
func (s *Store) Rights(ctx context.Context, id int64) (Rights, error) {
	var role string
	var mayPublish int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT role, may_publish FROM users WHERE id = $1`, id).Scan(&role, &mayPublish)
	if err != nil {
		return Rights{}, fmt.Errorf("read rights: %w", err)
	}
	if role == RoleAdmin {
		return Everything(), nil
	}

	out := Rights{MayPublish: mayPublish != 0}
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT website_id FROM user_websites WHERE user_id = $1 ORDER BY website_id`, id)
	if err != nil {
		return Rights{}, fmt.Errorf("read website assignment: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var websiteID int64
		if err := rows.Scan(&websiteID); err != nil {
			return Rights{}, fmt.Errorf("scan website assignment: %w", err)
		}
		out.Websites = append(out.Websites, websiteID)
	}
	return out, rows.Err()
}

// SetRights stores what a person may do.
//
// The assignment is replaced wholesale rather than merged: the form shows every
// website with a tick, so what comes back is the complete answer, and a
// difference calculation could only get it wrong.
func (s *Store) SetRights(ctx context.Context, id int64, rights Rights) error {
	publish := 0
	if rights.MayPublish {
		publish = 1
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET may_publish = $1 WHERE id = $2`, publish, id); err != nil {
		return fmt.Errorf("set publishing right: %w", err)
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM user_websites WHERE user_id = $1`, id); err != nil {
		return fmt.Errorf("clear website assignment: %w", err)
	}

	seen := map[int64]bool{}
	ids := append([]int64(nil), rights.Websites...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, websiteID := range ids {
		if websiteID <= 0 || seen[websiteID] {
			continue
		}
		seen[websiteID] = true
		if _, err := s.DB.Write.ExecContext(ctx,
			`INSERT INTO user_websites (user_id, website_id) VALUES ($1, $2)`, id, websiteID); err != nil {
			return fmt.Errorf("assign website: %w", err)
		}
	}
	return nil
}
