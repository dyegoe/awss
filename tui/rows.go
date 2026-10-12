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

package tui

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// tagsHeader is the header of the tags column. It is left out of the table, as in the table
// output of the CLI, but the filter still matches the tags.
const tagsHeader = "Tags"

// item is one row of the table: a resource found in one profile and region.
type item struct {
	profile, region string

	// cells are the table columns after Profile and Region.
	cells []string

	// row is the result row, shown as JSON by the details view.
	row interface{}

	// words are the values the filter matches: profile, region and every value of the row, each
	// slice element and tag on its own, so a match never runs across two values.
	words []string
}

// columns returns the table headers of a resource: Profile, Region and its own headers but Tags.
func columns(headers []interface{}) []string {
	cols := []string{"Profile", "Region"}
	for _, h := range headers {
		if s := fmt.Sprint(h); s != tagsHeader {
			cols = append(cols, s)
		}
	}
	return cols
}

// newItem builds the item of a result row from its fields with a `header` tag.
func newItem(profile, region string, row interface{}) item {
	it := item{profile: profile, region: region, row: row, words: []string{profile, region}}

	v := reflect.Indirect(reflect.ValueOf(row))
	if v.Kind() != reflect.Struct {
		s := fmt.Sprint(row)
		it.cells = []string{s}
		it.words = append(it.words, s)
		return it
	}

	for i := 0; i < v.NumField(); i++ {
		header, ok := v.Type().Field(i).Tag.Lookup("header")
		if !ok {
			continue
		}
		w := fieldWords(v.Field(i))
		it.words = append(it.words, w...)
		if header != tagsHeader {
			it.cells = append(it.cells, strings.Join(w, ", "))
		}
	}
	return it
}

// fieldWords returns the values of a field: map entries as key=value pairs sorted by key, slice
// elements, and the headed fields of a struct as one value.
func fieldWords(f reflect.Value) []string {
	switch f.Kind() {
	case reflect.Map:
		words := make([]string, 0, f.Len())
		iter := f.MapRange()
		for iter.Next() {
			words = append(words, fmt.Sprintf("%v=%v", iter.Key().Interface(), iter.Value().Interface()))
		}
		sort.Strings(words)
		return words
	case reflect.Slice:
		words := make([]string, 0, f.Len())
		for i := 0; i < f.Len(); i++ {
			words = append(words, fmt.Sprint(f.Index(i).Interface()))
		}
		return words
	case reflect.Struct:
		return []string{structText(f)}
	default:
		return []string{fmt.Sprint(f.Interface())}
	}
}

// structText returns the non-empty fields of a struct with a `header` tag as "Header: value".
func structText(f reflect.Value) string {
	parts := []string{}
	for i := 0; i < f.NumField(); i++ {
		header, ok := f.Type().Field(i).Tag.Lookup("header")
		if s := strings.Join(fieldWords(f.Field(i)), ", "); ok && s != "" {
			parts = append(parts, header+": "+s)
		}
	}
	return strings.Join(parts, ", ")
}

// hit is an item that matches the filter, and how well.
type hit struct{ index, score int }

// match returns the indexes of the items that match query, the best match first.
//
// query is space-separated terms, and an item matches when every term fuzzy-matches one of its
// words: "web prod" finds the web servers of the prod profile. An empty query matches every item,
// in their order.
func match(items []item, query string) []int {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		all := make([]int, len(items))
		for i := range items {
			all[i] = i
		}
		return all
	}

	hits := []hit{}
	for i := range items {
		if score, ok := matchScore(items[i].words, terms); ok {
			hits = append(hits, hit{index: i, score: score})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return cmp.Compare(b.score, a.score) })

	idx := make([]int, len(hits))
	for i, h := range hits {
		idx[i] = h.index
	}
	return idx
}

// matchScore returns the sum of the best score of each term in words, and false when a term
// matches none of them.
func matchScore(words, terms []string) (int, bool) {
	total := 0
	for _, term := range terms {
		matches := fuzzy.Find(term, words)
		if len(matches) == 0 {
			return 0, false
		}
		total += matches[0].Score
	}
	return total, true
}
