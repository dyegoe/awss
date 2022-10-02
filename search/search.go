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
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/markkurossi/tabulate"
)

// getString returns a pointer to a string
func getString(s string) *string {
	return aws.String(s)
}

// awsSearch is a struct to hold the AWS search
type awsSearch struct {
	profile string
	region  string
	ctx     context.Context
	cfg     aws.Config
}

// getConfig creates a new a new AWS config and returns error if something goes wrong.
func (a *awsSearch) getConfig() error {
	var err error
	a.ctx = context.TODO()
	a.cfg, err = config.LoadDefaultConfig(a.ctx, config.WithSharedConfigProfile(a.profile), config.WithRegion(a.region))
	return err
}

// table is a struct to hold the table
type table struct {
	table   *tabulate.Tabulate
	headers []string
	rows    [][]string
}

// addRow adds a row to the table
func (t *table) addRow(row []string) {
	t.rows = append(t.rows, row)
}

// newTable returns a new table
func (t *table) newTable() {
	t.table = tabulate.New(tabulate.Unicode)
}

// print creates and prints the table
func (t *table) print() {
	t.newTable()
	for _, header := range t.headers {
		t.table.Header(header).SetAlign(tabulate.TL)
	}
	for _, row := range t.rows {
		r := t.table.Row()
		for _, column := range row {
			r.Column(column)
		}
	}
	t.table.Print(os.Stdout)
}
