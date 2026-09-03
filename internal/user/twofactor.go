package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/totp"
)

// RecoveryCodeCount is how many single-use codes are issued at setup.
//
// Ten is enough to survive several lost phones and few enough that a person
// will actually print the list rather than promise themselves they will.
const RecoveryCodeCount = 10

var (
	// ErrNoSecondFactor means the account has no authenticator set up.
	ErrNoSecondFactor = errors.New("this account has no second factor")
	// ErrBadCode covers a wrong code, an expired one and a replayed one. They
	// are not distinguished: "that code was already used" tells an attacker
	// their guess was otherwise right.
	ErrBadCode = errors.New("the code is not correct")
)

// TwoFactor is the state of one account's second factor.
type TwoFactor struct {
	Secret string
	// PendingSecret is an enrolment waiting for its first code. It exists
	// alongside an active Secret while someone moves to a new phone.
	PendingSecret string
	// Confirmed is false while the secret exists but no code has been entered
	// yet, which is the state a half-finished setup leaves behind.
	Confirmed     bool
	ConfirmedAt   *time.Time
	RecoveryLeft  int
	RecoveryTotal int
}

// Enabled reports whether the account must present a code to sign in.
func (t TwoFactor) Enabled() bool { return t.Secret != "" && t.Confirmed }

// GetTwoFactor loads an account's second-factor state.
func (s *Store) GetTwoFactor(ctx context.Context, userID int64) (*TwoFactor, error) {
	var tf TwoFactor
	var confirmedAt sql.NullString
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT totp_secret, totp_pending_secret, totp_confirmed_at FROM users WHERE id = $1`, userID).
		Scan(&tf.Secret, &tf.PendingSecret, &confirmedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get two factor: %w", err)
	}
	if confirmedAt.Valid {
		t, _ := time.Parse(tokenTimeLayout, confirmedAt.String)
		tf.ConfirmedAt, tf.Confirmed = &t, true
	}

	err = s.DB.Read.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE used_at IS NULL)
		 FROM user_recovery_codes WHERE user_id = $1`, userID).
		Scan(&tf.RecoveryTotal, &tf.RecoveryLeft)
	if err != nil {
		return nil, fmt.Errorf("count recovery codes: %w", err)
	}
	return &tf, nil
}

// EnsurePendingSecret returns the enrolment in progress, starting one if there
// is none.
//
// Reusing an existing pending secret is the whole point: the setup page is
// rendered again on every reload and after every mistyped code, and generating
// a new secret each time would invalidate the QR code the user has already
// scanned. They would then be told their correct code is wrong, with nothing on
// screen to explain it.
func (s *Store) EnsurePendingSecret(ctx context.Context, userID int64) (string, error) {
	var pending string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT totp_pending_secret FROM users WHERE id = $1`, userID).Scan(&pending)
	if err != nil {
		return "", fmt.Errorf("load pending secret: %w", err)
	}
	if pending != "" {
		return pending, nil
	}
	return s.BeginTwoFactor(ctx, userID)
}

// BeginTwoFactor generates a fresh secret and parks it as pending.
//
// It never touches the active factor. Opening the setup page with two-factor
// already on — to move to a new phone, or by misclicking — must not be able to
// lock the account out of its own installation; the old app keeps working until
// a code from the new one arrives.
func (s *Store) BeginTwoFactor(ctx context.Context, userID int64) (string, error) {
	secret, err := totp.GenerateSecret()
	if err != nil {
		return "", err
	}
	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET totp_pending_secret = $1 WHERE id = $2`, secret, userID); err != nil {
		return "", fmt.Errorf("begin two factor: %w", err)
	}
	return secret, nil
}

// ConfirmTwoFactor checks the first code and switches the second factor on.
//
// It returns the recovery codes, which exist only in this return value: their
// hashes go to the database and the plain text is shown once.
func (s *Store) ConfirmTwoFactor(ctx context.Context, userID int64, code string) ([]string, error) {
	var pending string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT totp_pending_secret FROM users WHERE id = $1`, userID).Scan(&pending)
	if err != nil {
		return nil, fmt.Errorf("load pending secret: %w", err)
	}
	if pending == "" {
		return nil, ErrNoSecondFactor
	}
	step, ok := totp.Validate(pending, code, s.clock())
	if !ok {
		return nil, ErrBadCode
	}

	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return nil, err
	}

	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin confirm: %w", err)
	}
	defer tx.Rollback()

	// The pending secret becomes the real one and the slot is cleared, so a
	// stale enrolment cannot be confirmed twice.
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_secret = totp_pending_secret, totp_pending_secret = '',
		 totp_confirmed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
		 totp_last_step = $1 WHERE id = $2`, step, userID); err != nil {
		return nil, fmt.Errorf("confirm two factor: %w", err)
	}
	// Any codes from an earlier setup are void: they were printed against a
	// secret that no longer exists.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return nil, fmt.Errorf("clear old recovery codes: %w", err)
	}
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, h); err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit confirm: %w", err)
	}
	return codes, nil
}

