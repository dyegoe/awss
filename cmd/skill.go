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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const (
	// skillUse is the name of the awss skill command.
	skillUse = "skill"

	// flagDir is the flag of awss skill that writes the skill to a folder instead of printing it.
	flagDir = "dir"

	// skillName is the name of the skill: its folder under --dir, and the name of its front matter.
	skillName = "awss"

	// skillFile is the file name the Agent Skills format gives a skill.
	skillFile = "SKILL.md"

	// skillDirMode and skillFileMode are the permissions of what --dir writes: only the user's.
	skillDirMode  = 0o750
	skillFileMode = 0o600
)

// skillGuide is the hand-written part of the skill: a template that skillText fills with the
// version and the list of commands.
//
//go:embed skill.md
var skillGuide string

// skillCmd prints the agent skill, or writes it under --dir.
var skillCmd = &cobra.Command{
	Use:   skillUse,
	Short: "Print an agent skill (SKILL.md) that teaches an LLM to use awss; --dir writes it.",
	Long: `Print an agent skill: a SKILL.md file in the Agent Skills format that teaches an LLM in an agent
harness (Claude Code, GitHub Copilot, Antigravity...) how to use awss. It matches this binary:
its list of commands is built from the command tree.

--dir writes it to <dir>/awss/SKILL.md instead, replacing an older copy. Point it at the skills
folder of your harness; see the README for the folders.`,
	Example: `  awss skill
  awss skill --dir ~/.claude/skills`,
	Args: cobra.NoArgs,
	// The skill needs no config file, profile or region: skip the checks of the root command.
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
	RunE:              runSkill,
}

// initSkillCmd registers awss skill and its flag. setup calls it on every run, so it resets the
// flags first: a flag defined twice panics.
func initSkillCmd() {
	skillCmd.ResetFlags()
	skillCmd.Flags().String(flagDir, "",
		"Write the skill to <dir>/awss/SKILL.md instead of printing it. `~/.claude/skills`")
	rootCmd.AddCommand(skillCmd)
}

// runSkill prints the skill, or writes it under --dir and prints where.
func runSkill(cmd *cobra.Command, _ []string) error {
	text, err := skillText()
	if err != nil {
		return err
	}
	dir, err := cmd.Flags().GetString(flagDir)
	if err != nil {
		return fmt.Errorf("reading the --%s flag: %w", flagDir, err)
	}
	if dir == "" {
		_, err = fmt.Fprint(cmd.OutOrStdout(), text)
		return err //nolint:wrapcheck // a failed write to stdout needs no more context
	}
	path, err := writeSkill(dir, text)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "awss skill: wrote %s\n", path)
	return err //nolint:wrapcheck // a failed write to stdout needs no more context
}

// skillText returns the skill: the guide with the version of the binary and its commands.
func skillText() (string, error) {
	tmpl, err := template.New(skillFile).Parse(skillGuide)
	if err != nil {
		return "", fmt.Errorf("parsing the skill guide: %w", err)
	}
	var b strings.Builder
	data := struct{ Version, Commands string }{Version: version, Commands: skillCommands(rootCmd)}
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("writing the skill: %w", err)
	}
	return b.String(), nil
}

// skillCommands lists the commands an LLM runs, under cmd, depth first: one Markdown line per
// runnable command with its Short line. It leaves out help, completion and skill itself, which
// are not searches.
func skillCommands(cmd *cobra.Command) string {
	var lines []string
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == skillUse || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		if c.Runnable() {
			lines = append(lines, fmt.Sprintf("- `%s`: %s", c.CommandPath(), c.Short))
		}
		if sub := skillCommands(c); sub != "" {
			lines = append(lines, sub)
		}
	}
	return strings.Join(lines, "\n")
}

// writeSkill writes text to <dir>/awss/SKILL.md, creating the folders and replacing an older copy,
// and returns the path. A dir starting with ~/ is under the home directory.
func writeSkill(dir, text string) (string, error) {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding the home directory for --%s: %w", flagDir, err)
		}
		dir = filepath.Join(home, rest)
	}
	folder := filepath.Join(dir, skillName)
	if err := os.MkdirAll(folder, skillDirMode); err != nil {
		return "", fmt.Errorf("creating the skill folder: %w", err)
	}
	path := filepath.Join(folder, skillFile)
	if err := os.WriteFile(path, []byte(text), skillFileMode); err != nil {
		return "", fmt.Errorf("writing the skill: %w", err)
	}
	return path, nil
}
