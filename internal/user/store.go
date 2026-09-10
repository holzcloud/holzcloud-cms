// Package user owns the users table.
//
// The SQL used to sit inline in the admin handlers, which made users the only
// entity without a store and meant a CLI could not create or recover an account
// without duplicating it. Both callers now share this one path.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// Roles recognised by the application.
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
)

// ErrNotFound is returned when no user matches.
var ErrNotFound = errors.New("user not found")

// ErrDuplicateEmail is returned when an email is already taken.
var ErrDuplicateEmail = errors.New("a user with that email already exists")

// ErrLastAdmin is returned when an operation would leave no admin behind.
var ErrLastAdmin = errors.New("this is the last admin account")

// User is a row of the users table. Password holds the PHC-format hash and is
// only populated by the lookups that need to verify it.
type User struct {
	ID        int64
	Name      string
	Email     string
	Role      string
	Password  string
	CreatedAt string
	// Locale is the language this person reads the administration in. Empty
	// means they have made no choice, and the browser decides — which is not
	// the same as having chosen German.
	Locale string
}

// Store handles SQL operations for users.
type Store struct {
	DB     *db.DB
	Params auth.Argon2Params

	// now is the clock the time-based code checks read.
	//
	// A field rather than a direct call to time.Now, because the replay window
	// is thirty seconds wide: without moving the clock there is no way to test
	// that a second sign-in a minute later works, and that is exactly the path
	// every real user takes.
	now func() time.Time
}

// NewStore creates a user store. Params are the Argon2id costs used when
// hashing a new password.
func NewStore(database *db.DB, params auth.Argon2Params) *Store {
	return &Store{DB: database, Params: params, now: time.Now}
}

// clock returns the store's time source, defaulting to the real one so a Store
// built as a bare struct literal still works.
func (s *Store) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// ValidRole reports whether role is one the application understands.
func ValidRole(role string) bool {
	return role == RoleAdmin || role == RoleEditor
}

// List returns every user, ordered by name.
func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT id, name, email, role, created_at FROM users ORDER BY name, email`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetByID returns a user including the password hash, or nil when absent.
func (s *Store) GetByID(ctx context.Context, id int64) (*User, error) {
	return s.scanOne(s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, email, role, password, locale FROM users WHERE id = $1`, id))
}

// GetByEmail returns a user including the password hash, or nil when absent.
func (s *Store) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanOne(s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, email, role, password, locale FROM users WHERE email = $1`, strings.TrimSpace(email)))
}

// ErrSSOUsernameTaken is returned when an identity is already linked to
// another account.
var ErrSSOUsernameTaken = errors.New("that identity is already linked to another account")

// GetBySSOUsername returns the account linked to an identity at the identity
// provider, or nil when no account is.
//
// This, and not GetByEmail, is how a forward-auth sign-in finds its account.
// An address is only something an identity claims; until migration 00053 the
// sign-in trusted the claim, and whoever could make the identity provider emit
// an administrator's address became that administrator.
//
// The comparison is exact. Folding case or Unicode here would be a second
// definition of "the same identity" beside the identity provider's own.
func (s *Store) GetBySSOUsername(ctx context.Context, username string) (*User, error) {
	if username == "" {
		return nil, nil
	}
	return s.scanOne(s.DB.Read.QueryRowContext(ctx,
		`SELECT id, name, email, role, password, locale FROM users WHERE sso_username = $1`, username))
}

// LinkSSO links an account to an identity at the identity provider. The empty
// username removes the link, and the account is then unreachable through
// single sign-on until it is linked again.
func (s *Store) LinkSSO(ctx context.Context, id int64, username string) error {
	var value any
	if username != "" {
		value = username
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET sso_username = $1 WHERE id = $2`, value, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrSSOUsernameTaken
		}
		return fmt.Errorf("link identity: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("link identity: no account %d", id)
	}
	return nil
}

