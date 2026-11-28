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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestSkillText checks the skill has the front matter of the Agent Skills format (name and
// description) and names every command an LLM runs, but not help, completion or skill.
func TestSkillText(t *testing.T) {
	resetCLI(t, nil)
	got, err := skillText()
	if err != nil {
		t.Fatalf("skillText() error = %v", err)
	}
	front, _, ok := strings.Cut(strings.TrimPrefix(got, "---\n"), "\n---\n")
	if !strings.HasPrefix(got, "---\n") || !ok {
		t.Fatalf("skillText() = %q, want it to start with a front matter between --- lines", got)
	}
	if !strings.Contains(front, "\nname: awss\n") && !strings.HasPrefix(front, "name: awss\n") {
		t.Errorf("skill front matter = %q, want name: awss", front)
	}
	if !regexp.MustCompile(`(?m)^description: \S`).MatchString(front) {
		t.Errorf("skill front matter = %q, want a description", front)
	}
	for _, c := range documentedCommands(rootCmd) {
		listed := strings.Contains(got, "- `"+c.CommandPath()+"`: ")
		if c.Runnable() && c.Name() != skillUse && !listed {
			t.Errorf("skillText() does not list %s", c.CommandPath())
		}
	}
	for _, s := range []string{"`awss skill`:", "`awss completion`:", "`awss help`:"} {
		if strings.Contains(got, "- "+s) {
			t.Errorf("skillText() lists %s, want it left out", s)
		}
	}
}

// TestSkillText_guideCommands checks every awss command line of the hand-written guide names a
// command of the tree and flags that command has, so a renamed command or flag fails here.
func TestSkillText_guideCommands(t *testing.T) {
	resetCLI(t, nil)
	lines := guideCommandLines(skillGuide)
	if len(lines) < 10 {
		t.Fatalf("guideCommandLines(skill.md) = %q, want the commands of the guide", lines)
	}
	for _, line := range lines {
		checkGuideCommand(t, line)
	}
}

// guideCommandLines returns the awss commands of the guide: the inline code spans and the lines
// of code blocks that start with "awss ", each cut at a pipe.
func guideCommandLines(guide string) []string {
	var lines []string
	for _, m := range regexp.MustCompile("`(awss [^`]*)`").FindAllStringSubmatch(guide, -1) {
		lines = append(lines, m[1])
	}
	for _, m := range regexp.MustCompile(`(?m)^(awss .*)$`).FindAllStringSubmatch(guide, -1) {
		lines = append(lines, m[1])
	}
	for i, l := range lines {
		l, _, _ = strings.Cut(l, "|")
		lines[i] = strings.TrimSpace(l)
	}
	return lines
}

// checkGuideCommand checks the command line of the guide, such as
// "awss tgw routes --cidrs <ip> --match longest", names a command and flags it has. The words
// before the first that is not a command name are the command; "<command>" stands for any.
func checkGuideCommand(t *testing.T, line string) {
	t.Helper()
	words := strings.Fields(line)[1:]
	cmd := rootCmd
	for len(words) > 0 && regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(words[0]) {
		sub := findSubcommand(cmd, words[0])
		if sub == nil {
			t.Errorf("guide: %q: awss %s has no command %q", line, cmd.Name(), words[0])
			return
		}
		cmd, words = sub, words[1:]
	}
	for _, w := range words {
		name, ok := strings.CutPrefix(w, "--")
		if !ok || name == "help" {
			continue
		}
		name, _, _ = strings.Cut(name, "=")
		if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
			t.Errorf("guide: %q: %s has no flag --%s", line, cmd.CommandPath(), name)
		}
	}
}

// findSubcommand returns the subcommand of cmd named name, or nil.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, c := range cmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// TestRunSkill checks awss skill prints the skill without reading a config file or the AWS
// profiles, and --dir writes it and prints where.
func TestRunSkill(t *testing.T) {
	resetCLI(t, nil)
	got, err := runCLI(t, skillUse, "--profiles", "no-such-profile", "--config", "no-such-config")
	if err != nil {
		t.Fatalf("awss skill error = %v, want none: it needs no config or profile", err)
	}
	want, err := skillText()
	if err != nil {
		t.Fatalf("skillText() error = %v", err)
	}
	if got != want {
		t.Errorf("awss skill = %q, want the skill", got)
	}

	resetCLI(t, nil)
	dir := t.TempDir()
	got, err = runCLI(t, skillUse, "--dir", dir)
	path := filepath.Join(dir, skillName, skillFile)
	if err != nil || got != "awss skill: wrote "+path+"\n" {
		t.Errorf("awss skill --dir %s = %q, %v, want it to name %s", dir, got, err, path)
	}
}

// TestWriteSkill checks writeSkill writes <dir>/awss/SKILL.md, replaces an older copy, reads ~/
// as the home directory, and fails on a folder it cannot create.
func TestWriteSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name string
		dir  string
		want string // under home
	}{
		{name: "new folder", dir: filepath.Join(home, "skills"), want: "skills/awss/SKILL.md"},
		{name: "older copy", dir: filepath.Join(home, "skills"), want: "skills/awss/SKILL.md"},
		{name: "home", dir: "~/.claude/skills", want: ".claude/skills/awss/SKILL.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := "skill of " + tt.name
			got, err := writeSkill(tt.dir, text)
			want := filepath.Join(home, filepath.FromSlash(tt.want))
			if err != nil || got != want {
				t.Fatalf("writeSkill(%s) = %q, %v, want %q", tt.dir, got, err, want)
			}
			b, err := os.ReadFile(got)
			if err != nil || string(b) != text {
				t.Errorf("writeSkill(%s) wrote %q, %v, want %q", tt.dir, b, err, text)
			}
		})
	}

	file := filepath.Join(home, "a-file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatalf("writing %s: %v", file, err)
	}
	if _, err := writeSkill(file, "x"); err == nil || !strings.Contains(err.Error(), "creating the skill folder") {
		t.Errorf("writeSkill(a file) error = %v, want one creating the skill folder", err)
	}
	if err := os.MkdirAll(filepath.Join(home, "ro", skillName, skillFile), skillDirMode); err != nil {
		t.Fatalf("creating a folder named %s: %v", skillFile, err)
	}
	_, err := writeSkill(filepath.Join(home, "ro"), "x")
	if err == nil || !strings.Contains(err.Error(), "writing the skill") {
		t.Errorf("writeSkill(SKILL.md is a folder) error = %v, want one writing the skill", err)
	}
}
