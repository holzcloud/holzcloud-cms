package admin

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/bundle"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// ImportReportData shows what an import did.
type ImportReportData struct {
	web.LayoutData
	Report *bundle.Report
}

// bundleStores gathers the readers the export and import need.
func (h *Handler) bundleStores() bundle.Stores {
	return bundle.Stores{
		Domains:    h.domains,
		Pages:      h.pages,
		Menus:      h.menuStore,
		Snippets:   h.snippets,
		Terms:      h.terms,
		Media:      h.mediaStore,
		Fields:     h.fields,
		Kinds:      h.kinds,
		BlockTypes: h.blockTypes,
		DataDir:    h.cfg.DataDir,
	}
}

// HandleWebsiteExport streams a website out as an archive.
func (h *Handler) HandleWebsiteExport(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}

	filename := exportFilename(ws.Name)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	// The archive is built while it is written, so nothing may cache it and
	// nothing can know its length in advance.
	w.Header().Set("Cache-Control", "no-store")

	if err := bundle.Export(r.Context(), h.bundleStores(), websiteID, appVersion, w); err != nil {
		// The headers are already out, so there is no honest way to turn this
		// into an error page: the download will simply be short. Logging is
		// what makes it diagnosable.
		slog.Error("website export failed", "err", err, "website", websiteID)
		return nil
	}
	return nil
}

// appVersion is stamped into an archive so a puzzling import can be traced back
// to what wrote the file. It is set from main at startup.
var appVersion = "dev"

// SetVersion records the build version for export manifests.
func SetVersion(v string) { appVersion = v }

// exportFilename builds a name that sorts by date and survives every filesystem.
func exportFilename(siteName string) string {
	slug := page.Slugify(siteName)
	if slug == "" {
		slug = "website"
	}
	return fmt.Sprintf("%s-%s.holzcloud.zip", slug, time.Now().UTC().Format("2006-01-02"))
}

// HandleWebsiteImport creates a website from an uploaded archive.
func (h *Handler) HandleWebsiteImport(w http.ResponseWriter, r *http.Request) error {
	// An archive is bigger than a single upload: it holds every photo of a
	// site. Ten times the media limit is generous and still bounded.
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxMediaSize*10)

	file, _, err := r.FormFile("bundle")
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Datei zu groß oder nicht ausgewählt")
		return h.redirect(w, r, "/admin/websites")
	}
	defer file.Close()

	// zip needs to seek, so the archive is read into memory. It is bounded by
	// the reader above, which is what keeps the server out of the swap.
	data, err := io.ReadAll(file)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), "Die Datei konnte nicht gelesen werden")
		return h.redirect(w, r, "/admin/websites")
	}

	name := strings.TrimSpace(r.FormValue("name"))
	report, err := bundle.Import(r.Context(), h.bundleStores(),
		bytes.NewReader(data), int64(len(data)), name)
	if err != nil {
		web.SetFlashError(h.sm, r.Context(), web.Titlef(r, "Import fehlgeschlagen: %s", err))
		return h.redirect(w, r, "/admin/websites")
	}

	// A new site has no domain, which is deliberate — but it also means it is
	// unreachable until one is added, and that is worth saying immediately.
	h.resolver.InvalidateCache()

	data2 := ImportReportData{
		LayoutData: web.NewLayoutData(r, h.sm, "Import abgeschlossen"),
		Report:     report,
	}
	data2.ActiveNav = "websites"
	return web.RenderAdmin(w, h.templates, r, "import_report", data2)
}
