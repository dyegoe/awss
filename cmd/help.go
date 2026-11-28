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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The calls of cobra's usage template that list the flags, and what replaces them.
const (
	cobraLocalFlags     = ".LocalFlags.FlagUsages"
	cobraInheritedFlags = ".InheritedFlags.FlagUsages"
	awssLocalFlags      = "flagUsages .LocalFlags"
	awssInheritedFlags  = "flagUsages .InheritedFlags"
)

// fieldsTemplate ends --help with the Fields section of a command with an engine (fieldsHelp).
// It follows the usage, which ends with a new line.
const fieldsTemplate = "{{with fieldsHelp .}}\n{{.}}\n{{end}}"

// setUsageTemplate makes --help list the flags with flagUsages instead of pflag's FlagUsages, and
// end with the Fields section. The subcommands inherit the templates of the root command.
//
// The Fields section is in the help template, not the usage one: a wrong flag prints the usage,
// which stays short.
func setUsageTemplate() {
	cobra.AddTemplateFunc("flagUsages", flagUsages)
	cobra.AddTemplateFunc("fieldsHelp", fieldsHelp)
	tmpl := strings.ReplaceAll(rootCmd.UsageTemplate(), cobraLocalFlags, awssLocalFlags)
	rootCmd.SetUsageTemplate(strings.ReplaceAll(tmpl, cobraInheritedFlags, awssInheritedFlags))
	if !strings.HasSuffix(rootCmd.HelpTemplate(), fieldsTemplate) {
		rootCmd.SetHelpTemplate(rootCmd.HelpTemplate() + fieldsTemplate)
	}
}

// flagUsages lists the flags as pflag does, but prints an example placeholder once. A help text
// ending with an example in back quotes ("Keep these states. `blackhole`") makes pflag show the
// example after the flag name, and also leave it at the end of the text. flagUsages keeps the
// first one only.
func flagUsages(fs *pflag.FlagSet) string {
	clean := pflag.NewFlagSet(fs.Name(), pflag.ContinueOnError)
	fs.VisitAll(func(f *pflag.Flag) {
		clean.AddFlag(withoutTrailingExample(f))
	})
	return clean.FlagUsages()
}

// withoutTrailingExample returns f itself when its help text does not end with its example, or a
// copy that shows the example only as the placeholder.
//
// pflag takes the placeholder from the first back-quoted word of the text, else from the type of
// the value. The copy drops the word from the text and names its type after it. pflag also reads
// that type to decide whether to show and quote the default, so the copy writes the default itself.
func withoutTrailingExample(f *pflag.Flag) *pflag.Flag {
	example, text, ok := trailingExample(f.Usage)
	if !ok {
		return f
	}
	c := *f
	c.Usage = text
	c.Value = placeholder{Value: f.Value, name: example}
	c.DefValue = ""
	switch {
	case f.DefValue == "" || f.DefValue == "[]":
	case f.Value.Type() == "string":
		c.Usage += fmt.Sprintf(" (default %q)", f.DefValue)
	default:
		c.Usage += fmt.Sprintf(" (default %s)", f.DefValue)
	}
	return &c
}

// trailingExample splits a help text that ends with its only back-quoted word, such as
// "Search these route tables only, exact. `tgw-rtb-0123`", into the word and the text before it.
// A back-quoted word inside the sentence ("`all` uses every profile") is part of the text: it is
// left alone.
func trailingExample(usage string) (example, text string, ok bool) {
	if strings.Count(usage, "`") != 2 || !strings.HasSuffix(usage, "`") {
		return "", "", false
	}
	start := strings.Index(usage, "`")
	return usage[start+1 : len(usage)-1], strings.TrimSpace(usage[:start]), true
}

// placeholder is a flag value whose type is the example to show after the flag name. It is used
// only to print the help, never to parse a flag.
type placeholder struct {
	pflag.Value
	name string
}

// Type returns the example, which pflag prints as the placeholder of the flag.
func (p placeholder) Type() string { return p.name }
