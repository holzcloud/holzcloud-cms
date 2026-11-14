package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/dsr"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The 48-hour warning of the retention job on the dashboard (Phase 33.2,
// plan 07, D-05, D-09). Every helper of this file starts with stalled.

// stalledSince sets the moment the warning counts from on an installation
// whose job never succeeded.
func stalledSince(t *testing.T, database *db.DB, at time.Time) {
	t.Helper()
	if _, err := database.Write.Exec(`INSERT INTO app_settings (key, value) VALUES ('retention.since', $1)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, at.UTC().Format("2006-01-02T15:04:05Z")); err != nil {
		t.Fatalf("set retention.since: %v", err)
	}
}

// stalledDashboard renders the dashboard for a user of the role.
func stalledDashboard(t *testing.T, h *Handler, sm *scs.SessionManager, userID int64, role string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), auth.SessionKeyUserID, userID)
		sm.Put(r.Context(), auth.SessionKeyUserRole, role)
		if err := h.HandleDashboard(w, r); err != nil {
			t.Errorf("HandleDashboard: %v", err)
		}
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// stalledSentence is the beginning both warnings share.
const stalledSentence = "Expired personal data has not been deleted for more than 48 hours"

func TestAnAdministratorIsWarnedAboutAStalledRetention(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	h.SetDataRights(dsr.NewService(database))
	users := user.NewStore(database, cheapHashing)
	adminID, err := users.Create(t.Context(), "Admin", "chef@example.com", "passwort123", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	editorID := editorWithoutPublishing(t, database, ws.ID)

	// Installed three days ago and never ran.
	stalledSince(t, database, time.Now().Add(-72*time.Hour))
	body := stalledDashboard(t, h, sm, adminID, user.RoleAdmin)
	if !strings.Contains(body, retentionSays(stalledSentence)) {
		t.Fatal("an administrator's dashboard does not warn about a job that never ran")
	}
	if !strings.Contains(body, "journalctl -u holzcloud") {
		t.Error("the warning does not name the journal to look in")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("the warning is not an alert")
	}
	if !strings.Contains(body, `href="`+retentionAddress(ws.ID)+`"`) || !strings.Contains(body, retentionSays("Open the Retention screen")) {
		t.Errorf("the warning has no link to a Retention screen (%s)", retentionAddress(ws.ID))
	}
	if strings.Contains(body, "has not finished successfully since") {
		t.Error("a job that never ran is described as one that stopped succeeding")
	}

	if body := stalledDashboard(t, h, sm, editorID, user.RoleEditor); strings.Contains(body, retentionSays(stalledSentence)) {
		t.Error("an editor's dashboard shows the retention warning")
	}

	// A success three days ago: the warning names when.
	last := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	if _, err := database.Write.Exec(`INSERT INTO retention_runs (started_at, finished_at, status) VALUES ($1, $1, 'ok')`,
		last.Format("2006-01-02T15:04:05Z")); err != nil {
		t.Fatal(err)
	}
	body = stalledDashboard(t, h, sm, adminID, user.RoleAdmin)
	if !strings.Contains(body, "has not finished successfully since "+last.Format("02.01.2006 15:04")) {
		t.Error("the warning does not say since when the job has not succeeded")
	}

	// A success an hour ago: no warning for anybody.
	if _, err := database.Write.Exec(`INSERT INTO retention_runs (started_at, finished_at, status) VALUES ($1, $1, 'ok')`,
		time.Now().UTC().Add(-time.Hour).Format("2006-01-02T15:04:05Z")); err != nil {
		t.Fatal(err)
	}
	for _, who := range []struct {
		id   int64
		role string
	}{{adminID, user.RoleAdmin}, {editorID, user.RoleEditor}} {
		if body := stalledDashboard(t, h, sm, who.id, who.role); strings.Contains(body, retentionSays(stalledSentence)) {
			t.Errorf("%s: the dashboard warns although the job succeeded an hour ago", who.role)
		}
	}
}
