package wxr

import (
	"strings"
	"testing"
)

const beispiel = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:content="http://purl.org/rss/1.0/modules/content/"
  xmlns:excerpt="http://wordpress.org/export/1.2/excerpt/"
  xmlns:wp="http://wordpress.org/export/1.2/">
<channel>
  <title>Hof Sonnenberg</title>
  <item>
    <title>Über uns</title>
    <wp:post_name>ueber-uns</wp:post_name>
    <content:encoded><![CDATA[<p>Wir halten Schafe.</p>[gallery ids="4,5"]<img src="https://alt.example/wp-content/2020/hof.jpg">]]></content:encoded>
    <excerpt:encoded><![CDATA[Kurz gesagt.]]></excerpt:encoded>
    <wp:post_type>page</wp:post_type>
    <wp:status>publish</wp:status>
    <wp:post_date_gmt>2020-05-03 08:30:00</wp:post_date_gmt>
    <category domain="category"><![CDATA[Uncategorized]]></category>
  </item>
  <item>
    <title>Wolle 2021</title>
    <wp:post_name>wolle-2021</wp:post_name>
    <content:encoded><![CDATA[<p>Geschoren.</p>]]></content:encoded>
    <wp:post_type>post</wp:post_type>
    <wp:status>draft</wp:status>
    <category domain="post_tag"><![CDATA[Wolle]]></category>
    <category domain="category"><![CDATA[Hof]]></category>
  </item>
  <item>
    <title>hof.jpg</title>
    <wp:post_type>attachment</wp:post_type>
    <wp:status>inherit</wp:status>
    <wp:attachment_url>https://alt.example/wp-content/2020/hof.jpg</wp:attachment_url>
  </item>
  <item>
    <title>Papierkorb</title>
    <wp:post_type>post</wp:post_type>
    <wp:status>trash</wp:status>
  </item>
</channel>
</rss>`

func TestParse(t *testing.T) {
	export, err := Parse(strings.NewReader(beispiel))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if export.SiteTitle != "Hof Sonnenberg" {
		t.Errorf("Titel = %q", export.SiteTitle)
	}
	if len(export.Items) != 2 {
		t.Fatalf("%d entries, expected 2: %+v", len(export.Items), export.Items)
	}
	// Anhang und Papierkorb sind kein Inhalt.
	if export.Skipped != 2 {
		t.Errorf("skipped = %d, expected 2", export.Skipped)
	}

	page := export.Items[0]
	if page.Kind != "page" || !page.Published || page.Slug != "ueber-uns" {
		t.Errorf("Seite = %+v", page)
	}
	if page.Excerpt != "Kurz gesagt." {
		t.Errorf("Kurzfassung = %q", page.Excerpt)
	}
	if strings.Contains(page.HTML, "[gallery") {
		t.Errorf("the shortcode is still in the text: %q", page.HTML)
	}
	if !strings.Contains(page.HTML, "Wir halten Schafe") {
		t.Errorf("der Text fehlt: %q", page.HTML)
	}
	if page.Date.Year() != 2020 || page.Date.Month() != 5 {
		t.Errorf("Datum = %v", page.Date)
	}
	// "Uncategorized" ist keine Angabe, sondern die Abwesenheit einer.
	if len(page.Terms) != 0 {
		t.Errorf("terms of the page = %v", page.Terms)
	}

	post := export.Items[1]
	if post.Kind != "post" || post.Published {
		t.Errorf("Beitrag = %+v", post)
	}
	if len(post.Terms) != 2 {
		t.Errorf("terms = %v, expected Wolle and Hof", post.Terms)
	}

	// Die Bilder werden aufgezählt, nicht geholt: einmal aus dem Anhang und
	// einmal aus dem Text, und beides ist dieselbe Datei.
	if len(export.MediaURLs) != 1 || export.MediaURLs[0] != "https://alt.example/wp-content/2020/hof.jpg" {
		t.Errorf("Medien = %v", export.MediaURLs)
	}
}

func TestParseLehntUnsinnAb(t *testing.T) {
	if _, err := Parse(strings.NewReader("kein XML")); err == nil {
		t.Error("a file with no XML was accepted")
	}
	if _, err := Parse(strings.NewReader(`<rss><channel><title>Leer</title></channel></rss>`)); err == nil {
		t.Error("a file with no entries was accepted")
	}
}
