package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// These tests read main.go instead of running it. Each holds a property that
// behaves identically when it is broken — until the day it matters — which is
// the one kind of property an ordinary test cannot see. The Phase 10 audit
// measured both: removing the start-up check left the whole suite green, and so
// did moving the forward-auth middleware inside RequestID and AccessLog.

func parseMainFile(t *testing.T) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	return fset, file
}

func topLevelFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("no func %s in main.go", name)
	return nil
}

func callsIdent(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return !found
	})
	return found
}

// TestMainRunsTheSingleSignOnStartUpCheck: the check that refuses to start
// with a provisioning default website, or a website group, that names no
// website is only worth anything if main calls it.
func TestMainRunsTheSingleSignOnStartUpCheck(t *testing.T) {
	_, file := parseMainFile(t)
	if !callsIdent(topLevelFunc(t, file, "main"), "checkSSOWebsites") {
		t.Error("main does not call checkSSOWebsites; the service would start with provisioning or " +
			"website groups pointing at websites that do not exist")
	}
}

// TestForwardAuthIsTheOutermostLayer: the strip of every inbound identity
// header must run before anything else reads the request — the request ID and
// the access log included — so the last wrapper newRouter puts around the
// handler is web.ForwardAuth.
func TestForwardAuthIsTheOutermostLayer(t *testing.T) {
	fset, file := parseMainFile(t)
	fn := topLevelFunc(t, file, "newRouter")

	var wraps []*ast.AssignStmt
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 {
			return true
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok && id.Name == "handler" {
			wraps = append(wraps, as)
		}
		return true
	})
	if len(wraps) == 0 {
		t.Fatal("newRouter assigns nothing to handler")
	}
	wrapperOf := func(as *ast.AssignStmt) string {
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return ""
		}
		inner, ok := call.Fun.(*ast.CallExpr)
		if !ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				return sel.Sel.Name
			}
			return ""
		}
		if sel, ok := inner.Fun.(*ast.SelectorExpr); ok {
			return sel.Sel.Name
		}
		return ""
	}

	last := wraps[len(wraps)-1]
	if got := wrapperOf(last); got != "ForwardAuth" {
		t.Errorf("the outermost wrapper newRouter applies is %q at %s; want web.ForwardAuth, so no layer "+
			"reads an identity header before the strip", got, fset.Position(last.Pos()))
	}
	for _, inner := range []string{"RequestID", "AccessLog"} {
		seen := false
		for _, as := range wraps[:len(wraps)-1] {
			if wrapperOf(as) == inner {
				seen = true
			}
		}
		if !seen {
			t.Errorf("%s is not applied inside ForwardAuth", inner)
		}
	}
}