func (s *Store) scanOne(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Password, &u.Locale)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// SetLocale stores the language a person reads the administration in.
//
// The empty string is a legitimate value and means "let the browser decide" —
// which is why this takes the caller's word for it rather than falling back to
// a default here.
func (s *Store) SetLocale(ctx context.Context, id int64, locale string) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET locale = $1 WHERE id = $2`, locale, id)
	if err != nil {
		return fmt.Errorf("set user language: %w", err)
	}
	return nil
}

// Locale is the stored language of one user, for the request middleware.
//
// A query of its own rather than GetByID: this runs on every admin request, and
// pulling the password hash through the connection for every page view to read
// one short string is the sort of cost that is invisible until it is not.
func (s *Store) Locale(ctx context.Context, id int64) string {
	var locale string
	if err := s.DB.Read.QueryRowContext(ctx,
		`SELECT locale FROM users WHERE id = $1`, id).Scan(&locale); err != nil {
		return ""
	}
	return locale
}

// Role returns the current role of a user, and whether the user still exists.
// It is the lookup the auth middleware uses to re-validate a session.
func (s *Store) Role(ctx context.Context, id int64) (string, bool, error) {
	var role string
	err := s.DB.Read.QueryRowContext(ctx, `SELECT role FROM users WHERE id = $1`, id).Scan(&role)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return role, true, nil
}

// Create hashes the password and inserts a user.
//
// # The empty address is refused here for a security reason, not a tidiness one
//
// `users.email` is declared `NOT NULL UNIQUE COLLATE NOCASE` (00001:5) and
// nothing in that declaration forbids the empty string. A row whose address is
// empty is legal, and exactly one of them can exist. Every lookup by address
// that does not refuse an empty needle therefore *matches that row*, whoever it
// belongs to.
//
// Phase 10's forward authentication is where that stopped being hypothetical:
// an identity arriving with no e-mail header would be looked up rather than
// refused, and would sign in as whatever that row is. Its middleware refuses
// the empty address, its provisioning refuses it again by a named sentinel, and
// this is the third layer — the one that stops the row from being created in
// the first place.
//
// Said out loud because the line below reads like a validation message and is
// not one. A later reader softening it into a form error, or moving it into the
// handlers "where the other validation lives", would remove a guard without
// noticing there was one. Measured 2026-09-08: with this check and
// provisioning's own both removed, an account for the empty address is created.
func (s *Store) Create(ctx context.Context, name, email, password, role string) (int64, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return 0, errors.New("email is required")
	}
	if !ValidRole(role) {
		return 0, fmt.Errorf("role must be %q or %q", RoleAdmin, RoleEditor)
	}
	if len(password) < auth.MinPasswordLength {
		return 0, fmt.Errorf("password must be at least %d characters", auth.MinPasswordLength)
	}

	hash, err := auth.HashPassword(password, s.Params)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}

	res, err := s.DB.Write.ExecContext(ctx,
		`INSERT INTO users (name, email, password, role) VALUES ($1, $2, $3, $4)`,
		strings.TrimSpace(name), email, hash, role)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicateEmail
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// Update changes name, email and role. It refuses to demote the last admin.
func (s *Store) Update(ctx context.Context, id int64, name, email, role string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email is required")
	}
	if !ValidRole(role) {
		return fmt.Errorf("role must be %q or %q", RoleAdmin, RoleEditor)
	}

	current, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrNotFound
	}
	if current.Role == RoleAdmin && role != RoleAdmin {
		last, err := s.isLastAdmin(ctx)
		if err != nil {
			return err
		}
		if last {
			return ErrLastAdmin
		}
	}

	_, err = s.DB.Write.ExecContext(ctx,
		`UPDATE users SET name = $1, email = $2, role = $3 WHERE id = $4`,
		strings.TrimSpace(name), email, role, id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// SetPassword hashes and stores a new password.
func (s *Store) SetPassword(ctx context.Context, id int64, password string) error {
	if len(password) < auth.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", auth.MinPasswordLength)
	}
	hash, err := auth.HashPassword(password, s.Params)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	res, err := s.DB.Write.ExecContext(ctx, `UPDATE users SET password = $1 WHERE id = $2`, hash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a user, refusing to remove the last admin.
func (s *Store) Delete(ctx context.Context, id int64) error {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if u == nil {
		return ErrNotFound
	}
	if u.Role == RoleAdmin {
		last, err := s.isLastAdmin(ctx)
		if err != nil {
			return err
		}
		if last {
			return ErrLastAdmin
		}
	}
	if _, err := s.DB.Write.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// CountAdmins returns how many admin accounts exist.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = $1`, RoleAdmin).Scan(&n)
	return n, err
}

// Count returns the total number of users, used by the setup guard.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.DB.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) isLastAdmin(ctx context.Context) (bool, error) {
	n, err := s.CountAdmins(ctx)
	if err != nil {
		return false, err
	}
	return n <= 1, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
