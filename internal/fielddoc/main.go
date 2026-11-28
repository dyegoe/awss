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

// Command fielddoc copies the doc comments of the result fields into a Go file, for the Fields
// section of --help.
//
// The binary cannot read comments at run time, so `go generate ./cmd` runs it: it reads the
// struct types of the packages under search/ and cleanup/ and writes, for every field with a
// `json` tag, the first sentence of its comment, keyed by package path, type and field name.
// A test fails when the written file differs from what it would write now.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// fileMode is the permission of the written file: only its owner reads and writes it.
const fileMode = 0o600

// scanned are the directories, relative to the module root, whose packages hold result types.
var scanned = []string{"search", "cleanup"}

func main() {
	root := flag.String("root", "..", "the module root")
	out := flag.String("out", "fielddocs_gen.go", "the file to write")
	pkg := flag.String("package", "cmd", "the package of the written file")
	flag.Parse()
	if err := run(*root, *out, *pkg); err != nil {
		fmt.Fprintln(os.Stderr, "fielddoc:", err)
		os.Exit(1)
	}
}

// run writes to out the Go source of the field docs of the module at root.
func run(root, out, pkg string) error {
	src, err := Generate(root, pkg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, src, fileMode); err != nil {
		return fmt.Errorf("writing %s: %w", out, err)
	}
	return nil
}

// Generate returns the Go source, in package pkg, of the map fieldDocs: the description of every
// field with a `json` tag of the struct types under the scanned directories of the module at root.
func Generate(root, pkg string) ([]byte, error) {
	module, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	docs := map[string]string{}
	for _, dir := range scanned {
		if err := collectTree(root, dir, module, docs); err != nil {
			return nil, err
		}
	}
	return render(pkg, docs)
}

// modulePath returns the module path of the go.mod at root.
func modulePath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	for line := range strings.Lines(string(b)) {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in %s", filepath.Join(root, "go.mod"))
}

// collectTree adds to docs the field docs of every package directory under root/dir.
func collectTree(root, dir, module string, docs map[string]string) error {
	err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == "testdata" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", p, err)
		}
		return collectDir(p, path.Join(module, filepath.ToSlash(rel)), docs)
	})
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	return nil
}

// collectDir adds to docs the field docs of the non-test Go files in dir, the package importPath.
func collectDir(dir, importPath string, docs map[string]string) error {
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return fmt.Errorf("listing %s: %w", dir, err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", name, err)
		}
		collectFile(f, importPath, docs)
	}
	return nil
}

// collectFile adds to docs the field docs of the struct types declared in f.
func collectFile(f *ast.File, importPath string, docs map[string]string) {
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			addField(field, importPath+"."+spec.Name.Name, docs)
		}
		return true
	})
}

// addField adds to docs the description of field, of the type typeKey, when it has a `json` tag
// and a comment. A field line that declares several names gives each the same description.
func addField(field *ast.Field, typeKey string, docs map[string]string) {
	if field.Tag == nil || field.Doc == nil {
		return
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return
	}
	if name, ok := reflect.StructTag(tag).Lookup("json"); !ok || name == "-" {
		return
	}
	for _, name := range field.Names {
		docs[typeKey+"."+name.Name] = describe(name.Name, field.Doc.Text())
	}
}

// leadVerbs are the verbs a field comment starts with after the field name, which describe drops.
var leadVerbs = []string{"is", "are", "say", "reports", "indicates", "tells"}

// describe returns the first sentence of the comment of the field name, without the name and the
// verb it starts with: "Query is the searched CIDR." gives "The searched CIDR.", and "Public
// reports whether it is public." gives "Whether it is public.".
func describe(name, comment string) string {
	text := firstSentence(strings.Join(strings.Fields(comment), " "))
	for _, verb := range leadVerbs {
		if rest, ok := strings.CutPrefix(text, name+" "+verb+" "); ok {
			return capitalize(rest)
		}
	}
	return text
}

// firstSentence returns text up to the end of its first sentence: a period followed by a space
// and an upper-case letter or a back quote, so "e.g. a" and "1.2 GB" do not end it.
func firstSentence(text string) string {
	for i := 0; i+2 < len(text); i++ {
		if text[i] != '.' || text[i+1] != ' ' {
			continue
		}
		next, _ := utf8.DecodeRuneInString(text[i+2:])
		if unicode.IsUpper(next) || next == '`' {
			return text[:i+1]
		}
	}
	return text
}

// capitalize returns s with its first letter in upper case.
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// render returns the gofmt-formatted Go source of the map fieldDocs in package pkg.
func render(pkg string, docs map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b bytes.Buffer
	b.WriteString("// Code generated by internal/fielddoc; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("// fieldDocs maps \"<package path>.<type>.<field>\" to the first sentence of the field's\n")
	b.WriteString("// doc comment, for the Fields section of --help. Run `go generate ./cmd` after changing one.\n")
	b.WriteString("var fieldDocs = map[string]string{\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: %q,\n", k, docs[k])
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting the generated source: %w", err)
	}
	return src, nil
}
