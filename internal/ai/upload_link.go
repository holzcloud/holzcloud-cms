package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Upload links: a file that does not travel inside the call.
//
// upload_media carries its file as base64 in the arguments, and for a program
// that is fine. For an assistant it is not: every byte of the file becomes text
// the model has to write out, and a screenshot of 200 KB is 270 000 characters
// of it. In practice an assistant shrinks the picture until it fits, and the
// website ends up with blurry images nobody chose.
//
// create_upload_link hands out an address instead. The assistant's machine
// sends the file there directly — curl --upload-file — and the file goes
// through storeUpload, the same intake as upload_media, with the same checks,
// the same activity entry and the same answer.
//
// The address is the permission, so it is made to be worth little: it is
// random, used once, bound to the key, the website and the file name it was
// issued for, and gone after UploadLinkLifetime. The key is looked up again
// when the file arrives, so revoking it also ends its open links. Nothing is
// fetched by this server: the file is pushed to it, as with every other upload.

// UploadLinkLifetime is how long an upload link waits for its file.
const UploadLinkLifetime = 15 * time.Minute

// maxOpenUploadLinks bounds the links waiting at once, over all keys. A link
// costs a few hundred bytes, but an assistant in a loop should not be able to
// grow the map without end.
const maxOpenUploadLinks = 200

// UploadPath is where upload links point; the secret follows it.
const UploadPath = "/ai/upload/"

// Uploads holds the open upload links. It lives in memory: the links last
// minutes, and a restart that forgets them costs an assistant one more call.
type Uploads struct {
	// Secure is the deployment's HOLZCLOUD_SECURE: whether a link is written
	// with https although the request reached this process as plain http behind
	// the cluster's proxy.
	Secure bool

	mu      sync.Mutex
	tickets map[string]uploadTicket // by hash of the secret
	now     func() time.Time
}

type uploadTicket struct {
	scope   Scope
	website int64
	name    string
	alt     string
	caption string
	expires time.Time
}

// NewUploads creates the store of upload links.
func NewUploads(secure bool) *Uploads {
	return &Uploads{Secure: secure, tickets: map[string]uploadTicket{}, now: time.Now}
}

var errTooManyUploadLinks = errors.New("too many upload links are open; use or let the open ones expire first")

// issue opens a link and returns its secret.
func (u *Uploads) issue(t uploadTicket) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)

	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
	for k, open := range u.tickets {
		if now.After(open.expires) {
			delete(u.tickets, k)
		}
	}
	if len(u.tickets) >= maxOpenUploadLinks {
		return "", time.Time{}, errTooManyUploadLinks
	}
	t.expires = now.Add(UploadLinkLifetime)
	u.tickets[secretHash(secret)] = t
	return secret, t.expires, nil
}

// take hands out a link's ticket and closes the link, whatever happens to the
// file after. A second attempt needs a new link: a link that stayed open after
// a failure would be one somebody else could still use.
func (u *Uploads) take(secret string) (uploadTicket, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	key := secretHash(secret)
	t, ok := u.tickets[key]
	if !ok {
		return uploadTicket{}, false
	}
	delete(u.tickets, key)
	if u.now().After(t.expires) {
		return uploadTicket{}, false
	}
	return t, true
}

func secretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func createUploadLink(d Deps) Tool {
	return Tool{
		Name:   "create_upload_link",
		Writes: true,
		Description: "Opens a one-time address to upload one file into a website's media library " +
			"without sending its bytes through this call — the way to upload anything larger than " +
			"a few kilobytes, such as a sharp screenshot or a photograph. Send the file there with " +
			"an HTTP PUT of its raw bytes, for example: curl --fail --upload-file bild.webp '<upload_url>'. " +
			"The answer to that request is the stored file, as upload_media would answer. The link " +
			"works once and expires after 15 minutes. The file goes through the same checks as " +
			"upload_media. Give every picture a description (alt_text) of what it shows.",
		InputSchema: Schema{
			Type: "object",
			Properties: map[string]Property{
				"website": websiteProp(),
				"file_name": {Type: "string", Description: "the file's name with its ending, " +
					"such as workshop.jpg"},
				"alt_text": {Type: "string", Description: "what the picture shows, for someone " +
					"who cannot see it"},
				"caption": {Type: "string", Description: "the visible text under the picture, " +
					"where the design shows one"},
			},
			Required: []string{"website", "file_name"},
		},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Name    string `json:"file_name"`
				Alt     string `json:"alt_text"`
				Caption string `json:"caption"`
			}
			if err := c.Into(&a); err != nil {
				return nil, err
			}
			if err := c.Scope.MaySee(a.Website); err != nil {
				return nil, err
			}
			if _, ok := d.Ops.(mediaOps); !ok || d.Uploads == nil {
				return nil, errNoMediaOps
			}
			if strings.TrimSpace(a.Name) == "" {
				return nil, errors.New("the file needs a name")
			}
			if d.Domains != nil {
				ws, err := d.Domains.GetWebsite(c.Ctx, a.Website)
				if err != nil {
					return nil, err
				}
				if ws == nil {
					return nil, errors.New("there is no such website")
				}
			}

			secret, expires, err := d.Uploads.issue(uploadTicket{
				scope: c.Scope, website: a.Website, name: a.Name, alt: a.Alt, caption: a.Caption,
			})
			if err != nil {
				return nil, err
			}
			scheme := "http"
			if d.Uploads.Secure {
				scheme = "https"
			}
			link := scheme + "://" + c.Host + UploadPath + secret
			c.Log.Info("ai opened upload link", "key", c.Scope.Name, "website", a.Website, "file", a.Name)
			return map[string]any{
				"upload_url": link,
				"method":     "PUT",
				"expires_at": expires.UTC().Format(time.RFC3339),
				"max_bytes":  max(d.Limits.MaxMediaSize, d.Limits.MaxVideoSize),
				"example":    "curl --fail --upload-file " + shellQuote(a.Name) + " " + shellQuote(link),
			}, nil
		},
	}
}

