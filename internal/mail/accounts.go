package mail

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// A website's own mail account.
//
// The installation's account comes from the environment and is the one every
// message used until websites could have their own. It still is the fallback:
// a website without a row in website_mail sends through it, and an invitation
// or a password link, which belong to no website, always do.
//
// The password is the one thing here that is a secret, and the one thing this
// package would rather not store. It has to: an operator running four websites
// types four passwords into the administration, and they have to be somewhere
// when the job sends at three in the morning. So the database holds ciphertext
// and the key stays in the environment (HOLZCLOUD_SECRET_KEY), where the
// payment key and the SMTP password of the installation already live. A copy of
// the database alone opens nothing.

// ErrNoSecretKey is returned when a password is to be stored or read and
// HOLZCLOUD_SECRET_KEY is not set.
var ErrNoSecretKey = errors.New("HOLZCLOUD_SECRET_KEY is not set")

// ErrPasswordUnreadable is returned when a stored password does not open with
// the present key: the key was changed, or the value was copied from another
// website. Sending stops until the password is entered again.
var ErrPasswordUnreadable = errors.New("the stored mail password cannot be decrypted with HOLZCLOUD_SECRET_KEY")

// Account is what the administration shows and edits. The password never
// leaves this package in the clear; HasPassword is all a screen learns.
type Account struct {
	WebsiteID   int64
	Host        string
	Port        int
	User        string
	HasPassword bool
	From        string
	FromName    string
	TLS         string
	UpdatedAt   string
}

// Accounts reads and writes the per-website accounts and picks the sender for
// a message.
type Accounts struct {
	db       *db.DB
	key      []byte
	fallback *Sender
}

// NewAccounts creates the store. secret is HOLZCLOUD_SECRET_KEY as configured,
// or empty; fallback is the installation's sender.
func NewAccounts(database *db.DB, secret string, fallback *Sender) *Accounts {
	a := &Accounts{db: database, fallback: fallback}
	if secret != "" {
		// Hashed rather than decoded: an operator pastes whatever openssl
		// printed, and every character of it should count. The prefix keeps
		// this key apart from anything else that might one day be derived
		// from the same variable.
		sum := sha256.Sum256([]byte("holzcloud website mail password\x00" + secret))
		a.key = sum[:]
	}
	return a
}

// CanStorePasswords reports whether a password can be saved at all.
func (a *Accounts) CanStorePasswords() bool { return a != nil && len(a.key) == 32 }

// Fallback is the installation's sender.
func (a *Accounts) Fallback() *Sender {
	if a == nil {
		return nil
	}
	return a.fallback
}

// Get returns the website's own account, or nil when it has none.
func (a *Accounts) Get(ctx context.Context, websiteID int64) (*Account, error) {
	acc, _, err := a.load(ctx, websiteID)
	return acc, err
}

func (a *Accounts) load(ctx context.Context, websiteID int64) (*Account, string, error) {
	if a == nil || a.db == nil || websiteID <= 0 {
		return nil, "", nil
	}
	var acc Account
	var sealed string
	err := a.db.Read.QueryRowContext(ctx,
		`SELECT website_id, host, port, username, password, from_addr, from_name, tls, updated_at
		 FROM website_mail WHERE website_id = $1`, websiteID).
		Scan(&acc.WebsiteID, &acc.Host, &acc.Port, &acc.User, &sealed,
			&acc.From, &acc.FromName, &acc.TLS, &acc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read mail account: %w", err)
	}
	acc.HasPassword = sealed != ""
	return &acc, sealed, nil
}

