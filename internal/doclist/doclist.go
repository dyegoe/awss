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

// Package doclist checks that the docs list every Go package of the module (AGENTS.md, Standards
// and guardrails), so a new package cannot be left off the package layout of AGENTS.md or the map
// of docs/ARCHITECTURE.md.
package doclist

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Check returns one "doc: missing dir/" per package directory under root that a doc does not list,
// sorted. docs are paths relative to root.
//
// A package directory holds at least one non-test Go file; root itself, hidden and testdata
// directories are skipped. A doc lists a directory when it names it as "dir/" on its own: at the
// start of a line or after a space, backtick, parenthesis or "|", and not followed by more path,
// so "search/ebs/" does not list "search/".
func Check(root string, docs ...string) ([]string, error) {
	dirs, err := packageDirs(root)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, doc := range docs {
		b, err := os.ReadFile(filepath.Join(root, doc))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", doc, err)
		}
		for _, dir := range dirs {
			if !lists(string(b), dir) {
				missing = append(missing, fmt.Sprintf("%s: missing %s/", doc, dir))
			}
		}
	}
	sort.Strings(missing)
	return missing, nil
}

// lists reports whether text names dir as "dir/" on its own.
func lists(text, dir string) bool {
	re := regexp.MustCompile("(?m)(^|[\\s`(|])" + regexp.QuoteMeta(dir) + "/($|[\\s`)|,])")
	return re.MatchString(text)
}

// packageDirs returns the slash-separated paths, relative to root, of the package directories
// under root, sorted.
func packageDirs(root string) ([]string, error) {
	set := map[string]bool{}
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
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		if rel != "." {
			set[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing packages under %s: %w", root, err)
	}
	dirs := make([]string, 0, len(set))
	for dir := range set {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs, nil
}
