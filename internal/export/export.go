// Package export writes one website as plain files: the static export.
//
// It is not a second renderer. It asks the same http.Handler the server runs,
// with the website's own host name, and writes down what comes back — so a
// page looks in the export exactly as it looks when served, and a theme needs
// to know nothing about it. What it cannot write down it says: a protected
// page answers 401 and is left out, a search needs a query the export does
// not know, a form posts to a server that is not there. docs/offene-punkte.md
// called it "an explicitly reduced output", and the report is what makes it
// explicit.
package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// MaxFiles bounds one export. A website with more addresses than this is
// either very large or produces addresses without end, and in both cases a
// directory that keeps growing is the wrong answer.
const MaxFiles = 50000

// Options are what one export needs.
type Options struct {
	// Handler answers the requests — the server's own, fully wired.
	Handler http.Handler
	// Host is a domain of the website to export. The resolver decides by it
	// which website answers, exactly as for a visitor.
	Host string
	// Dir is where the files go. It must not exist yet or be empty: an export
	// over an old one would keep pages that were deleted since.
	Dir string
}

// Skip is one address the export met and did not write.
type Skip struct {
	URL    string
	Reason string
}

// Report is what an export did.
type Report struct {
	// Files are the written paths, relative to Dir, sorted.
	Files []string
	// Skipped are the addresses that were found and not written, sorted.
	Skipped []Skip
	// Forms are the written pages that carry a form. The form is in the file
	// and does nothing there, because nothing receives it.
	Forms []string
}

// Reasons a page is not written. Plain English on purpose: they are printed
// by the command line, which speaks English like every other subcommand.
const (
	ReasonServer   = "needs the running server"
	ReasonQuery    = "address with a query string"
	ReasonStatus   = "answered "
	ReasonLimit    = "limit of files reached"
	ReasonExternal = "redirects off this website"
)

// serverOnly are the prefixes that mean something only with a server behind
// them: the admin, the shop's cart and checkout, payments, share links and
// the unlock form of a protected page.
var serverOnly = []string{
	"/admin/", "/warenkorb", "/kasse", "/bestellung/", "/zahlung/",
	"/vorschau/", "/freischalten", "/preise", "/ai",
}

// notFoundProbe is an address no website has, asked for once so the export
// carries the site's own 404 page as 404.html — the name nearly every static
// host looks for.
const notFoundProbe = "/holzcloud-export-404-probe"

// Run exports the website Host names into Dir.
func Run(ctx context.Context, opt Options) (*Report, error) {
	if opt.Handler == nil || opt.Host == "" || opt.Dir == "" {
		return nil, errors.New("export: handler, host and directory are required")
	}
	if err := prepareDir(opt.Dir); err != nil {
		return nil, err
	}
	c := &crawler{
		opt:     opt,
		seen:    map[string]bool{},
		written: map[string]bool{},
		skipped: map[string]string{},
		forms:   map[string]bool{},
	}
	if err := c.run(ctx); err != nil {
		return nil, err
	}
	return c.report(), nil
}

func prepareDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("export: %s is not empty — an export over an old one would keep pages deleted since", dir)
	}
	return nil
}

type crawler struct {
	opt     Options
	queue   []string
	seen    map[string]bool
	written map[string]bool
	skipped map[string]string
	forms   map[string]bool
}

func (c *crawler) run(ctx context.Context) error {
	for _, seed := range []string{"/", "/sitemap.xml", "/robots.txt", "/feed.xml"} {
		c.enqueue(seed)
	}
	for len(c.queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := c.queue[0]
		c.queue = c.queue[1:]
		if len(c.written) >= MaxFiles {
			c.skipped[target] = ReasonLimit
			continue
		}
		if err := c.visit(ctx, target); err != nil {
			return err
		}
	}
	return c.notFoundPage(ctx)
}

// enqueue adds a site-relative address ("/pfad?query") once.
func (c *crawler) enqueue(target string) {
	// ?seite=1 is the list itself; asking twice would write it twice.
	if u, err := url.Parse(target); err == nil && u.RawQuery == "seite=1" {
		target = u.EscapedPath()
	}
	if c.seen[target] {
		return
	}
	c.seen[target] = true
	u, err := url.Parse(target)
	if err != nil {
		return
	}
	for _, p := range serverOnly {
		if u.Path == strings.TrimSuffix(p, "/") || strings.HasPrefix(u.Path, p) {
			c.skipped[target] = ReasonServer
			return
		}
	}
	if u.RawQuery != "" && !exportableQuery(u.Query()) {
		c.skipped[target] = ReasonQuery
		return
	}
	c.queue = append(c.queue, target)
}

