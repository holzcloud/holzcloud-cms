package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/sharelink"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// ShareLinkData shows a freshly minted preview link.
type ShareLinkData struct {
	web.LayoutData
	WebsiteID int64
	PageID    int64
	Title     string
	URL       string
	Expires   time.Time
}

// HandlePageShare mints a link that shows an unpublished page to someone
// without an account.
//
// The alternative people reach for otherwise is publishing a draft "just for a
// minute" so a customer can look at it, which is how a half-finished price list
// ends up in a search index for a year.
func (h *Handler) HandlePageShare(w http.ResponseWriter, r *http.Request) error {
	websiteID, ws, ok, err := h.lookupWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	pageID, err := strconv.ParseInt(r.PathValue("pageID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	pg, err := h.pages.GetPage(r.Context(), pageID)
	if err != nil {
		return err
	}
	if pg == nil || pg.WebsiteID != websiteID || pg.InTrash() {
		http.NotFound(w, r)
		return nil
	}
	if h.share == nil {
		return fmt.Errorf("share links are not configured")
	}

	lifetime := sharelink.DefaultLifetime
	if days, err := strconv.Atoi(r.FormValue("tage")); err == nil && days > 0 {
		lifetime = time.Duration(days) * 24 * time.Hour
		if lifetime > sharelink.MaxLifetime {
			lifetime = sharelink.MaxLifetime
		}
	}
	expires := time.Now().UTC().Add(lifetime)
	token := h.share.Token(pg.ID, expires)

	// An absolute URL, because the link is going to be pasted into an email and
	// a relative path would be useless there.
	base := h.publicBase(r, ws.ID)

	data := ShareLinkData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Vorschaulink – %s", pg.Title)),
		WebsiteID:  websiteID,
		PageID:     pg.ID,
		Title:      pg.Title,
		URL:        base + sharelink.Path(token),
		Expires:    expires,
	}
	data.ActiveNav = "pages"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "share_link", data)
}

// publicBase is the address a visitor would type, for a link that leaves the
// admin.
//
// The website's primary domain when it has one; otherwise the host the admin is
// being served from, which at least works on the machine it was copied from.
func (h *Handler) publicBase(r *http.Request, websiteID int64) string {
	scheme := "http"
	if h.cfg.Secure || r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if primary, err := h.domains.PrimaryDomain(r.Context(), websiteID); err == nil && primary != "" {
		host = primary
	}
	return scheme + "://" + host
}
