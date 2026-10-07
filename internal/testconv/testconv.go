/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package testconv checks that the tests of the module follow the test conventions of AGENTS.md.
//
// It parses the Go files of every package with go/ast and reports, for the _test.go files:
//   - a test whose name is not Test<Subject>[_<Method>][_<scenario>], where the subject is an
//     identifier declared in the package (first letter capitalised) and the scenario is lowerCamel;
//   - a test without a doc comment;
//   - a gotests "args" struct, a commented-out test, and a package-level var.
//
// It reads declarations only, without type checking: a method promoted from an embedded struct
// (GetProfile from common.BaseResults on search Results) is not a method of the outer type here.
// Test such a method on the type that declares it, or in TestResults_accessors.
package testconv

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// part is one "_"-separated part of a test name: letters and digits only.
var part = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// commentedTest matches a comment line that holds a test function.
var commentedTest = regexp.MustCompile(`^\s*func Test`)

// pkg holds the identifiers a package declares outside its test files.
type pkg struct {
	// names are the package-level functions, types, variables and constants.
	names map[string]bool

	// methods maps a type name to the names of its methods.
	methods map[string]map[string]bool
}

// Check checks every package under root, skipping hidden and testdata directories.
//
// It returns one "path:line: message" per violation, sorted, with paths relative to root.
func Check(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		found, err := checkDir(root, path)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("checking tests under %s: %w", root, err)
	}
	sort.Strings(violations)
	return violations, nil
}

// checkDir checks the test files of the package in dir.
func checkDir(root, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	fset := token.NewFileSet()
	p := pkg{names: map[string]bool{}, methods: map[string]map[string]bool{}}
	var tests []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", e.Name(), err)
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			tests = append(tests, f)
			continue
		}
		p.declare(f)
	}

	var violations []string
	for _, f := range tests {
		for _, v := range p.checkFile(f) {
			pos := fset.Position(v.pos)
			rel, err := filepath.Rel(root, pos.Filename)
			if err != nil {
				rel = pos.Filename
			}
			violations = append(violations, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), pos.Line, v.msg))
		}
	}
	return violations, nil
}

// declare records the package-level identifiers and methods of f.
func (p *pkg) declare(f *ast.File) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				p.names[d.Name.Name] = true
				continue
			}
			recv := receiverType(d.Recv.List[0].Type)
			if p.methods[recv] == nil {
				p.methods[recv] = map[string]bool{}
			}
			p.methods[recv][d.Name.Name] = true
		case *ast.GenDecl:
			p.declareSpecs(d.Specs)
		}
	}
}

// declareSpecs records the names of type, var and const specs.
func (p *pkg) declareSpecs(specs []ast.Spec) {
	for _, spec := range specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			p.names[s.Name.Name] = true
		case *ast.ValueSpec:
			for _, n := range s.Names {
				p.names[n.Name] = true
			}
		}
	}
}

// receiverType returns the type name of a method receiver: T for T, *T and T[P].
func receiverType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverType(e.X)
	case *ast.IndexExpr:
		return receiverType(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// violation is one broken convention at a position.
type violation struct {
	pos token.Pos
	msg string
}

// checkFile checks one test file.
func (p *pkg) checkFile(f *ast.File) []violation {
	var found []violation
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				found = append(found, violation{d.Pos(),
					"package-level var in a test file: return the fixture from a function"})
			}
		case *ast.FuncDecl:
			if !isTest(d) {
				continue
			}
			if msg := p.checkName(d.Name.Name); msg != "" {
				found = append(found, violation{d.Pos(), msg})
			}
			if d.Doc == nil {
				found = append(found, violation{d.Pos(), d.Name.Name + " has no doc comment"})
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "args" {
			found = append(found, violation{ts.Pos(), `"args" struct: use flat fields in the test table`})
		}
		return true
	})
	for _, group := range f.Comments {
		for _, c := range group.List {
			if commentedTest.MatchString(strings.TrimPrefix(c.Text, "//")) {
				found = append(found, violation{c.Pos(), "commented-out test: delete it"})
			}
		}
	}
	return found
}

// isTest reports whether fn is a test function: TestXxx(t *testing.T), not TestMain(m *testing.M).
func isTest(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}
	params := fn.Type.Params.List
	if len(params) != 1 {
		return false
	}
	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "T"
}

// checkName returns why name breaks the naming convention, or "" when it follows it.
func (p *pkg) checkName(name string) string {
	rest := strings.TrimPrefix(name, "Test")
	if strings.HasPrefix(rest, "_") {
		return name + ": no Test_ prefix, use Test<Subject>"
	}
	parts := strings.Split(rest, "_")
	if len(parts) > 3 {
		return name + ": at most Test<Subject>_<Method>_<scenario>"
	}
	for _, pt := range parts {
		if !part.MatchString(pt) {
			return name + ": each part of the name is letters and digits"
		}
	}
	subject, ok := p.subject(parts[0])
	if !ok {
		return fmt.Sprintf("%s: %s names no identifier of the package", name, parts[0])
	}
	if name == "TestMain" {
		return name + ": TestMain is reserved for func TestMain(m *testing.M), add a _<scenario>"
	}
	return p.checkSuffix(name, subject, parts[1:])
}

// subject returns the declared identifier that the capitalised name refers to.
func (p *pkg) subject(capitalised string) (string, bool) {
	for _, candidate := range []string{capitalised, lowerFirst(capitalised)} {
		if p.names[candidate] {
			return candidate, true
		}
	}
	return "", false
}

// checkSuffix checks what follows the subject: an optional method of it, then an optional scenario.
func (p *pkg) checkSuffix(name, subject string, rest []string) string {
	if len(rest) > 0 && p.methods[subject][rest[0]] {
		rest = rest[1:]
	}
	switch {
	case len(rest) == 0:
		return ""
	case len(rest) > 1:
		return fmt.Sprintf("%s: %s is not a method of %s", name, rest[0], subject)
	case !unicode.IsLower(rune(rest[0][0])):
		return fmt.Sprintf("%s: the scenario %s starts lower-case (or it is not a method of %s)",
			name, rest[0], subject)
	}
	return ""
}

// lowerFirst returns s with its first letter lower-cased.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
