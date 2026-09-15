// Command surface reports exported names that nothing outside their package uses.
//
//	go run ./tools/surface            # the candidates, by package
//	go run ./tools/surface -all       # every exported name and its use count
//	go run ./tools/surface -count     # just the two numbers
//
// # Why this exists
//
// This project's own rule is that a name is exported when somebody outside
// needs it, and it states the rule at the one place it went out of its way to
// follow it: hasAlbumMarker is unexported "and that is a decision rather than
// tidiness", with the reasoning written down. Measured on 2026-09-15, 301 of
// 1 312 exported names in internal/ were used nowhere outside their own
// package. That is the size of the drift from the rule.
//
// # Why it parses instead of grepping
//
// The first measurement was a regular expression over words, and it was wrong
// in both directions. It counted a name as used when the same word appeared in
// an unrelated sentence, and it missed a package imported under an alias. This
// one reads the syntax: a use is a selector expression whose base resolves to
// an import of the defining package, which is what a use actually is.
//
// # What it still cannot see, and says so
//
// A field read by an html/template through reflection. This matters here — the
// admin passes a struct to a template on every screen — so TYPES are reported
// apart from functions and values, and the report says why: a type reached only
// by a template can be unexported without anything breaking, because the
// template reads its FIELDS, and the fields stay exported either way.
//
// It also does not look at methods. A method on an exported type is reachable
// wherever the type is, and deciding that surface is a different question from
// deciding this one.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// decl is one exported top-level declaration.
type decl struct {
	name string
	pkg  string // the import path, relative to the module root
	kind string // "type", "func", "var" or "const"
	file string
	line int
	used int // uses from outside pkg
	home int // uses inside its own package, its own declaration not counted
}

func main() {
	all := flag.Bool("all", false, "every exported name, used or not")
	count := flag.Bool("count", false, "print only the totals")
	dead := flag.Bool("dead", false, "only names nothing uses at all, inside or out")
	flag.Parse()

	fset := token.NewFileSet()
	decls := map[string]*decl{} // key: pkg + "." + name
	files := map[string][]*ast.File{}

	err := filepath.Walk("internal", func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, perr := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if perr != nil {
			return perr
		}
		dir := filepath.Dir(p)
		files[dir] = append(files[dir], f)
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		for _, d := range f.Decls {
			for _, e := range exportedOf(d) {
				decls[dir+"."+e.name] = &decl{name: e.name, pkg: dir, kind: e.kind, file: p, line: fset.Position(e.pos).Line}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// Every file in the module, because a use from a test in another package
	// is a use, and so is one from cmd/ or tools/.
	countUses(fset, decls)

	byPkg := map[string][]*decl{}
	var unused, nowhere int
	for _, d := range decls {
		if d.used == 0 {
			unused++
		}
		if d.used == 0 && d.home == 0 {
			nowhere++
		}
		show := *all || (*dead && d.used == 0 && d.home == 0) || (!*dead && d.used == 0)
		if show {
			byPkg[d.pkg] = append(byPkg[d.pkg], d)
		}
	}

	if !*count {
		var pkgs []string
		for p := range byPkg {
			pkgs = append(pkgs, p)
		}
		sort.Strings(pkgs)
		for _, p := range pkgs {
			list := byPkg[p]
			sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
			fmt.Printf("\n%s\n", p)
			for _, d := range list {
				fmt.Printf("  %-6s %-32s %s:%d", d.kind, d.name, d.file, d.line)
				if *all {
					fmt.Printf("  outside %d×, inside %d×", d.used, d.home)
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}

	fmt.Printf("%d exported names in internal/, %d used nowhere outside their package, %d used nowhere at all\n",
		len(decls), unused, nowhere)
}

type exported struct {
	name string
	kind string
	pos  token.Pos
}

func exportedOf(d ast.Decl) []exported {
	var out []exported
	switch d := d.(type) {
	case *ast.FuncDecl:
		// A method belongs to its receiver's surface, not to the package's.
		if d.Recv == nil && d.Name.IsExported() {
			out = append(out, exported{d.Name.Name, "func", d.Name.Pos()})
		}
	case *ast.GenDecl:
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				if s.Name.IsExported() {
					out = append(out, exported{s.Name.Name, "type", s.Name.Pos()})
				}
			case *ast.ValueSpec:
				kind := "var"
				if d.Tok == token.CONST {
					kind = "const"
				}
				for _, n := range s.Names {
					if n.IsExported() {
						out = append(out, exported{n.Name, kind, n.Pos()})
					}
				}
			}
		}
	}
	return out
}

// countUses walks every Go file in the module and counts selector expressions
// that name one of the declarations, from outside its own package.
func countUses(fset *token.FileSet, decls map[string]*decl) {
	_ = filepath.Walk(".", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "data", "node_modules", ".planning", "testdata":
				return filepath.SkipDir
			}
			// The plugins are their own modules and cannot import internal/.
			if p == "plugins" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		here := filepath.Dir(p)

		// What each import is called in this file, by local name.
		imports := map[string]string{}
		for _, im := range f.Imports {
			path, e := strconv.Unquote(im.Path.Value)
			if e != nil || !strings.Contains(path, "holzcloud-cms/internal") {
				continue
			}
			dir := path[strings.Index(path, "internal"):]
			name := filepath.Base(dir)
			if im.Name != nil {
				name = im.Name.Name
			}
			imports[name] = dir
		}
		if len(imports) == 0 {
			return nil
		}

		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			base, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			dir, ok := imports[base.Name]
			if !ok || dir == here {
				return true
			}
			if d := decls[dir+"."+sel.Sel.Name]; d != nil {
				d.used++
			}
			return true
		})
		return nil
	})

	// Inside the package a name is used as a bare identifier, so the count is a
	// different question and asked separately: a name nothing uses ANYWHERE is
	// dead code, and a name only its own package uses is merely over-exported.
	// Those two call for opposite fixes, which is why one number cannot answer
	// both.
	for _, d := range decls {
		d.home = homeUses(fset, d)
	}
}

// homeUses counts a name's mentions in its own package, not counting the line
// that declares it.
func homeUses(fset *token.FileSet, d *decl) int {
	entries, err := os.ReadDir(d.pkg)
	if err != nil {
		return 0
	}
	var n int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		p := filepath.Join(d.pkg, e.Name())
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			continue
		}
		ast.Inspect(f, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if !ok || id.Name != d.name {
				return true
			}
			if p == d.file && fset.Position(id.Pos()).Line == d.line {
				return true // the declaration itself
			}
			n++
			return true
		})
	}
	return n
}
