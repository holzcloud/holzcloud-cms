package media

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// Images held back between the upload and the answer to "scale down?".
//
// On disk and not in SQLite: up to twenty images at the size limit each would
// be blobs in the single-writer file that also carries the sessions. The shape
// is internal/csvimport's — a token, a per-user check, the expired/foreign
// split, a prune — without its table.
//
// Layout: <root>/<website id>/<token>/ with a manifest.json and the held bytes
// as files named by their index. The root is a sibling of media/, so the public
// media route can never serve anything in it.

// StagingRetention is how long a staged upload waits for its answer before the
// prune job removes it.
const StagingRetention = time.Hour

var (
	// ErrStagingExpired says there is no such staged upload for this website:
	// it was answered, it was pruned, the token is not one, or it belongs to
	// another website. These are not told apart, because telling them apart is
	// what lets a token be probed across websites.
	ErrStagingExpired = errors.New("the staged upload has expired")
	// ErrStagingForeign says the staged upload exists on this website but was
	// made by another user.
	ErrStagingForeign = errors.New("the staged upload belongs to another user")
)

// tokenForm is the only shape of token that is looked up. Checked before any
// path is built, which is what keeps "../" out of the file system.
var tokenForm = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Staging is the store of held-back uploads.
type Staging struct{ root string }

// NewStaging returns the store under dataDir/media-staging.
func NewStaging(dataDir string) *Staging {
	return &Staging{root: filepath.Join(dataDir, "media-staging")}
}

type stagedManifest struct {
	WebsiteID int64        `json:"website_id"`
	UserID    int64        `json:"user_id"`
	Created   time.Time    `json:"created"`
	Files     []StagedFile `json:"files"`
}

// StagedFile describes one held image.
type StagedFile struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int64  `json:"size"`
}

// Put holds the intakes for this website and user and returns the token that
// finds them again. The directory appears under its final name only when it is
// complete, so a half-written staging is never visible to Get or Prune's
// bookkeeping.
func (s *Staging) Put(websiteID, userID int64, intakes []*Intake) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("stage upload: %w", err)
	}
	token := hex.EncodeToString(raw)

	siteDir := filepath.Join(s.root, strconv.FormatInt(websiteID, 10))
	if err := os.MkdirAll(siteDir, 0o700); err != nil {
		return "", fmt.Errorf("stage upload: %w", err)
	}
	tmp := filepath.Join(siteDir, token+".tmp")
	if err := os.Mkdir(tmp, 0o700); err != nil {
		return "", fmt.Errorf("stage upload: %w", err)
	}
	fail := func(err error) (string, error) {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("stage upload: %w", err)
	}

	m := stagedManifest{WebsiteID: websiteID, UserID: userID, Created: time.Now().UTC()}
	for i, in := range intakes {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(in.content))
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, strconv.Itoa(i)), in.content, 0o600); err != nil {
			return fail(err)
		}
		m.Files = append(m.Files, StagedFile{
			Name: in.OriginalName, Width: cfg.Width, Height: cfg.Height, Size: int64(len(in.content)),
		})
	}
	body, err := json.Marshal(m)
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), body, 0o600); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp, filepath.Join(siteDir, token)); err != nil {
		return fail(err)
	}
	return token, nil
}

// Staged is a held upload that was found.
type Staged struct {
	dir      string
	manifest stagedManifest
}

// Files lists what is held, in upload order.
func (st *Staged) Files() []StagedFile {
	return append([]StagedFile(nil), st.manifest.Files...)
}

// Content is the bytes of the i-th file.
func (st *Staged) Content(i int) ([]byte, error) {
	if i < 0 || i >= len(st.manifest.Files) {
		return nil, ErrStagingExpired
	}
	return os.ReadFile(filepath.Join(st.dir, strconv.Itoa(i)))
}

// Discard removes what is held.
func (st *Staged) Discard() { os.RemoveAll(st.dir) }

// Get finds a held upload without claiming it, for showing the question.
func (s *Staging) Get(websiteID int64, token string, userID int64) (*Staged, error) {
	if !tokenForm.MatchString(token) {
		return nil, ErrStagingExpired
	}
	dir := filepath.Join(s.root, strconv.FormatInt(websiteID, 10), token)
	return s.open(dir, websiteID, userID)
}

func (s *Staging) open(dir string, websiteID, userID int64) (*Staged, error) {
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, ErrStagingExpired
	}
	var m stagedManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, ErrStagingExpired
	}
	if m.WebsiteID != websiteID {
		return nil, ErrStagingExpired
	}
	if m.UserID != userID {
		return nil, ErrStagingForeign
	}
	return &Staged{dir: dir, manifest: m}, nil
}

// Take is Get that also claims the upload: the directory is renamed to a
// ".taken" sibling, which only one caller can do, so a double submit gets the
// expired error instead of storing everything twice. The caller owns the result
// and Discards it.
func (s *Staging) Take(websiteID int64, token string, userID int64) (*Staged, error) {
	st, err := s.Get(websiteID, token, userID)
	if err != nil {
		return nil, err
	}
	taken := st.dir + ".taken"
	if err := os.Rename(st.dir, taken); err != nil {
		return nil, ErrStagingExpired
	}
	st.dir = taken
	return st, nil
}

// Prune removes staged uploads older than olderThan, including the leftovers of
// a crash (".tmp" and ".taken"), and says how many went.
func (s *Staging) Prune(olderThan time.Duration) (int, error) {
	sites, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("prune staging: %w", err)
	}
	cutoff := time.Now().Add(-olderThan)
	removed := 0
	for _, site := range sites {
		if !site.IsDir() {
			continue
		}
		siteDir := filepath.Join(s.root, site.Name())
		entries, err := os.ReadDir(siteDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil || !info.ModTime().Before(cutoff) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(siteDir, e.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
