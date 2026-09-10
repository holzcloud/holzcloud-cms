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
// Which websites somebody may enter is said in two parts: whether they are
// limited at all, and to what. An account nobody ever limited may enter every
// website — not "none", otherwise migration 00033 would have locked everybody
// out, and an operator who never uses this never has to know it exists.
//
// Until migration 00052 the first part was not stored. It was read off the
// second, so "no rows" meant "every website", and user_websites loses rows
// without anybody touching the person: ON DELETE CASCADE when a website is
// deleted, a SetRights that failed between its DELETE and its INSERT, a
// sign-in that demoted an administrator, who has no rows by design. Each of
// those turned "only this one" into "all of them". Since 00052 a limited
// account stays limited when its last row goes and reaches nothing, which is
// the last thing the operator said about it.

// Rights are one person's limits.
type Rights struct {
	// MayPublish is false for somebody who writes and submits, and whose text
	// somebody else puts online.
	MayPublish bool
	// Websites are the sites this person may enter, when Limited reports true.
	Websites []int64
	// limited is true for an account restricted to Websites even when that list
	// is empty. It is unexported so that no caller can build "limited to
	// nothing" by forgetting a field: a Rights literal with websites in it is
	// limited by those websites, and the one way to say "none" is Nothing().
	limited bool
}

// Everything is the rights of somebody with no limits at all — what an
// administrator has and what a person nobody ever limited has.
func Everything() Rights { return Rights{MayPublish: true} }

// Nothing is the rights of an editor who may enter no website at all. It
// carries no publishing right either; a caller that means to keep one sets
// MayPublish on the result.
func Nothing() Rights { return Rights{limited: true} }

// MayUse reports whether a website is one this person may enter.
func (r Rights) MayUse(websiteID int64) bool {
	if !r.Limited() {
		return true
	}
	for _, id := range r.Websites {
		if id == websiteID {
			return true
		}
	}
	return false
}

// Limited reports whether this person is restricted to Websites — including
// restricted to none of them.
func (r Rights) Limited() bool { return r.limited || len(r.Websites) > 0 }

// Rights loads one person's limits.
//
// An administrator has none: the role is the right to run the installation, and
// a site an administrator may not enter would be a site nobody could repair.
func (s *Store) Rights(ctx context.Context, id int64) (Rights, error) {
	var role string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT role FROM users WHERE id = $1`, id).Scan(&role)
	if err != nil {
		return Rights{}, fmt.Errorf("read rights: %w", err)
	}
	if role == RoleAdmin {
		return Everything(), nil
	}
	return s.Assignment(ctx, id)
}

// Assignment loads what is stored about a person's limits, whatever their role.
//
// Rights answers the question the screens and the middleware ask, and for an
// administrator that answer is Everything() without reading a row. The
// forward-auth sync asks the other question — what is written there — because
// it changes the websites before it changes the role, and comparing against
// Everything() would read an administrator's stored assignment as no rows.
func (s *Store) Assignment(ctx context.Context, id int64) (Rights, error) {
	var mayPublish, limited int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT may_publish, websites_limited FROM users WHERE id = $1`, id).Scan(&mayPublish, &limited)
	if err != nil {
		return Rights{}, fmt.Errorf("read rights: %w", err)
	}

	out := Rights{MayPublish: mayPublish != 0, limited: limited != 0}
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
	limited := 0
	if rights.Limited() {
		limited = 1
	}

	// One transaction. These statements used to run without one, and a failure
	// between the DELETE and the INSERTs left the person with no rows and a
	// publishing right already changed — half of a change nobody asked for,
	// and before migration 00052 the half that meant every website. Since
	// Phase 10 the forward-auth sync calls this unattended at every sign-in,
	// and a website group pointing at a deleted website fails exactly that
	// INSERT, on the foreign key.
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rights: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET may_publish = $1, websites_limited = $2 WHERE id = $3`,
		publish, limited, id); err != nil {
		return fmt.Errorf("set publishing right and limit: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
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
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_websites (user_id, website_id) VALUES ($1, $2)`, id, websiteID); err != nil {
			return fmt.Errorf("assign website: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rights: %w", err)
	}
	return nil
}