// VerifyCode checks a code at sign-in and refuses one that was already used.
func (s *Store) VerifyCode(ctx context.Context, userID int64, code string) error {
	var secret string
	var lastStep int64
	var confirmed sql.NullString
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT totp_secret, totp_last_step, totp_confirmed_at FROM users WHERE id = $1`, userID).
		Scan(&secret, &lastStep, &confirmed)
	if err != nil {
		return fmt.Errorf("load secret for verification: %w", err)
	}
	if secret == "" || !confirmed.Valid {
		return ErrNoSecondFactor
	}

	step, ok := totp.Validate(secret, code, s.clock())
	if !ok {
		return ErrBadCode
	}
	// The same code twice is refused even though it is arithmetically valid:
	// the thirty-second window is exactly long enough for someone who read the
	// digits off a screen to type them in after their owner did.
	if step <= lastStep {
		return ErrBadCode
	}

	if _, err := s.DB.Write.ExecContext(ctx,
		`UPDATE users SET totp_last_step = $1 WHERE id = $2 AND totp_last_step < $1`,
		step, userID); err != nil {
		return fmt.Errorf("record used step: %w", err)
	}
	return nil
}

// UseRecoveryCode spends one of the printed codes.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, code string) error {
	hash := hashRecoveryCode(code)

	// The UPDATE is the check: doing it in one statement means two requests
	// racing on the same code cannot both succeed.
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE user_recovery_codes
		 SET used_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`, userID, hash)
	if err != nil {
		return fmt.Errorf("use recovery code: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrBadCode
	}
	return nil
}

// RegenerateRecoveryCodes issues a fresh set, voiding the old one.
func (s *Store) RegenerateRecoveryCodes(ctx context.Context, userID int64) ([]string, error) {
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return nil, err
	}

	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin regenerate: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return nil, fmt.Errorf("clear recovery codes: %w", err)
	}
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, h); err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit regenerate: %w", err)
	}
	return codes, nil
}

// DisableTwoFactor removes an account's second factor entirely.
//
// This is the way back for someone locked out. It is reachable from the command
// line, which needs access to the machine — the one thing an attacker who has
// the password and not the phone does not have.
func (s *Store) DisableTwoFactor(ctx context.Context, userID int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin disable: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_secret = '', totp_pending_secret = '',
		 totp_confirmed_at = NULL, totp_last_step = 0 WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("disable two factor: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear recovery codes: %w", err)
	}
	return tx.Commit()
}

// recoveryCodeAlphabet leaves out the characters people confuse when copying a
// code off paper: 0/O, 1/I/l, and the vowels that turn a random string into a
// word nobody wants printed on their wall.
const recoveryCodeAlphabet = "23456789bcdfghjkmnpqrstvwxz"

// generateRecoveryCodes returns the plain codes and their hashes.
func generateRecoveryCodes() (codes []string, hashes []string, err error) {
	for i := 0; i < RecoveryCodeCount; i++ {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, code)
		hashes = append(hashes, hashRecoveryCode(code))
	}
	return codes, hashes, nil
}

// randomRecoveryCode builds one code as two groups of five.
func randomRecoveryCode() (string, error) {
	const length = 10
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate recovery code: %w", err)
	}

	var b strings.Builder
	for i, v := range buf {
		if i == length/2 {
			b.WriteByte('-')
		}
		// The alphabet's length divides evenly enough into 256 that the modulo
		// bias is under half a percent — irrelevant against a 10-character code
		// from a 27-character alphabet, which is about 47 bits.
		b.WriteByte(recoveryCodeAlphabet[int(v)%len(recoveryCodeAlphabet)])
	}
	return b.String(), nil
}

// hashRecoveryCode normalises and hashes a code.
//
// SHA-256 without a work factor, unlike a password: a recovery code is 47 bits
// of randomness rather than something a person chose, so there is no dictionary
// to run against it and nothing for slowness to buy.
func hashRecoveryCode(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
