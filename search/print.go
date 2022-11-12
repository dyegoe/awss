package search

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/markkurossi/tabulate"
	"github.com/spf13/viper"
)

// ValidateOutput validates the output options
func ValidateOutput(output string) error {
	o := strings.Split(output, ":")
	switch o[0] {
	case "table":
		if len(o) == 1 {
			return nil
		} else if len(o) == 2 {
			if _, ok := tabulate.Styles[o[1]]; ok {
				return nil
			}
		}
	case "json":
		if len(o) == 1 {
			return nil
		} else if len(o) == 2 {
			if o[1] == "pretty" {
				return nil
			}
		}
	}
	return fmt.Errorf("invalid output format: %s", output)
}

// print prints theh search results
func printResult(sChan <-chan search, output string, showEmptyResults bool, done chan<- bool) {
	o := strings.Split(output, ":")
	for s := range sChan {
		switch o[0] {
		case "table":
			if len(o) == 1 {
				printTable(s, viper.GetString("table_style"), showEmptyResults)
			} else if len(o) >= 2 {
				printTable(s, o[1], showEmptyResults)
			}
		case "json":
			printJSON(s, showEmptyResults)
			if len(o) > 1 && o[1] == "pretty" {
				printJSONPretty(s, showEmptyResults)
			}
		}
	}
	done <- true
}

// printTable prints search result as a table
func printTable(s search, tableType string, showEmptyResults bool) {
	table := tabulate.New(tabulate.Styles[tableType])
	headers := s.getHeaders()
	rows := s.getRows()

	if (len(rows) > 0 || showEmptyResults) && tableType != "json" && tableType != "csv" {
		fmt.Println("[+] [profile]", s.getProfile(), "[region]", s.getRegion())
	}
	err := s.getError()
	if len(err) > 0 && showEmptyResults {
		fmt.Println(err)
	}
	if len(rows) == 0 {
		if showEmptyResults {
			fmt.Printf("No results found\n\n")
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
	fmt.Println()
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