// shellQuote wraps a word in single quotes for the example command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// UploadHandler receives the files sent to upload links: PUT or POST of the
// raw bytes to UploadPath plus the secret.
//
// Like /ai it reads no cookie and sets no cross-origin header; the secret in
// the address is the only way in.
func UploadHandler(d Deps, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	limit := max(d.Limits.MaxMediaSize, d.Limits.MaxVideoSize)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		answer := func(status int, v any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(v)
		}
		refuse := func(status int, msg string) { answer(status, map[string]any{"error": msg}) }

		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			w.Header().Set("Allow", "PUT, POST")
			refuse(http.StatusMethodNotAllowed, "send the file with PUT")
			return
		}
		if d.Uploads == nil {
			refuse(http.StatusNotFound, "no such upload link")
			return
		}
		secret := strings.TrimPrefix(r.URL.Path, UploadPath)
		t, ok := d.Uploads.take(secret)
		if !ok {
			// One answer for unknown, used and expired: which of the three it
			// was is nothing a stranger needs to learn.
			refuse(http.StatusNotFound, "this upload link does not exist, was used or has expired")
			return
		}

		// The key again, as it is now: revoked, expired or turned read-only
		// since the link was opened, and the link is worth nothing.
		scope := t.scope
		if d.Tokens != nil {
			if err := stillValid(r.Context(), d.Tokens, &scope); err != nil {
				refuse(http.StatusForbidden, err.Error())
				return
			}
		}
		if err := scope.MayWrite(); err != nil {
			refuse(http.StatusForbidden, err.Error())
			return
		}
		if err := scope.MaySee(t.website); err != nil {
			refuse(http.StatusForbidden, err.Error())
			return
		}

		if limit > 0 && r.ContentLength > limit {
			refuse(http.StatusRequestEntityTooLarge, "the file is larger than this installation accepts")
			return
		}
		content, err := io.ReadAll(http.MaxBytesReader(w, r.Body, max(limit, 1)))
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				refuse(http.StatusRequestEntityTooLarge, "the file is larger than this installation accepts")
				return
			}
			refuse(http.StatusBadRequest, "the file cannot be read")
			return
		}
		if len(content) == 0 {
			refuse(http.StatusBadRequest, "the file is empty")
			return
		}

		c := Call{Ctx: r.Context(), Scope: scope, Log: log, Host: r.Host}
		out, err := storeUpload(d, c, t.website, t.name, content, t.alt, t.caption)
		if err != nil {
			log.Warn("ai upload link failed", "key", scope.Name, "err", err)
			refuse(http.StatusUnprocessableEntity, err.Error())
			return
		}
		if d.Tokens != nil {
			d.Tokens.Touch(r.Context(), scope.TokenID)
		}
		answer(http.StatusCreated, out)
	})
}

// stillValid reads a key again and narrows the scope to what it may do now.
func stillValid(ctx context.Context, tokens *Store, scope *Scope) error {
	tok, err := tokens.Get(ctx, scope.TokenID)
	if err != nil || tok == nil {
		return ErrBadToken
	}
	if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
		return ErrExpired
	}
	scope.CanWrite = tok.CanWrite || tok.Admin
	scope.Admin = tok.Admin
	scope.WebsiteID = tok.WebsiteID
	return nil
}
