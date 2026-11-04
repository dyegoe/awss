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

package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// useHome makes a new temporary directory the home directory, with no AWSS_CONFIG, and returns
// it.
func useHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envConfig, "")
	return home
}

// writeNamedConfigs writes ~/.awss/<file> with content for every file of files, in the home
// directory of the test.
func writeNamedConfigs(t *testing.T, files map[string]string) {
	t.Helper()
	dir, err := configDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestIsConfigName checks which --config values are names and which are paths.
func TestIsConfigName(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "devops-lz2", want: true},
		{value: "config", want: true},
		{value: "prod.v2", want: true},
		{value: "devops-lz2.yaml"},
		{value: "devops-lz2.YML"},
		{value: "./devops-lz2"},
		{value: "configs/devops"},
		{value: "/etc/awss.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := isConfigName(tt.value); got != tt.want {
				t.Errorf("isConfigName(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestResolveConfig checks the file a run reads from --config, AWSS_CONFIG and the default.
func TestResolveConfig(t *testing.T) {
	tests := []struct {
		name         string
		flag         string
		env          string
		wantPath     string
		wantRequired bool
		wantName     string
	}{
		{name: "default", wantPath: "HOME/.awss/config.yaml"},
		{name: "name", flag: "devops-lz2", wantPath: "HOME/.awss/devops-lz2.yaml", wantRequired: true,
			wantName: "devops-lz2"},
		{name: "path", flag: "./x.yaml", wantPath: "./x.yaml", wantRequired: true},
		{name: "home path", flag: "~/configs/x.yaml", wantPath: "HOME/configs/x.yaml", wantRequired: true},
		{name: "env name", env: "godpowers-legacy", wantPath: "HOME/.awss/godpowers-legacy.yaml",
			wantRequired: true, wantName: "godpowers-legacy"},
		{name: "env home path", env: "~/x.yml", wantPath: "HOME/x.yml", wantRequired: true},
		{name: "flag wins over env", flag: "devops-lz2", env: "godpowers-legacy",
			wantPath: "HOME/.awss/devops-lz2.yaml", wantRequired: true, wantName: "devops-lz2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := useHome(t)
			t.Setenv(envConfig, tt.env)
			got, err := resolveConfig(tt.flag)
			if err != nil {
				t.Fatalf("resolveConfig(%q) error = %v", tt.flag, err)
			}
			want := configFile{
				path:     strings.Replace(tt.wantPath, "HOME", home, 1),
				required: tt.wantRequired, name: tt.wantName,
			}
			if got != want {
				t.Errorf("resolveConfig(%q) = %+v, want %+v", tt.flag, got, want)
			}
		})
	}
}

// TestInitConfig_named checks that a named config is read, and that a missing one names its path
// and the configs available, from the flag and from AWSS_CONFIG.
func TestInitConfig_named(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		env     string
		files   map[string]string
		wantErr string
	}{
		{name: "flag", flag: "devops-lz2", files: map[string]string{"devops-lz2.yaml": "output: json\n"}},
		{name: "env", env: "devops-lz2", files: map[string]string{"devops-lz2.yaml": "output: json\n"}},
		{name: "missing, others there", flag: "devops-lz3",
			files:   map[string]string{"devops-lz2.yaml": "", "config.yaml": "", "notes.txt": ""},
			wantErr: "Available: configs in "},
		{name: "missing, from env", env: "devops-lz3", files: map[string]string{"devops-lz2.yaml": ""},
			wantErr: "config devops-lz3 not found"},
		{name: "missing, no awss directory", flag: "devops-lz3", wantErr: "Available: no config in "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			useHome(t)
			t.Setenv(envConfig, tt.env)
			if tt.files != nil {
				writeNamedConfigs(t, tt.files)
			}
			err := initConfig(tt.flag)
			if tt.files != nil && tt.wantErr != "" && !strings.HasSuffix(err.Error(), ": config, devops-lz2") &&
				len(tt.files) > 1 {
				t.Errorf("initConfig(%q) error = %v, want the names config, devops-lz2 at the end", tt.flag, err)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("initConfig(%q) error = %v, want it to contain %q", tt.flag, err, tt.wantErr)
				}
				return
			}
			if err != nil || viper.GetString("output") != "json" {
				t.Errorf("initConfig(%q) = %v, output %q; want the named config read", tt.flag, err, viper.GetString("output"))
			}
		})
	}
}

