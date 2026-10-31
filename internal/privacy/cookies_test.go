package privacy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

func allFeatures() Features {
	return Features{Shop: true, PriceChoice: true, Payment: true, ProtectedPages: true, Forms: true}
}

func TestCookieRegisterReadsTheCodesConstants(t *testing.T) {
	got := Cookies(allFeatures())
	if len(got) != 3 {
		t.Fatalf("want 3 cookies, got %d: %+v", len(got), got)
	}
	// The order is part of the contract: the text prints rows in it.
	wantConst := []Cookie{
		{Name: shop.CartCookieName, Lifetime: shop.CartLifetime, Trigger: TriggerBasket},
		{Name: shop.AudienceCookieName, Lifetime: shop.AudienceLifetime, Trigger: TriggerPriceView},
		{Name: page.UnlockCookiePrefix + "N", Lifetime: page.UnlockLifetime, Trigger: TriggerUnlock},
	}
	// Pinned literals as well: a constant that moves must be noticed here, not
	// only followed.
	wantLiteral := []Cookie{
		{Name: "hc_warenkorb", Lifetime: 30 * 24 * time.Hour, Trigger: TriggerBasket},
		{Name: "hc_preise", Lifetime: 30 * 24 * time.Hour, Trigger: TriggerPriceView},
		{Name: "hc_frei_N", Lifetime: 12 * time.Hour, Trigger: TriggerUnlock},
	}
	for i := range got {
		if got[i] != wantConst[i] {
			t.Errorf("entry %d reads %+v, the constants say %+v", i, got[i], wantConst[i])
		}
		if got[i] != wantLiteral[i] {
			t.Errorf("entry %d is %+v, the pinned literal is %+v: if the cookie was meant to change, change the literal and review the privacy text together", i, got[i], wantLiteral[i])
		}
	}
}

func TestRegisterFollowsTheFeatures(t *testing.T) {
	names := func(f Features) string {
		var n []string
		for _, c := range Cookies(f) {
			n = append(n, c.Name)
		}
		return strings.Join(n, ",")
	}
	cases := []struct {
		name string
		f    Features
		want string
	}{
		{"none", Features{}, ""},
		{"shop", Features{Shop: true}, "hc_warenkorb"},
		{"shop and price choice", Features{Shop: true, PriceChoice: true}, "hc_warenkorb,hc_preise"},
		{"price choice without shop", Features{PriceChoice: true}, ""},
		{"protected pages", Features{ProtectedPages: true}, "hc_frei_N"},
		{"payment and forms add none", Features{Shop: true, Payment: true, Forms: true}, "hc_warenkorb"},
		{"payment without shop", Features{Payment: true, Forms: true}, ""},
	}
	for _, c := range cases {
		if got := names(c.f); got != c.want {
			t.Errorf("%s: cookies %q, want %q", c.name, got, c.want)
		}
	}
}

// writerFiles are the only non-test files that write a cookie. The session
// cookie of the admin is named through the scs configuration in
// internal/auth/session.go and not through SetCookie, and it is not a public
// cookie, so it is not in the register. internal/admin/oidc.go sets short-lived
// sign-in cookies for the admin only. internal/public/plugins.go only names
// Set-Cookie on the deny list of headers a plugin may not set.
var writerFiles = map[string]bool{
	"internal/shop/cart.go":      true,
	"internal/shop/pricing.go":   true,
	"internal/public/access.go":  true,
	"internal/admin/oidc.go":     true,
	"internal/public/plugins.go": true,
}

// cookieWriting reports whether Go source writes a cookie: a call of a function
// named SetCookie, a composite literal of http.Cookie, or a string literal
// equal to set-cookie in any case.
func cookieWriting(name string, src any) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			switch fn := x.Fun.(type) {
			case *ast.SelectorExpr:
				if fn.Sel.Name == "SetCookie" {
					found = true
				}
			case *ast.Ident:
				if fn.Name == "SetCookie" {
					found = true
				}
			}
		case *ast.CompositeLit:
			if sel, ok := x.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Cookie" {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "http" {
					found = true
				}
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil && strings.EqualFold(s, "set-cookie") {
					found = true
				}
			}
		}
		return true
	})
	return found, nil
}

func TestEveryCookieWriterIsAccountedFor(t *testing.T) {
	root := filepath.Join("..", "..")
	// .claude holds whole copies of the tree in worktrees.
	skip := map[string]bool{".git": true, ".planning": true, ".claude": true, "build": true, "node_modules": true, "vendor": true, "data": true}
	var parsed int
	writers := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed++
		ok, err := cookieWriting(path, src)
		if err != nil {
			return err
		}
		if ok {
			rel, _ := filepath.Rel(root, path)
			writers[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if parsed < 150 {
		t.Fatalf("only %d Go files parsed: the walk no longer sees the tree", parsed)
	}
	var unknown []string
	for w := range writers {
		if !writerFiles[w] {
			unknown = append(unknown, w)
		}
	}
	sort.Strings(unknown)
	for _, w := range unknown {
		t.Errorf("%s writes a cookie and is not a known writer: a new cookie must be added to the register in cookies.go, to the privacy text in text.go and to this list", w)
	}
	for w := range writerFiles {
		if !writers[w] {
			t.Errorf("%s is listed as a cookie writer but no longer contains one: update the list", w)
		}
	}
}

func TestCookieWriterDetectionSeesACall(t *testing.T) {
	sources := map[string]string{
		"call":    "package x\nimport \"net/http\"\nfunc f(w http.ResponseWriter){ http.SetCookie(w, nil) }\n",
		"literal": "package x\nimport \"net/http\"\nvar c = &http.Cookie{Name: \"a\"}\n",
		"header":  "package x\nvar h = \"Set-Cookie\"\n",
	}
	for name, src := range sources {
		ok, err := cookieWriting(name+".go", src)
		if err != nil || !ok {
			t.Errorf("%s: writer not detected (found=%v err=%v)", name, ok, err)
		}
	}
	without := "package x\nimport \"net/http\"\nfunc f(w http.ResponseWriter){ w.WriteHeader(200) }\n"
	ok, err := cookieWriting("without.go", without)
	if err != nil || ok {
		t.Errorf("source without a cookie writer reported as one (found=%v err=%v)", ok, err)
	}
}
