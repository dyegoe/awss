package search

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/markkurossi/tabulate"
)

// print prints theh search results
func printResult(sChan <-chan search, output string, showEmptyResults bool, err <-chan error) {
	for s := range sChan {
		switch output {
		case "table":
			printTable(s, showEmptyResults)
		case "json":
			printJSON(s, showEmptyResults)
		case "json-pretty":
			printJSONPretty(s, showEmptyResults)
		}
		if <-err != nil && showEmptyResults {
			fmt.Println(fmt.Errorf("searching instances: %v", err))
		}
	}
}

// printTable prints search result as a table
func printTable(s search, showEmptyResults bool) {
	table := tabulate.New(tabulate.Unicode)
	headers := s.getHeaders()
	rows := s.getRows()

	if len(rows) > 0 || showEmptyResults {
		fmt.Println("[+] [profile]", s.getProfile(), "[region]", s.getRegion())
	}
	if len(rows) == 0 {
		if showEmptyResults {
			fmt.Println("No results found")
		}
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

// printJSON prints search result as JSON
func printJSON(s search, showEmptyResults bool) {
	if len(s.getRows()) > 0 || showEmptyResults {
		json, err := json.Marshal(s)
		if err != nil {
			fmt.Println(fmt.Errorf("marshalling instances: %v", err))
		}
		fmt.Println(string(json))
	}
}

// printJSONPretty prints search result as pretty JSON
func printJSONPretty(s search, showEmptyResults bool) {
	if len(s.getRows()) > 0 || showEmptyResults {
		json, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			fmt.Println(fmt.Errorf("marshalling instances: %v", err))
		}
		fmt.Println(string(json))
	}
}
