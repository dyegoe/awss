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

// Package cmd enables the CLI commands and flags for the EC2 engine.
package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Test_initFlags(t *testing.T) {
	flags := map[string]string{
		"ids":                "[]",
		"names":              "[]",
		"tags":               "[]",
		"tags-key":           "[]",
		"instance-types":     "[]",
		"availability-zones": "[]",
		"instance-states":    "[]",
		"private-ips":        "[]",
		"public-ips":         "[]",
		"sort":               "name",
	}

	testCmd := &cobra.Command{}
	initFlags(testCmd)
	testCmd.Flags().VisitAll(func(f *pflag.Flag) {
		t.Run(f.Name, func(t *testing.T) {
			want, ok := flags[f.Name]
			if !ok {
				t.Errorf("initFlags() flag %v not found", f.Name)
				return
			}
			if got := f.Value.String(); want != got {
				t.Errorf("initFlags() %v = %v, want %v", f.Name, got, want)
			}
		})
	})
}
