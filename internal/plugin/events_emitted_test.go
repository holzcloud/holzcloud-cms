package plugin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notEmitted names an event the host declares and deliberately does not send,
// with the reason. Anything else declared must have a sender.
//
// One entry, and it is not laziness. form.received happens inside the contact
// form PLUGIN: the host never sees the submission, because the route belongs to
// the module. Only the host may emit, so there is nothing here that could send
// it. Removing the name would break a published SDK constant; sending it would
// need a way for a plugin to ask the host to emit, which is a feature and not a
// fix.
var notEmitted = map[string]string{
	EventFormReceived: "the submission is handled inside the contact form plugin; " +
		"the host never sees it, and only the host may emit",
}

// Every event this program promises has somebody who sends it.
//
// This is the test the defect it was written for would have failed. The host
// declared five event names and emitted one. The other four stood in abi.go
// and, mirrored, in sdk/plugin.go — where a plugin author reads them and
// registers a handler that is then never called, with no error anywhere and
// nothing in the log. It looks exactly like "no page was saved", which is why
// it survived three milestones: nobody can report a silence.
//
// It reads the source rather than running anything, and that is the point. A
// behavioural test would prove that the path it exercises emits; this proves
// that no declared name is without a sender, which is the property that was
// false.
func TestEveryDeclaredEventIsEmittedSomewhere(t *testing.T) {
	declared := declaredEvents(t)
	if len(declared) < 4 {
		t.Fatalf("found %d event constants; abi.go declares more than that — the reader is broken, not the code", len(declared))
	}
	sent := emittedEvents(t)

	for name, konst := range declared {
		if _, ok := notEmitted[name]; ok {
			continue
		}
		if !sent[konst] && !sent[name] {
			t.Errorf("%s (%q) is declared and mirrored in the SDK, and nothing in this program emits it. "+
				"A plugin registering for it is never called and never told why. Either emit it, or "+
				"put it in notEmitted with the reason.", konst, name)
		}
	}

	// The exemption list has to stay honest in the other direction too: an
	// entry for an event that IS emitted would quietly excuse the next one.
	for name := range notEmitted {
		konst := declared[name]
		if sent[konst] || sent[name] {
			t.Errorf("%q is in notEmitted and is emitted after all; remove the exemption", name)
		}
	}
}

// declaredEvents reads the Event* constants out of this package, mapping the
// value ("page.saved") to the name (EventPageSaved).
func declaredEvents(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "abi.go", nil, 0)
	if err != nil {
		t.Fatalf("parse abi.go: %v", err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, id := range vs.Names {
			if !strings.HasPrefix(id.Name, "Event") || i >= len(vs.Values) {
				continue
			}
			lit, ok := vs.Values[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			out[strings.Trim(lit.Value, `"`)] = id.Name
			_ = id
		}
		return true
	})
	return out
}

// emittedEvents collects the first argument of every REACHED Emit call in the
// tree, as it is written: "plugin.EventPageSaved" reduces to "EventPageSaved".
//
// Reached, and that word is the whole test. The first version only looked for
// an Emit call with the constant in it, and stayed green when the three call
// sites of the emitting helpers were deleted — because the helpers themselves
// still contained the Emit. A helper nobody calls emits nothing, and a test
// that cannot tell the difference would have passed on the original defect just
// as happily.
//
// So an Emit inside a function counts only if that function is itself called
// from somewhere else. One level, not a full reachability analysis: the shape
// this guards against is an emitter that is written and never wired, and one
// level is exactly the distance that mistake travels.
func emittedEvents(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	called := calledFunctions(t)
	root := filepath.Join("..", "..")
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "data", "node_modules", ".planning", "testdata", "plugins":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			// Manager.Emit itself is the mechanism, not a sender.
			if fn.Name.Name == "Emit" {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Emit" {
					return true
				}
				if !called[fn.Name.Name] {
					return true // written, never wired
				}
				switch a := call.Args[0].(type) {
				case *ast.SelectorExpr:
					out[a.Sel.Name] = true
				case *ast.Ident:
					out[a.Name] = true
				case *ast.BasicLit:
					out[strings.Trim(a.Value, `"`)] = true
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// calledFunctions is every function name this tree calls, by bare name.
//
// By name and not by identity, because that is enough here and a great deal
// cheaper: two different methods sharing a name would both count as called, and
// the failure that guards against — an emitter written and never wired — does
// not involve a second function of the same name.
func calledFunctions(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	root := filepath.Join("..", "..")
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "data", "node_modules", ".planning", "testdata", "plugins":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		for _, d := range f.Decls {
			fn, isFunc := d.(*ast.FuncDecl)
			ast.Inspect(d, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch c := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = c.Sel.Name
				case *ast.Ident:
					name = c.Name
				}
				// A function calling itself is not somebody else calling it.
				if name != "" && !(isFunc && fn.Name.Name == name) {
					out[name] = true
				}
				return true
			})
		}
		return nil
	})
	return out
}
