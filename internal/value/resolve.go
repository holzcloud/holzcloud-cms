package value

import (
	"html"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
)

// rawTextElements are the elements whose content the browser does not read as
// character data, so a marker inside one is not in text however much it looks
// like it. The list is internal/marker's; it is repeated rather than exported
// from there because that package is a leaf that answers one question.
var rawTextElements = map[string]bool{
	"script": true, "style": true, "textarea": true, "title": true,
	"xmp": true, "iframe": true, "noembed": true, "noframes": true,
	"noscript": true, "plaintext": true,
}

// Resolve replaces every value marker in a stored document with its text,
// written for the place the marker stands.
//
// texts is ONE website's values, key to text — the request's website and no
// other (D-03). A nil or empty map makes every marker empty.
//
// # Why a tokenizer and not a regular expression over the body
//
// block.ResolveWords replaces over the whole body because its words are
// constants and html.EscapeString is right for them in text and in an
// attribute alike. A value is written by an editor and may stand inside a link
// address, where escaping makes markup inert but does not make an address safe:
// a regular expression cannot tell an href from a sentence, and the two need
// different treatment. So the document is tokenized the way marker.OnlyInText
// does it: every token without a marker is written back as its raw bytes, and
// the output differs from the input only where a marker stood.
//
// # Where a marker is replaced and where it is removed
//
// In text — a paragraph, a heading, a list item, code — the marker becomes
// html.EscapeString of its text. In a tag, resolveTag decides per attribute: an
// address gets the text percent-encoded under the scheme-and-host invariant of
// resolveURL, a text attribute such as alt gets the text, every other attribute
// loses the marker. Anywhere the browser is not reading character
// data — a comment, a doctype, the content of <script>, <style> or <title> — it
// is removed and never expanded.
//
// # Why an unknown key is nothing
//
// A printed [[wert:…]] would put this program's internal syntax in front of a
// visitor, which is the rule internal/album/expand.go states for a gallery
// whose album is gone. The key is reported to the editor elsewhere (VAL-03); the
// visitor reads the sentence without it.
//
// A document without the prefix is returned as it came, with no tokenizer and
// no allocation: that is what a page without a value costs.
func Resolve(doc string, texts map[string]string) string {
	if !Has(doc) {
		return doc
	}
	z := xhtml.NewTokenizer(strings.NewReader(doc))
	var out strings.Builder
	out.Grow(len(doc))
	inRawText := false
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			// A strings.Reader ends with io.EOF and the tokenizer refuses no
			// input, so this is the end of the document (marker.OnlyInText).
			break
		}
		raw := string(z.Raw())
		marked := Has(raw)
		switch tt {
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			if marked {
				// Token, not TagName: TagName consumes the name, and the
				// attributes are needed whole.
				tok := z.Token()
				if tt == xhtml.StartTagToken {
					inRawText = rawTextElements[tok.Data]
				}
				out.WriteString(resolveTag(tok, texts))
				continue
			}
			if tt == xhtml.StartTagToken {
				name, _ := z.TagName()
				inRawText = rawTextElements[string(name)]
			}
		case xhtml.EndTagToken:
			inRawText = false
		}
		if !marked {
			out.WriteString(raw)
			continue
		}
		if tt == xhtml.TextToken && !inRawText {
			out.WriteString(pattern.ReplaceAllStringFunc(raw, func(m string) string {
				return html.EscapeString(texts[keyOf(m)])
			}))
			continue
		}
		out.WriteString(pattern.ReplaceAllString(raw, ""))
	}
	return out.String()
}

// urlAttributes hold an address. A value in one is percent-encoded and kept
// only if it leaves the address's scheme and host as they were.
var urlAttributes = map[string]bool{
	"href": true, "src": true, "srcset": true, "action": true,
	"formaction": true, "cite": true, "poster": true,
}

// textAttributes are read by a person — a screen reader, a tooltip, a picture
// that did not load. A value in one is its plain text; Token.String escapes it.
var textAttributes = map[string]bool{
	"alt": true, "title": true, "aria-label": true, "aria-description": true,
	"placeholder": true, "label": true,
}