// exportableQuery is the one query each address may carry and still become a
// file: a version (?v=, the file is the same without it) or a page number of
// a list (?seite=, which becomes a directory of its own).
func exportableQuery(q url.Values) bool {
	if len(q) != 1 {
		return false
	}
	if vs, ok := q["v"]; ok && len(vs) == 1 {
		return true
	}
	if vs, ok := q["seite"]; ok && len(vs) == 1 {
		n, err := strconv.Atoi(vs[0])
		return err == nil && n > 0
	}
	return false
}

func (c *crawler) get(ctx context.Context, target string) *http.Response {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.opt.Host+target, nil)
	req.Host = c.opt.Host
	rec := httptest.NewRecorder()
	c.opt.Handler.ServeHTTP(rec, req)
	return rec.Result()
}

func (c *crawler) visit(ctx context.Context, target string) error {
	resp := c.get(ctx, target)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("export: read %s: %w", target, err)
	}
	u, _ := url.Parse(target)

	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return c.redirect(u, resp.Header.Get("Location"))
	case resp.StatusCode != http.StatusOK:
		c.skipped[target] = ReasonStatus + strconv.Itoa(resp.StatusCode)
		return nil
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isHTML := mediaType == "text/html"
	var links []string
	switch {
	case isHTML:
		links = htmlLinks(body)
		if hasForm(body) {
			c.forms[target] = true
		}
	case mediaType == "text/css":
		links = cssLinks(body)
	case strings.HasSuffix(mediaType, "xml"):
		links = xmlLinks(body)
	case mediaType == "text/plain" && u.Path == "/robots.txt":
		links = robotsLinks(body)
	}

	for _, l := range links {
		if t, ok := c.local(u, l); ok {
			c.enqueue(t)
		}
	}
	if isHTML {
		body = rewritePageLinks(body, links, u, c)
	}
	return c.write(u, isHTML, body)
}

// local turns a link found on the page at base into a site-relative address,
// or reports that it points elsewhere.
func (c *crawler) local(base *url.URL, link string) (string, bool) {
	link = strings.TrimSpace(link)
	if link == "" || strings.HasPrefix(link, "#") {
		return "", false
	}
	ref, err := url.Parse(link)
	if err != nil {
		return "", false
	}
	abs := (&url.URL{Scheme: "http", Host: c.opt.Host, Path: base.Path}).ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}
	if !strings.EqualFold(abs.Hostname(), c.opt.Host) {
		return "", false
	}
	out := abs.EscapedPath()
	if out == "" {
		out = "/"
	}
	if abs.RawQuery != "" {
		out += "?" + abs.RawQuery
	}
	return out, true
}

// redirect writes a page that sends the reader on, since a static host has no
// redirect table of this website's. A meta refresh needs no script.
func (c *crawler) redirect(u *url.URL, location string) error {
	target, ok := c.local(u, location)
	if !ok {
		c.skipped[u.String()] = ReasonExternal
		return nil
	}
	c.enqueue(target)
	href := staticHref(target)
	esc := html.EscapeString(href)
	page := "<!doctype html>\n<meta charset=\"utf-8\">\n<meta http-equiv=\"refresh\" content=\"0; url=" + esc + "\">\n" +
		"<link rel=\"canonical\" href=\"" + esc + "\">\n<a href=\"" + esc + "\">" + esc + "</a>\n"
	return c.write(u, true, []byte(page))
}

func (c *crawler) notFoundPage(ctx context.Context) error {
	resp := c.get(ctx, notFoundProbe)
	if resp.StatusCode != http.StatusNotFound {
		return nil
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/html" {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("export: read 404 page: %w", err)
	}
	return c.writeFile("404.html", body)
}

// filePath is where the answer for an address lies in the export.
//
// A page becomes a directory with an index.html, so the address the site
// links to — /ueber-uns — is the address a static host answers. A list's
// second page, /blog?seite=2, becomes /blog/seite/2/: a static host does not
// look at the query. Everything else keeps its own path.
func filePath(u *url.URL, isHTML bool) string {
	p := path.Clean("/" + u.Path)
	if n := u.Query().Get("seite"); n != "" && n != "1" {
		p = path.Join(p, "seite", n)
	}
	if isHTML && path.Ext(p) != ".html" && path.Ext(p) != ".htm" {
		p = path.Join(p, "index.html")
	}
	if p == "/" {
		p = "/index.html"
	}
	return strings.TrimPrefix(p, "/")
}

