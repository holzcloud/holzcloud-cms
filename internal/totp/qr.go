package totp

import (
	"fmt"
	"html/template"
	"strings"

	"rsc.io/qr"
)

// QRCode renders a value as an inline SVG QR code.
//
// SVG written into the page rather than a PNG in a data: URI, because the admin
// runs under a `default-src 'self'` policy and a data: URI would mean widening
// img-src for every page to serve one image on one screen. Inline markup needs
// no exception at all.
//
// The squares are drawn as one <path> rather than a few hundred <rect>
// elements: same picture, a fifth of the bytes, and no gaps between modules
// where a renderer rounds coordinates differently.
func QRCode(value string) (template.HTML, error) {
	// M corrects about 15% and is what authenticator apps are tested against;
	// a higher level makes the code denser for no gain on a screen.
	code, err := qr.Encode(value, qr.M)
	if err != nil {
		return "", fmt.Errorf("encode qr code: %w", err)
	}

	size := code.Size
	// A four-module quiet zone is part of the specification. Without it a
	// scanner reading the code off a screen with a busy background often fails.
	const quiet = 4
	total := size + 2*quiet

	var path strings.Builder
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}

	var svg strings.Builder
	fmt.Fprintf(&svg,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" `+
			`width="240" height="240" shape-rendering="crispEdges" `+
			`role="img" aria-label="QR-Code für die Authenticator-App">`,
		total, total)
	// The white background is drawn rather than assumed: a dark admin theme
	// behind a transparent code makes it unreadable.
	fmt.Fprintf(&svg, `<rect width="%d" height="%d" fill="#fff"/>`, total, total)
	fmt.Fprintf(&svg, `<path fill="#000" d="%s"/>`, path.String())
	svg.WriteString(`</svg>`)

	// The content is assembled here from a QR bitmap and integers, never from
	// user input, so marking it safe does not hand anyone a way in.
	return template.HTML(svg.String()), nil
}
