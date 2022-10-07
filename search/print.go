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
	headers := s.getHeaders()
	rows := s.getRows()

	fmt.Println("[+] [profile]", s.getProfile(), "[region]", s.getRegion())
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

// printJSON returns the instances as JSON
func printJSON(s search) {
	json, err := json.Marshal(s)
	if err != nil {
		fmt.Println(fmt.Errorf("marshalling instances: %v", err))
	}
	fmt.Println(string(json))
}

// printJSONPretty returns the instances as pretty JSON
func printJSONPretty(s search) {
	json, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Println(fmt.Errorf("marshalling instances: %v", err))
	}
	fmt.Println(string(json))
}