// Save stores the website's account. password is the new password; with
// keepPassword the stored one stays as it is, which is what an edit form sends
// when its password field was left empty.
func (a *Accounts) Save(ctx context.Context, acc Account, password string, keepPassword bool) error {
	if a == nil || a.db == nil {
		return ErrNotConfigured
	}
	if err := acc.validate(); err != nil {
		return err
	}
	sealed := ""
	if !keepPassword && password != "" {
		if !a.CanStorePasswords() {
			return ErrNoSecretKey
		}
		var err error
		if sealed, err = a.seal(acc.WebsiteID, password); err != nil {
			return err
		}
	}
	if keepPassword {
		_, err := a.db.Write.ExecContext(ctx,
			`INSERT INTO website_mail (website_id, host, port, username, from_addr, from_name, tls)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (website_id) DO UPDATE SET
			   host = excluded.host, port = excluded.port, username = excluded.username,
			   from_addr = excluded.from_addr, from_name = excluded.from_name, tls = excluded.tls,
			   updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
			acc.WebsiteID, acc.Host, acc.Port, acc.User, acc.From, acc.FromName, acc.TLS)
		return err
	}
	_, err := a.db.Write.ExecContext(ctx,
		`INSERT INTO website_mail (website_id, host, port, username, password, from_addr, from_name, tls)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (website_id) DO UPDATE SET
		   host = excluded.host, port = excluded.port, username = excluded.username,
		   password = excluded.password, from_addr = excluded.from_addr,
		   from_name = excluded.from_name, tls = excluded.tls,
		   updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		acc.WebsiteID, acc.Host, acc.Port, acc.User, sealed, acc.From, acc.FromName, acc.TLS)
	return err
}

// Delete removes the website's account; it sends through the installation's
// again afterwards.
func (a *Accounts) Delete(ctx context.Context, websiteID int64) error {
	if a == nil || a.db == nil {
		return nil
	}
	_, err := a.db.Write.ExecContext(ctx, `DELETE FROM website_mail WHERE website_id = $1`, websiteID)
	return err
}

// Own is one website with an account of its own, for the status screen.
type Own struct {
	WebsiteID int64
	Website   string
	Host      string
	From      string
}

// List returns every website that has its own account, by name.
func (a *Accounts) List(ctx context.Context) ([]Own, error) {
	if a == nil || a.db == nil {
		return nil, nil
	}
	rows, err := a.db.Read.QueryContext(ctx,
		`SELECT m.website_id, w.name, m.host, m.from_addr
		 FROM website_mail m JOIN websites w ON w.id = m.website_id
		 ORDER BY w.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list mail accounts: %w", err)
	}
	defer rows.Close()
	var out []Own
	for rows.Next() {
		var o Own
		if err := rows.Scan(&o.WebsiteID, &o.Website, &o.Host, &o.From); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Any reports whether at least one website has an account of its own.
func (a *Accounts) Any(ctx context.Context) bool {
	if a == nil || a.db == nil {
		return false
	}
	var n int
	if err := a.db.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM website_mail`).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// EnabledFor reports whether a message about this website would be sent:
// through its own account or the installation's.
func (a *Accounts) EnabledFor(ctx context.Context, websiteID int64) bool {
	if a == nil {
		return false
	}
	if acc, _ := a.Get(ctx, websiteID); acc != nil {
		return true
	}
	return a.fallback.Enabled()
}

// SenderFor returns the sender for a message about this website: its own
// account when it has one, the installation's otherwise. The result may be
// disabled — no account anywhere — which Send reports as ErrNotConfigured.
func (a *Accounts) SenderFor(ctx context.Context, websiteID int64) (*Sender, error) {
	if a == nil {
		return nil, nil
	}
	acc, sealed, err := a.load(ctx, websiteID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return a.fallback, nil
	}
	password := ""
	if sealed != "" {
		if !a.CanStorePasswords() {
			return nil, ErrNoSecretKey
		}
		if password, err = a.open(acc.WebsiteID, sealed); err != nil {
			return nil, err
		}
	}
	timeout := a.fallback.Config().Timeout
	return NewSender(Config{
		Host: acc.Host, Port: acc.Port, User: acc.User, Password: password,
		From: acc.From, FromName: acc.FromName, TLS: acc.TLS, Timeout: timeout,
	}), nil
}

// seal encrypts a password for its row.
func (a *Accounts) seal(websiteID int64, plain string) (string, error) {
	gcm, err := a.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), aad(websiteID))
	return base64.RawStdEncoding.EncodeToString(out), nil
}

func (a *Accounts) open(websiteID int64, sealed string) (string, error) {
	gcm, err := a.gcm()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", ErrPasswordUnreadable
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], aad(websiteID))
	if err != nil {
		return "", ErrPasswordUnreadable
	}
	return string(plain), nil
}

func (a *Accounts) gcm() (cipher.AEAD, error) {
	if !a.CanStorePasswords() {
		return nil, ErrNoSecretKey
	}
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// aad binds a sealed password to its row.
func aad(websiteID int64) []byte {
	return []byte("website_mail:" + strconv.FormatInt(websiteID, 10))
}

// Account errors. Plain English, like every error this package returns; the
// administration words them for the operator.
var (
	ErrAccountHost = errors.New("the mail server is missing")
	ErrAccountFrom = errors.New("the sender address is not an e-mail address")
	ErrAccountPort = errors.New("the port must lie between 1 and 65535")
	ErrAccountTLS  = errors.New("encryption must be starttls, tls or none")
)

func (acc *Account) validate() error {
	acc.Host = strings.TrimSpace(acc.Host)
	acc.User = strings.TrimSpace(acc.User)
	acc.From = strings.TrimSpace(acc.From)
	acc.FromName = header(acc.FromName)
	acc.TLS = strings.ToLower(strings.TrimSpace(acc.TLS))
	if acc.TLS == "" {
		acc.TLS = "starttls"
	}
	if acc.Port == 0 {
		acc.Port = 587
	}
	switch {
	case acc.Host == "" || acc.Host != header(acc.Host) || strings.ContainsAny(acc.Host, " /:"):
		return ErrAccountHost
	case validAddress(acc.From) != nil:
		return ErrAccountFrom
	case acc.Port < 1 || acc.Port > 65535:
		return ErrAccountPort
	}
	switch acc.TLS {
	case "starttls", "tls", "none":
	default:
		return ErrAccountTLS
	}
	return nil
}
