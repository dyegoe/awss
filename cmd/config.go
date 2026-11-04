/*
Copyright © 2026 Dyego Alexandre Eugenio github@dyego.com.br

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

// Package cmd enables the CLI commands and flags.
//
// It is based on Cobra and Viper.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// envConfig names the environment variable that sets the config, as --config does, for every
	// command of a shell session. --config wins when both are set.
	envConfig = "AWSS_CONFIG"

	// configDirName is the directory, in the home directory, of the default config and of the
	// named configs.
	configDirName = ".awss"

	// defaultConfigName is the name of the default config: ~/.awss/config.yaml.
	defaultConfigName = "config"
)

// configExts are the extensions of a config file. A named config is <name> plus the first one.
var configExts = []string{".yaml", ".yml"}

// configDir returns ~/.awss.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the home directory for the config file: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// isConfigName reports whether a --config value is a name, such as devops-lz2, rather than a
// path: it has no path separator and no config file extension.
func isConfigName(v string) bool {
	if strings.ContainsRune(v, '/') || strings.ContainsRune(v, filepath.Separator) {
		return false
	}
	return !isConfigExt(filepath.Ext(v))
}

// configFile is the config file a run reads.
type configFile struct {
	// path is the file to read.
	path string

	// required is true when the file was asked for, by --config or AWSS_CONFIG: a missing one is
	// an error. The default config may be missing.
	required bool

	// name is the config name when the file was asked for by name, "" otherwise.
	name string
}

// resolveConfig returns the config file of a run: the --config value flag, or AWSS_CONFIG when
// flag is empty, or ~/.awss/config.yaml when both are empty. A value is a name (see
// isConfigName), read as ~/.awss/<name>.yaml, or a path; a leading ~/ is the home directory.
func resolveConfig(flag string) (configFile, error) {
	v := flag
	if v == "" {
		v = os.Getenv(envConfig)
	}
	dir, err := configDir()
	if err != nil {
		return configFile{}, err
	}
	switch {
	case v == "":
		return configFile{path: filepath.Join(dir, defaultConfigName+configExts[0])}, nil
	case isConfigName(v):
		return configFile{path: filepath.Join(dir, v+configExts[0]), required: true, name: v}, nil
	case strings.HasPrefix(v, "~/"):
		return configFile{path: filepath.Join(filepath.Dir(dir), v[2:]), required: true}, nil
	default:
		return configFile{path: v, required: true}, nil
	}
}

// notFound returns the error of a config file that was asked for and does not exist. For a named
// config, it lists the names available.
func (c configFile) notFound() error {
	if c.name == "" {
		return fmt.Errorf("config file not found: %s", c.path)
	}
	dir := filepath.Dir(c.path)
	names := configNames(dir)
	available := "no config in " + dir
	if len(names) > 0 {
		available = "configs in " + dir + ": " + strings.Join(names, ", ")
	}
	return fmt.Errorf("config %s not found: %s. Available: %s", c.name, c.path, available)
}

// configNames returns the names of the configs in dir: its .yaml and .yml files, without the
// extension, sorted. It returns nil when dir cannot be read.
func configNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		name := strings.TrimSuffix(e.Name(), ext)
		if e.IsDir() || !isConfigExt(ext) || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// isConfigExt reports whether ext is the extension of a config file.
func isConfigExt(ext string) bool {
	for _, e := range configExts {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// configValues completes --config: the config names of ~/.awss for a bare value, and the YAML
// files for a value that looks like a path (it has a /, or starts with . or ~), or when there is
// no named config.
func configValues(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	files := []string{"yaml", "yml"}
	if strings.ContainsRune(toComplete, '/') || strings.HasPrefix(toComplete, ".") ||
		strings.HasPrefix(toComplete, "~") {
		return files, cobra.ShellCompDirectiveFilterFileExt
	}
	dir, err := configDir()
	if err != nil {
		return files, cobra.ShellCompDirectiveFilterFileExt
	}
	names := configNames(dir)
	if len(names) == 0 {
		return files, cobra.ShellCompDirectiveFilterFileExt
	}
	var matches []string
	for _, n := range names {
		if strings.HasPrefix(n, toComplete) {
			matches = append(matches, n)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}
