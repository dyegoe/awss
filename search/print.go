/*
Copyright © 2022 Dyego Alexandre Eugenio dyegoe@gmail.com

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
package search

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/markkurossi/tabulate"
)

// printTable prints the instances as a table
func printTable(s search) {
	table := tabulate.New(tabulate.Unicode)
	headers := s.GetHeaders()
	rows := s.GetRows()

	fmt.Println("[+] [profile]", s.GetProfile(), "[region]", s.GetRegion())
	if len(rows) == 0 {
		fmt.Println("No results found")
		return
	}

	for _, header := range headers {
		table.Header(header).SetAlign(tabulate.TL)
	}
	for _, r := range rows {
		row := table.Row()
		for _, column := range r {
			row.Column(column)
		}
	}
	table.Print(os.Stdout)
}

// printJson returns the instances as JSON
func printJson(s search) {
	json, err := json.Marshal(s)
	if err != nil {
		l.Errorf("marshalling instances", err)
	}
	fmt.Println(string(json))
}

// printJsonPretty returns the instances as pretty JSON
func printJsonPretty(s search) {
	json, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		l.Errorf("marshalling instances", err)
	}
	fmt.Println(string(json))
}
