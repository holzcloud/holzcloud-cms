package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestSecretMatchesComparesInConstantTime reads secretMatches instead of timing
// it. Replacing subtle.ConstantTimeCompare with == behaves identically in every
// test that exists; the Phase 10 audit did exactly that and the whole suite
// stayed green (code review IN-01).
func TestSecretMatchesComparesInConstantTime(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "forwardauth.go", nil, 0)
	if err != nil {
		t.Fatalf("parse forwardauth.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == "secretMatches" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("no func secretMatches in forwardauth.go")
	}

	constant := false
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "ConstantTimeCompare" {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "subtle" {
				constant = true
			}
		}
		// An equality between two things that are not literals is a byte-by-byte
		// comparison that stops at the first difference.
		if bin, ok := n.(*ast.BinaryExpr); ok && (bin.Op == token.EQL || bin.Op == token.NEQ) {
			_, xLit := bin.X.(*ast.BasicLit)
			_, yLit := bin.Y.(*ast.BasicLit)
			if !xLit && !yLit {
				t.Errorf("secretMatches compares with %s at %s; the secret must be compared in constant time",
					bin.Op, fset.Position(bin.Pos()))
			}
		}
		return true
	})
	if !constant {
		t.Error("secretMatches does not call subtle.ConstantTimeCompare")
	}
}