// TestConfigNames checks the names listed: .yaml and .yml files once each, sorted, without
// directories or other files.
func TestConfigNames(t *testing.T) {
	home := useHome(t)
	writeNamedConfigs(t, map[string]string{
		"lz2.yaml": "", "lz2.yml": "", "legacy.YML": "", "config.yaml": "", "notes.txt": "",
	})
	if err := os.Mkdir(filepath.Join(home, configDirName, "old.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := configNames(filepath.Join(home, configDirName))
	if want := []string{"config", "legacy", "lz2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("configNames() = %v, want %v", got, want)
	}
	if got := configNames(filepath.Join(home, "missing")); got != nil {
		t.Errorf("configNames(missing) = %v, want nil", got)
	}
}

// TestConfigValues checks the --config completion: the names for a bare value, the YAML files for
// a path or when there is no named config.
func TestConfigValues(t *testing.T) {
	files := []string{"yaml", "yml"}
	tests := []struct {
		name          string
		configs       bool
		toComplete    string
		want          []string
		wantDirective cobra.ShellCompDirective
	}{
		{name: "names", configs: true, want: []string{"config", "devops-lz2", "godpowers-lz2"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp},
		{name: "names with a prefix", configs: true, toComplete: "dev", want: []string{"devops-lz2"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp},
		{name: "a path", configs: true, toComplete: "./", want: files,
			wantDirective: cobra.ShellCompDirectiveFilterFileExt},
		{name: "a home path", configs: true, toComplete: "~", want: files,
			wantDirective: cobra.ShellCompDirectiveFilterFileExt},
		{name: "no named config", want: files, wantDirective: cobra.ShellCompDirectiveFilterFileExt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t)
			if tt.configs {
				writeNamedConfigs(t, map[string]string{"config.yaml": "", "devops-lz2.yaml": "", "godpowers-lz2.yml": ""})
			}
			got, directive := configValues(nil, nil, tt.toComplete)
			if !reflect.DeepEqual(got, tt.want) || directive != tt.wantDirective {
				t.Errorf("configValues(%q) = %v, %d, want %v, %d", tt.toComplete, got, directive, tt.want, tt.wantDirective)
			}
		})
	}
}

// TestExecute_namedConfig checks a run with a named config, from --config and from AWSS_CONFIG:
// --profiles all expands to its all-profiles list, and --regions completes from its all-regions.
func TestExecute_namedConfig(t *testing.T) {
	for _, viaEnv := range []bool{false, true} {
		name := map[bool]string{false: "flag", true: "env"}[viaEnv]
		t.Run(name, func(t *testing.T) {
			// resetCLI gives every run a new home directory, so each one writes the config again.
			named := map[string]string{"devops-lz2.yaml": "all-profiles: [dev]\nall-regions: [eu-north-1]\n"}
			calls := resetCLI(t, nil)
			useAwsConfig(t, testAwsConfig)
			writeNamedConfigs(t, named)
			args := []string{"--config", "devops-lz2", "vpc", "--all", "--profiles", "all"}
			if viaEnv {
				t.Setenv(envConfig, "devops-lz2")
				args = args[2:]
			}
			if _, err := runCLI(t, args...); err != nil {
				t.Fatalf("Execute(%v) error = %v", args, err)
			}
			if got := (*calls)[0].profiles; !reflect.DeepEqual(got, []string{"dev"}) {
				t.Errorf("Execute(%v) profiles = %v, want [dev]", args, got)
			}

			resetCLI(t, nil)
			writeNamedConfigs(t, named)
			if viaEnv {
				t.Setenv(envConfig, "devops-lz2")
			}
			got, _ := complete(t, append(args[:len(args)-4], "vpc", "--regions", "")...)
			if !reflect.DeepEqual(got, []string{"all", "eu-north-1"}) {
				t.Errorf("complete(--regions) = %v, want [all eu-north-1]", got)
			}
		})
	}
}
