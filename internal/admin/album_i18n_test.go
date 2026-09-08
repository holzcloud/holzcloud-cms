package admin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// The gate on a sentence that is written in one place and shown in another.
//
// tools/i18n reads the ARGUMENTS of SetFlashError and SetFlashSuccess — the
// literal has to stand at the call site, or the collector sees a variable and
// collects nothing. internal/admin/menu.go passes its literals directly and
// every one of them is in the catalogue. This file does not: albumSaid and
// requireOwnPicture return their sentences from a helper, and six operator
// sentences were therefore structurally invisible. The gate would have read
// "0 offen, 0 verwaist" while all six still printed in German to a French
// operator, with nothing anywhere saying so — which is the C-i18n family
// exactly.
//
// i18n.N is the answer the codebase already had: it returns its argument
// unchanged and exists to be a name the collector can find (internal/i18n/
// i18n.go says so), and internal/block/render.go's three keys are collected
// because of it.
//
// So the rule this test holds is one sentence: no function in album.go returns
// a non-empty string literal bare. It is deliberately a rule about the SHAPE
// and not a list of the six sentences — a seventh added next year is the case a
// list cannot catch, and it is the case that would ship.
func TestNoAlbumSentenceIsReturnedUnmarked(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "album.go", nil, 0)
	if err != nil {
		t.Fatalf("parse album.go: %v", err)
	}

	marked := 0
	ast.Inspect(file, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, res := range ret.Results {
			switch v := res.(type) {
			case *ast.BasicLit:
				if v.Kind != token.STRING {
					continue
				}
				// "" is not a sentence: it is how albumSaid says "this is not
				// one of the store's named errors" and how requireOwnPicture
				// says "nothing to refuse".
				if s, err := strconv.Unquote(v.Value); err == nil && s != "" {
					t.Errorf("%s: the sentence %q is returned as a bare literal. "+
						"tools/i18n reads the arguments of SetFlash*, so a sentence handed "+
						"to one through a variable is collected nowhere and reported as "+
						"neither offen nor verwaist. Wrap it in i18n.N where it is written, "+
						"the way internal/block/render.go does.",
						fset.Position(v.Pos()), s)
				}
			case *ast.CallExpr:
				if name(v.Fun) == "N" && len(v.Args) == 1 {
					marked++
				}
			}
		}
		return true
	})

	// The positive control. A file that had stopped returning sentences
	// altogether — refactored, renamed, moved — would satisfy the rule above
	// while proving nothing, and this is the phase in which that mattered.
	if marked < 6 {
		t.Errorf("only %d marked sentences are returned from album.go; the six the "+
			"album screens refuse with are what this gate exists for", marked)
	}
}

func name(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.Ident:
		return f.Name
	}
	return ""
}
