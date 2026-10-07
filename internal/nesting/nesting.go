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

// Package nesting checks that no function of the module nests control statements more than
// MaxDepth levels deep (AGENTS.md, Standards and guardrails).
//
// if, for, range, switch, type switch and select each add a level; an else-if stays at the level
// of its if; a function literal is a function of its own and starts again at 0.
package nesting

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// MaxDepth is the deepest nesting of control statements allowed in a function.
const MaxDepth = 3

// Check checks every Go file under root, skipping hidden and testdata directories.
//
// It returns one "path:line: message" per function deeper than MaxDepth, sorted, with paths
// relative to root.
func Check(root string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		for _, v := range checkFile(fset, f) {
			violations = append(violations, filepath.ToSlash(rel)+":"+v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("checking nesting under %s: %w", root, err)
	}
	sort.Strings(violations)
	return violations, nil
}

// checkFile returns "line: message" for every function and function literal of f deeper than MaxDepth.
func checkFile(fset *token.FileSet, f *ast.File) []string {
	var found []string
	report := func(pos token.Pos, name string, body *ast.BlockStmt) {
		if d := depth(body); d > MaxDepth {
			found = append(found, fmt.Sprintf("%d: %s nests %d levels deep, the maximum is %d: "+
				"use guard clauses or extract a function", fset.Position(pos).Line, name, d, MaxDepth))
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil {
				report(fn.Pos(), fn.Name.Name, fn.Body)
			}
		case *ast.FuncLit:
			report(fn.Pos(), "function literal", fn.Body)
		}
		return true
	})
	return found
}

// depth returns the deepest nesting of control statements in body.
func depth(body *ast.BlockStmt) int {
	deepest := 0
	var walk func(n ast.Node, d int)
	walk = func(n ast.Node, d int) {
		if n == nil {
			return
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n {
				return true
			}
			inner, nested := controlBody(c)
			switch {
			case !nested:
				_, isFunc := c.(*ast.FuncLit)
				return !isFunc
			case d >= deepest:
				deepest = d + 1
			}
			walk(inner, d+1)
			if s, ok := c.(*ast.IfStmt); ok && s.Else != nil {
				walkElse(walk, s.Else, d)
			}
			return false
		})
	}
	walk(body, 0)
	return deepest
}

// walkElse walks an else branch: an else-if stays at the level of its if, an else block is one deeper.
func walkElse(walk func(ast.Node, int), els ast.Stmt, d int) {
	if elseIf, ok := els.(*ast.IfStmt); ok {
		walk(&ast.BlockStmt{List: []ast.Stmt{elseIf}}, d)
		return
	}
	walk(els, d+1)
}

// controlBody returns the body of a control statement, and false for any other node.
func controlBody(n ast.Node) (*ast.BlockStmt, bool) {
	switch s := n.(type) {
	case *ast.IfStmt:
		return s.Body, true
	case *ast.ForStmt:
		return s.Body, true
	case *ast.RangeStmt:
		return s.Body, true
	case *ast.SwitchStmt:
		return s.Body, true
	case *ast.TypeSwitchStmt:
		return s.Body, true
	case *ast.SelectStmt:
		return s.Body, true
	}
	return nil, false
}
