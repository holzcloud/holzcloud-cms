package admin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// These tests read forwardauth.go instead of running it. Each holds a property
// the Phase 10 audit found correct and held by nothing: breaking it left the
// suite green, or went red only because the breakage happened to land in a
// handler that is tested for something else.

func parseAdminSource(t *testing.T, name string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return fset, file
}

func declNamed(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// TestProvisioningPassesOnlyARandomSecretToCreate (threat T-10-24). The audit
// replaced randomSecret() with a constant and the whole suite stayed green: a
// password every provisioned account shares is a password somebody learns, and
// then every such account opens with it on the ordinary login form.
func TestProvisioningPassesOnlyARandomSecretToCreate(t *testing.T) {
	_, file := parseAdminSource(t, "forwardauth.go")
	fn := declNamed(file, "provisionSSOUser")
	if fn == nil {
		t.Fatal("no provisionSSOUser in forwardauth.go")
	}
	var create *ast.CallExpr
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Create" {
				create = call
			}
		}
		return true
	})
	if create == nil || len(create.Args) < 4 {
		t.Fatal("provisionSSOUser does not hand a password to Create")
	}
	password, ok := create.Args[3].(*ast.Ident)
	if !ok {
		t.Fatalf("the password handed to Create is a %T, not a variable assigned from randomSecret()", create.Args[3])
	}
	assignments := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name != password.Name {
				continue
			}
			assignments++
			rhs := as.Rhs[0]
			if len(as.Rhs) == len(as.Lhs) {
				rhs = as.Rhs[i]
			}
			call, isCall := rhs.(*ast.CallExpr)
			var fun *ast.Ident
			if isCall {
				fun, _ = call.Fun.(*ast.Ident)
			}
			if fun == nil || fun.Name != "randomSecret" {
				t.Errorf("%s, the password of every provisioned account, is assigned from something other than randomSecret()", password.Name)
			}
		}
		return true
	})
	if assignments == 0 {
		t.Errorf("%s is never assigned in provisionSSOUser", password.Name)
	}
}

// TestTheSSOMarksHaveExactlyOneWriter (threat T-10-36). via_sso exempts a
// session from the second factor, and sso_username decides which identity a
// running session belongs to. Each is set in exactly one place, the
// forward-auth sign-in, after completeLogin has removed both. A second writer
// used to fall out only when it landed in a handler that happened to be tested.
func TestTheSSOMarksHaveExactlyOneWriter(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	writers := map[string][]string{"SessionKeyViaSSO": nil, "SessionKeySSOUsername": nil}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Put" {
					return true
				}
				if key, ok := call.Args[1].(*ast.SelectorExpr); ok {
					if _, tracked := writers[key.Sel.Name]; tracked {
						writers[key.Sel.Name] = append(writers[key.Sel.Name],
							fmt.Sprintf("%s in %s", fset.Position(call.Pos()), fn.Name.Name))
					}
				}
				return true
			})
		}
	}
	for key, at := range writers {
		if len(at) != 1 || !strings.HasSuffix(at[0], " in ForwardAuthSignIn") {
			t.Errorf("auth.%s is written at %d place(s); want exactly one, in ForwardAuthSignIn\n  %s",
				key, len(at), strings.Join(at, "\n  "))
		}
	}
}

// TestForwardAuthSignInNeverAnswersTheRequest (threat T-10-19). A refused
// sign-in falls through to the next handler, which shows the ordinary login
// form: an untrusted or unknown identity must never meet a 403, or the way back
// in dies with the proxy. The plan checked this once with grep.
func TestForwardAuthSignInNeverAnswersTheRequest(t *testing.T) {
	fset, file := parseAdminSource(t, "forwardauth.go")
	for _, name := range []string{"ForwardAuthSignIn", "refuseSSO", "endSSOSession", "refreshSSOSession",
		"syncRightsFromGroups", "syncWebsites", "provisionSSOUser"} {
		fn := declNamed(file, name)
		if fn == nil {
			t.Errorf("no %s in forwardauth.go", name)
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, isPkg := sel.X.(*ast.Ident)
			answers := sel.Sel.Name == "WriteHeader" || sel.Sel.Name == "redirect" ||
				(isPkg && pkg.Name == "http" && (sel.Sel.Name == "Error" || sel.Sel.Name == "Redirect" ||
					strings.HasPrefix(sel.Sel.Name, "Status")))
			if answers {
				t.Errorf("%s uses %s at %s; a refused sign-in must fall through, never answer",
					name, sel.Sel.Name, fset.Position(sel.Pos()))
			}
			return true
		})
	}
}

// TestForwardAuthNeverFeedsTheLoginThrottle (threat T-10-20). The throttle
// counts failures per address and per account; a proxy that refuses the same
// identity on every request would lock the ordinary password form for that
// account within minutes. The absence is deliberate and was held by nothing.
func TestForwardAuthNeverFeedsTheLoginThrottle(t *testing.T) {
	fset, file := parseAdminSource(t, "forwardauth.go")
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "loginThrottle" {
			t.Errorf("forwardauth.go reaches the login throttle at %s", fset.Position(sel.Pos()))
		}
		return true
	})
}
