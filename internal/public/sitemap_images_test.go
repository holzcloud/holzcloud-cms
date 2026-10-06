package public

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

func seedMedia(t *testing.T, database *db.DB, websiteID int64, filename, mime string) *media.Media {
	t.Helper()
	m, err := media.NewStore(database).Create(context.Background(), websiteID, filename, filename, mime, 10, "h-"+filename+fmt.Sprint(websiteID))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func seedImagePage(t *testing.T, database *db.DB, websiteID int64, slug, status, blocks, html string, featured *int64) {
	t.Helper()
	_, err := page.NewStore(database).CreatePage(context.Background(), page.PageCreate{
		WebsiteID: websiteID, Title: slug, Slug: slug, Markdown: "x", HTML: html,
		Status: status, Blocks: blocks, Meta: page.PageMeta{FeaturedMediaID: featured},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSitemapImages(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	other := seedWebsite(t, database, "Andere")

	feat := seedMedia(t, database, ws.ID, "feat.jpg", "image/jpeg")
	blk := seedMedia(t, database, ws.ID, "blk.png", "image/png")
	gal := seedMedia(t, database, ws.ID, "gal.webp", "image/webp")
	seedMedia(t, database, ws.ID, "md.gif", "image/gif")
	pdf := seedMedia(t, database, ws.ID, "doc.pdf", "application/pdf")
	foreign := seedMedia(t, database, other.ID, "fremd.jpg", "image/jpeg")
	hidden := seedMedia(t, database, ws.ID, "entwurf.jpg", "image/jpeg")
	if err := media.NewStore(database).UpdateMeta(context.Background(), feat.ID, `Alt <&> "x"`, `Cap </image:caption> & co`); err != nil {
		t.Fatal(err)
	}

	blocks := fmt.Sprintf(`[{"typ":"bild","medium":%d},{"typ":"bild","medium":%d},{"typ":"galerie","eintraege":[{"medium":%d},{"medium":%d},{"medium":%d}]}]`,
		blk.ID, blk.ID, gal.ID, foreign.ID, pdf.ID)
	htmlBody := fmt.Sprintf(`<p><img src="/media/%d/md.gif"><img src="/media/%d/md.gif"><img src="/media/%d/fremd.jpg"><img src="/media/%d/unbekannt.jpg"></p>`,
		ws.ID, ws.ID, other.ID, ws.ID)
	seedImagePage(t, database, ws.ID, "bilder", "published", blocks, htmlBody, &feat.ID)
	seedImagePage(t, database, ws.ID, "entwurf", "draft", "",
		fmt.Sprintf(`<img src="/media/%d/entwurf.jpg">`, ws.ID), &hidden.ID)

	rec, err := request(h.HandleSitemap, ws, "GET", "/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	var doc struct {
		URLs []struct {
			Loc    string `xml:"loc"`
			Images []struct {
				Loc     string `xml:"loc"`
				Title   string `xml:"title"`
				Caption string `xml:"caption"`
			} `xml:"image"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, body)
	}
	if !strings.Contains(body, `xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"`) {
		t.Errorf("image namespace missing:\n%s", body)
	}
	if !strings.Contains(body, "<image:image>") || !strings.Contains(body, "<image:loc>") {
		t.Errorf("prefixed image elements missing:\n%s", body)
	}
	for _, bad := range []string{"fremd.jpg", "doc.pdf", "entwurf.jpg", "unbekannt"} {
		if strings.Contains(body, bad) {
			t.Errorf("%s must not be listed:\n%s", bad, body)
		}
	}
	base := fmt.Sprintf("http://demo.test/media/%d/", ws.ID)
	want := []string{base + "feat.jpg", base + "blk.png", base + "gal.webp", base + "md.gif"}
	var got []string
	for _, u := range doc.URLs {
		if strings.HasSuffix(u.Loc, "/bilder") {
			for _, im := range u.Images {
				got = append(got, im.Loc)
				if strings.HasSuffix(im.Loc, "feat.jpg") && (im.Title != `Alt <&> "x"` || !strings.HasPrefix(im.Caption, "Cap </image:caption>")) {
					t.Errorf("title/caption lost: %+v", im)
				}
			}
		} else if len(u.Images) > 0 {
			t.Errorf("%s must carry no images", u.Loc)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("images = %v; want %v", got, want)
	}
}

func TestSitemapImagesAreCapped(t *testing.T) {
	h, database := newTestHandler(t)
	ws := seedWebsite(t, database, "Test Site")
	var sb strings.Builder
	for i := 0; i < 1005; i++ {
		name := fmt.Sprintf("b%04d.jpg", i)
		seedMedia(t, database, ws.ID, name, "image/jpeg")
		fmt.Fprintf(&sb, `<img src="/media/%d/%s">`, ws.ID, name)
	}
	seedImagePage(t, database, ws.ID, "viele", "published", "", sb.String(), nil)

	rec, err := request(h.HandleSitemap, ws, "GET", "/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(rec.Body.String(), "<image:loc>"); n != 1000 {
		t.Errorf("image count = %d; want 1000", n)
	}
}
