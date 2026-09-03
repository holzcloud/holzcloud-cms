package admin

import "testing"

func TestRewritePreviewURLs(t *testing.T) {
	const base = "/admin/websites/7/preview"

	cases := map[string]struct {
		in   string
		want string
	}{
		"template asset": {
			`<link rel="stylesheet" href="/t/style.css">`,
			`<link rel="stylesheet" href="/admin/websites/7/preview/t/style.css">`,
		},
		"internal page link": {
			`<a href="/about">About</a>`,
			`<a href="/admin/websites/7/preview/about">About</a>`,
		},
		"home link": {
			`<a href="/">Home</a>`,
			`<a href="/admin/websites/7/preview/">Home</a>`,
		},
		"template image": {
			`<img src="/t/img/logo.png">`,
			`<img src="/admin/websites/7/preview/t/img/logo.png">`,
		},
		// /media/ is a public route in the same origin — rewriting it would
		// break every image and download link in the preview.
		"media src is preserved": {
			`<img src="/media/7/abc.png">`,
			`<img src="/media/7/abc.png">`,
		},
		"media href is preserved": {
			`<a href="/media/7/handbook.pdf">PDF</a>`,
			`<a href="/media/7/handbook.pdf">PDF</a>`,
		},
		"admin link is preserved": {
			`<a href="/admin/websites/7/pages">Back</a>`,
			`<a href="/admin/websites/7/pages">Back</a>`,
		},
		"protocol-relative url is preserved": {
			`<a href="//cdn.example.com/x.css">CDN</a>`,
			`<a href="//cdn.example.com/x.css">CDN</a>`,
		},
		"absolute url is untouched": {
			`<a href="https://example.com/">External</a>`,
			`<a href="https://example.com/">External</a>`,
		},
		"relative url is untouched": {
			`<a href="about.html">About</a>`,
			`<a href="about.html">About</a>`,
		},
		"multiple occurrences": {
			`<a href="/a">A</a><a href="/b">B</a><img src="/t/c.png">`,
			`<a href="/admin/websites/7/preview/a">A</a>` +
				`<a href="/admin/websites/7/preview/b">B</a>` +
				`<img src="/admin/websites/7/preview/t/c.png">`,
		},
		"no matches": {
			`<p>nothing to rewrite</p>`,
			`<p>nothing to rewrite</p>`,
		},
	}

	for label, tc := range cases {
		got := string(rewritePreviewURLs([]byte(tc.in), base))
		if got != tc.want {
			t.Errorf("%s:\n got: %s\nwant: %s", label, got, tc.want)
		}
	}
}