// staticHref is the address a page links to in the export: the same, except
// for a list's page number, which has become part of the path.
func staticHref(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	n := u.Query().Get("seite")
	if n == "" || len(u.Query()) != 1 {
		return target
	}
	p := u.EscapedPath()
	if n == "1" {
		return p
	}
	return strings.TrimSuffix(p, "/") + "/seite/" + n + "/"
}

func (c *crawler) write(u *url.URL, isHTML bool, body []byte) error {
	rel := filePath(u, isHTML)
	if c.written[rel] {
		return nil
	}
	return c.writeFile(rel, body)
}

func (c *crawler) writeFile(rel string, body []byte) error {
	full := filepath.Join(c.opt.Dir, filepath.FromSlash(rel))
	root := filepath.Clean(c.opt.Dir) + string(filepath.Separator)
	if !strings.HasPrefix(full, root) {
		return fmt.Errorf("export: %s would lie outside the target directory", rel)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	c.written[rel] = true
	return nil
}

// rewritePageLinks replaces every link with a page number by the path it has
// in the export. Only those: every other address is already the right one.
func rewritePageLinks(body []byte, links []string, base *url.URL, c *crawler) []byte {
	done := map[string]bool{}
	for _, l := range links {
		if done[l] || !strings.Contains(l, "seite=") {
			continue
		}
		done[l] = true
		t, ok := c.local(base, l)
		if !ok {
			continue
		}
		to := staticHref(t)
		if to == t {
			continue
		}
		// The attribute holds the link HTML-escaped; & is the one character in
		// such an address that changes.
		raw := strings.ReplaceAll(l, "&", "&amp;")
		body = bytes.ReplaceAll(body, []byte(`"`+raw+`"`), []byte(`"`+to+`"`))
		body = bytes.ReplaceAll(body, []byte(`"`+l+`"`), []byte(`"`+to+`"`))
	}
	return body
}

// linkAttrs are the attributes a page loads or links through.
var linkAttrs = map[string]bool{"href": true, "src": true, "poster": true, "data": true}

func htmlLinks(body []byte) []string {
	var out []string
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return out
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		for _, a := range tok.Attr {
			switch {
			case linkAttrs[a.Key]:
				out = append(out, a.Val)
			case a.Key == "srcset" || a.Key == "imagesrcset":
				for _, cand := range strings.Split(a.Val, ",") {
					if f := strings.Fields(cand); len(f) > 0 {
						out = append(out, f[0])
					}
				}
			case a.Key == "style":
				out = append(out, cssLinks([]byte(a.Val))...)
			}
		}
		if tok.Data == "style" {
			if z.Next() == html.TextToken {
				out = append(out, cssLinks(z.Text())...)
			}
		}
	}
}

func hasForm(body []byte) bool {
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return false
		}
		if tt == html.StartTagToken {
			if name, _ := z.TagName(); string(name) == "form" {
				return true
			}
		}
	}
}

var (
	cssURL    = regexp.MustCompile(`url\(\s*['"]?([^'")]+)['"]?\s*\)`)
	cssImport = regexp.MustCompile(`@import\s+['"]([^'"]+)['"]`)
	xmlLoc    = regexp.MustCompile(`<loc>\s*([^<\s]+)\s*</loc>`)
	xmlHref   = regexp.MustCompile(`href="([^"]+)"`)
	robotsMap = regexp.MustCompile(`(?mi)^\s*sitemap:\s*(\S+)`)
)

func cssLinks(body []byte) []string {
	var out []string
	for _, m := range cssURL.FindAllSubmatch(body, -1) {
		if !bytes.HasPrefix(m[1], []byte("data:")) {
			out = append(out, string(m[1]))
		}
	}
	for _, m := range cssImport.FindAllSubmatch(body, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

func xmlLinks(body []byte) []string {
	var out []string
	for _, m := range xmlLoc.FindAllSubmatch(body, -1) {
		out = append(out, html.UnescapeString(string(m[1])))
	}
	for _, m := range xmlHref.FindAllSubmatch(body, -1) {
		out = append(out, html.UnescapeString(string(m[1])))
	}
	return out
}

func robotsLinks(body []byte) []string {
	var out []string
	for _, m := range robotsMap.FindAllSubmatch(body, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

func (c *crawler) report() *Report {
	r := &Report{}
	for f := range c.written {
		r.Files = append(r.Files, f)
	}
	sort.Strings(r.Files)
	for u, why := range c.skipped {
		r.Skipped = append(r.Skipped, Skip{URL: u, Reason: why})
	}
	sort.Slice(r.Skipped, func(i, j int) bool { return r.Skipped[i].URL < r.Skipped[j].URL })
	for u := range c.forms {
		r.Forms = append(r.Forms, u)
	}
	sort.Strings(r.Forms)
	return r
}
