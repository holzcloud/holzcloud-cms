package admin

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
)

// An agent that wrote a theme reads, in upload_template's answer, which
// contract this installation documents and which one the theme declared — the
// same two facts `holzcloud template check` prints. Through the real handler,
// so the answer is read from the files actually installed.
func TestUploadTemplateAnswersWithTheContract(t *testing.T) {
	h, _, database, _ := newTestAdmin(t)
	h.cfg.MaxTemplateSize = 10 << 20

	tokens := ai.NewStore(database)
	admin, _, err := tokens.IssueLevel(context.Background(), "verwaltung", 0, ai.LevelAdmin, 0)
	if err != nil {
		t.Fatal(err)
	}
	srv := ai.NewServer(tokens, "Test", slog.New(slog.DiscardHandler), ai.Tools(ai.Deps{
		Domains: h.domains, Pages: h.pages, Media: h.mediaStore, Fields: h.fields, Ops: h,
		Limits: ai.Limits{DataDir: h.cfg.DataDir},
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	r := &structureRig{t: t, h: h, ts: ts, admin: admin}

	upload := func(name string, extra map[string]string) (map[string]any, bool) {
		return r.call(admin, "upload_template", map[string]any{
			"name": name, "archive": base64.StdEncoding.EncodeToString(themeZip(t, extra)),
		})
	}

	got := r.must(admin, "upload_template", map[string]any{
		"name": "Erklaert", "archive": base64.StdEncoding.EncodeToString(
			themeZip(t, map[string]string{"theme.json": `{"contract": "1.0"}`})),
	})
	if got["contract"] != "1.0" || got["declares"] != "1.0" {
		t.Errorf("a theme declaring 1.0: contract %v, declares %v", got["contract"], got["declares"])
	}

	got, failed := upload("Schweigsam", nil)
	if failed {
		t.Fatalf("a theme without theme.json was refused: %v", got["text"])
	}
	if got["contract"] != "1.0" || got["declares"] != "" {
		t.Errorf("a theme declaring nothing: contract %v, declares %q", got["contract"], got["declares"])
	}

	got, failed = upload("Kaputt", map[string]string{"theme.json": `{"contract": "eins"}`})
	if !failed {
		t.Fatalf("a malformed theme.json was installed: %v", got)
	}
	if text, _ := got["text"].(string); !strings.Contains(text, "theme.json") {
		t.Errorf("the refusal does not name theme.json: %q", text)
	}
	if stored, _ := h.tmplStore.GetBySlug(context.Background(), "kaputt"); stored != nil {
		t.Error("the refused template was recorded")
	}
	if _, err := os.Stat(filepath.Join(h.cfg.DataDir, "templates", "kaputt")); !os.IsNotExist(err) {
		t.Errorf("the refused template is on disk: %v", err)
	}
}
