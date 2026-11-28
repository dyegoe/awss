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

package cmd

import (
	"reflect"
	"strings"
	"testing"
)

// fieldsRow is a row type with every case rowFields handles: a stacked field, a field the JSON
// output leaves out, a JSON-only field, a struct shown in one cell and a slice of structs.
type fieldsRow struct {
	ID      string            `json:"id" header:"ID" sort:"id"`
	Name    string            `json:"name,omitempty" header:"Name" stack:"" sort:"name"`
	Plain   string            `header:"Plain"`
	Hidden  string            `json:"-" header:"Hidden"`
	Extra   string            `json:"extra,omitempty"`
	Info    fieldsInfo        `json:"info" header:"Info"`
	Members []fieldsInfo      `json:"members,omitempty"`
	Unshown fieldsInfo        `json:"unshown"`
	Tags    map[string]string `json:"tags" header:"Tags"`
}

// fieldsInfo is a struct held by fieldsRow.
type fieldsInfo struct {
	Zone  string `json:"zone" header:"Zone" sort:"zone"`
	Owner string `json:"owner"`
}

// TestRowFields checks the JSON name, the column and the sort field of every case of a row type.
func TestRowFields(t *testing.T) {
	want := []field{
		{json: "id", column: "ID", sort: "id"},
		{json: "name", column: "Name (under ID)", sort: "name"},
		{json: "Plain", column: "Plain"},
		{json: "extra", column: jsonOnly},
		{json: "info", column: "Info"},
		{json: "info.zone", column: "Info: Zone", sort: "zone"},
		{json: "info.owner", column: jsonOnly},
		{json: "members", column: jsonOnly},
		{json: "members[].zone", column: jsonOnly, sort: "zone"},
		{json: "members[].owner", column: jsonOnly},
		{json: "unshown", column: jsonOnly},
		{json: "unshown.zone", column: jsonOnly, sort: "zone"},
		{json: "unshown.owner", column: jsonOnly},
		{json: "tags", column: "Tags"},
	}
	got := rowFields(reflect.TypeFor[fieldsRow](), "", "", false)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rowFields(fieldsRow) =\n%+v\nwant\n%+v", got, want)
	}
}

// TestFieldsHelp checks the Fields section of every subcommand lists every JSON field of its row
// type with a description, and marks "JSON only" exactly the fields without a `header` tag.
func TestFieldsHelp(t *testing.T) {
	resetCLI(t, nil)
	for _, sub := range subcommands {
		help := fieldsHelp(sub.cmd)
		if !strings.HasPrefix(help, fieldsTitle+"\n") {
			t.Errorf("%s: fieldsHelp() = %q, want it to start with %q", sub.cmd.CommandPath(), help, fieldsTitle)
			continue
		}
		rt := rowType(sub.engine)
		for i := range rt.NumField() {
			sf := rt.Field(i)
			checkFieldLine(t, sub.cmd.CommandPath(), help, rt, &sf)
		}
	}
}

// checkFieldLine checks the Fields section help of the command path lists the field sf of the
// row type rt on its own line, with a description, as JSON only exactly when it has no header.
func checkFieldLine(t *testing.T, path, help string, rt reflect.Type, sf *reflect.StructField) {
	t.Helper()
	name, ok := jsonName(sf)
	if !ok {
		return
	}
	line := ""
	for l := range strings.SplitSeq(help, "\n") {
		if strings.HasPrefix(l, "  "+name+" ") {
			line = l
		}
	}
	if line == "" {
		t.Errorf("%s --help: Fields has no line for %s", path, name)
		return
	}
	_, hasHeader := sf.Tag.Lookup("header")
	if gotJSONOnly := strings.Contains(line, jsonOnly); gotJSONOnly == hasHeader {
		t.Errorf("%s --help: %q, want %q: %v", path, line, jsonOnly, !hasHeader)
	}
	if fieldDocs[rt.PkgPath()+"."+rt.Name()+"."+sf.Name] == "" {
		t.Errorf("%s: %s.%s has no description: add a doc comment and run go generate ./cmd",
			path, rt.Name(), sf.Name)
	}
}

// TestFieldsHelp_noEngine checks a command with no engine has no Fields section.
func TestFieldsHelp_noEngine(t *testing.T) {
	resetCLI(t, nil)
	if got := fieldsHelp(rootCmd); got != "" {
		t.Errorf("fieldsHelp(awss) = %q, want \"\"", got)
	}
}

// TestSetUsageTemplate_fields checks --help ends with the Fields section, after the global flags,
// and a wrong flag prints the usage without it.
func TestSetUsageTemplate_fields(t *testing.T) {
	resetCLI(t, nil)
	got, err := runCLI(t, "tgw", "routes", "--help")
	if err != nil {
		t.Fatalf("awss tgw routes --help error = %v", err)
	}
	fields := strings.Index(got, fieldsTitle)
	if fields < 0 || fields < strings.Index(got, "Global Flags:") ||
		!strings.HasSuffix(got, "The route table announcement of the route, for a route from a Cloud WAN peering.\n") {
		t.Errorf("awss tgw routes --help = %q, want it to end with the Fields section", got)
	}

	resetCLI(t, nil)
	got, err = runCLI(t, "tgw", "routes", "--no-such-flag")
	if err == nil || strings.Contains(got, fieldsTitle) {
		t.Errorf("awss tgw routes --no-such-flag = %q, %v, want an error and no Fields section", got, err)
	}
}