// resolveTag writes a start tag whose raw bytes name a value, with every marker
// in its attributes resolved for the attribute it stands in.
//
// The tag is re-serialised with Token.String, which escapes every attribute
// value, so nothing a value holds can end the attribute or the tag. Every other
// attribute — class, id, style, data-… — loses its markers: none of them is a
// place a reader sees a value, and a value in a style is a value in CSS.
func resolveTag(tok xhtml.Token, texts map[string]string) string {
	for i, a := range tok.Attr {
		if !Has(a.Val) {
			continue
		}
		switch {
		case a.Namespace == "" && urlAttributes[a.Key]:
			tok.Attr[i].Val = resolveURL(a.Val, texts)
		case a.Namespace == "" && textAttributes[a.Key]:
			tok.Attr[i].Val = pattern.ReplaceAllStringFunc(a.Val, func(m string) string {
				return texts[keyOf(m)]
			})
		default:
			tok.Attr[i].Val = pattern.ReplaceAllString(a.Val, "")
		}
	}
	// A marker in the tag outside any attribute value — between attributes,
	// in the name — never survives the tokenizer as an attribute value, so
	// there is nothing left to resolve.
	return tok.String()
}

// resolveURL puts the texts into an address and keeps the result only if the
// address still has the scheme and host it has with a neutral placeholder.
//
// Why an invariant and not just escaping: html.EscapeString makes markup inert
// but does not make an address safe. bluemonday and block.safeURL checked the
// scheme at save, and neither of them ever saw the value — it did not exist
// yet, or it has changed since. So the property they guaranteed is stated
// directly here: the editor wrote the scheme and the host, and a value cannot
// move either. https://x.ch/v plus "javascript:alert(1)" is a path on x.ch;
// "/" plus "/evil.example" would be the protocol-relative //evil.example and is
// refused; a value standing where the host is cannot become the host.
//
// It costs one url.Parse per marked attribute, where sanitising per request
// would cost a parse and a render of the whole body (33-RESEARCH.md,
// alternatives). When the invariant fails the markers are replaced with nothing:
// the link stays, the value is dropped.
//
// A value with a backslash is dropped. A browser treats a backslash as a slash
// in an http(s) address, so /\host is //host to it; pctEncode writes it as %5C,
// which no browser decodes back into a separator, but a value carrying one is
// dropped rather than argued about. The check is on the value's own text and
// not on the decoded address: decoding the whole address also decoded a %5C
// the editor wrote, and such a link always lost its value (IN-01 of the phase
// 33 review).
func resolveURL(val string, texts map[string]string) string {
	neutral := pattern.ReplaceAllString(val, "x")
	refused := false
	out := pattern.ReplaceAllStringFunc(val, func(m string) string {
		text, ok := texts[keyOf(m)]
		if !ok {
			return ""
		}
		if strings.Contains(text, `\`) {
			refused = true
		}
		return pctEncode(text)
	})
	if refused || !sameOrigin(neutral, out) {
		return pattern.ReplaceAllString(val, "")
	}
	return out
}

// sameOrigin reports whether out keeps the scheme and host of want, and is not
// an address a browser would read as protocol-relative.
func sameOrigin(want, out string) bool {
	w, err := url.Parse(want)
	if err != nil {
		return false
	}
	g, err := url.Parse(out)
	if err != nil {
		return false
	}
	if !strings.EqualFold(g.Scheme, w.Scheme) || g.Host != w.Host {
		return false
	}
	return !strings.HasPrefix(out, "//") && !strings.Contains(out, `\`)
}

// pctEncode writes a text for an address: every byte that is not an RFC 3986
// unreserved character and not one of ! $ ' ( ) * , ; : @ / becomes %XX in
// upper-case hex. Space, quote, angle brackets, backslash, percent, question
// mark, hash, square brackets, control and non-ASCII bytes are all encoded, so
// a value can neither end the attribute nor start a query or a fragment.
//
// & = and + are encoded too, wherever the marker stands. In a query string
// they would let a value add a parameter — ?q=[[wert:a]] with "1&admin=1" was
// ?q=1&admin=1 — or turn into a space; encoded, the value stays the one
// parameter's value (IN-01 of the phase 33 review). In a path the encoded
// spelling means the same as the bare one, so nothing is lost there.
func pctEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if keepInAddress(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

func keepInAddress(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("-._~!$'()*,;:@/", c) >= 0
}

// keyOf takes the key out of a complete marker.
func keyOf(marker string) string { return marker[len(Prefix) : len(marker)-2] }
