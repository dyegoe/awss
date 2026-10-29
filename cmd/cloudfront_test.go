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
	"reflect"
	"testing"

	searchCloudFront "github.com/dyegoe/awss/search/cloudfront"
)

// TestCloudfrontFilterFlags_coversStruct checks every cloudfrontFilters field has a registered flag
// in the --all list.
func TestCloudfrontFilterFlags_coversStruct(t *testing.T) {
	if cloudfrontCmd.Flags().Lookup(flagAll) == nil {
		cloudfrontInitFlags()
	}
	checkFilterFlags(t, "cloudfront", cloudfrontCmd, reflect.TypeOf(cloudfrontFilters{}).NumField(), cloudfrontFilterFlags)
}

// TestExecute_cloudfront checks the filters and options every cloudfront flag sends to the
// search, and the invalid values rejected before any search.
func TestExecute_cloudfront(t *testing.T) {
	runFilterCases(t, "cloudfront", "id", []filterCase{
		{name: "--all", args: []string{"cloudfront", "-a"}, wantFilters: map[string][]string{}},
		{name: "--ids", args: []string{"cloudfront", "-i", "E1,E2"}, wantFilters: map[string][]string{"id": {"E1", "E2"}}},
		{name: "--domains", args: []string{"cloudfront", "-d", "www.example.com,*.example.org"},
			wantFilters: map[string][]string{"domain": {"www.example.com", "*.example.org"}}},
		{name: "--origins with --regex", args: []string{"cloudfront", "-O", `\.s3\.`, "--regex"},
			wantFilters: map[string][]string{"origin": {`\.s3\.`}}, wantRegex: true},
		{name: "--tags", args: []string{"cloudfront", "-t", "Env=prod:stg,Team=web"},
			wantFilters: map[string][]string{"tag": {"Env=prod:stg", "Team=web"}}},
		{name: "--sort", args: []string{"cloudfront", "-a", "--sort", "modified"}, wantFilters: map[string][]string{},
			wantSort: "modified"},
		{name: "no filter", args: []string{"cloudfront"}, wantErr: "at least one filter"},
		{name: "--all with a filter", args: []string{"cloudfront", "-a", "-d", "x"}, wantErr: "--all cannot be combined"},
		{name: "bad domain glob", args: []string{"cloudfront", "-d", "[a"}, wantErr: "domains: invalid pattern"},
		{name: "bad origin regex", args: []string{"cloudfront", "-O", "(a", "--regex"},
			wantErr: "origins: invalid regular expression"},
		{name: "bad tag", args: []string{"cloudfront", "-t", "Env"}, wantErr: "invalid tag format: Env"},
		{name: "bad sort field", args: []string{"cloudfront", "-a", "--sort", "aliases"}, wantErr: "aliases"},
	})
}

// TestExecute_cloudfrontProfiles checks that cloudfront searches every profile once, in the
// global region whatever --regions says, and fetches tags with --show-tags.
func TestExecute_cloudfrontProfiles(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantProfiles []string
		wantShowTags bool
	}{
		{name: "default profile", args: []string{"cloudfront", "-a"}, wantProfiles: []string{""}},
		{
			name:         "several profiles, regions ignored",
			args:         []string{"cloudfront", "-a", "--profiles", "dev,prod", "--regions", "eu-west-1,us-east-2"},
			wantProfiles: []string{"dev", "prod"},
		},
		{
			name:         "all profiles, --region ignored, --show-tags",
			args:         []string{"cloudfront", "-a", "--profile", "all", "--region", "eu-west-1", "--show-tags"},
			wantProfiles: []string{"default", "dev", "prod"}, wantShowTags: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := resetCLI(t, nil)
			useAwsConfig(t, testAwsConfig)

			if _, err := runCLI(t, tt.args...); err != nil || len(*calls) != 1 {
				t.Fatalf("Execute(%v) error = %v, searches %d, want one search", tt.args, err, len(*calls))
			}
			got := (*calls)[0]
			if !reflect.DeepEqual(got.profiles, tt.wantProfiles) {
				t.Errorf("Execute(%v) profiles = %v, want %v", tt.args, got.profiles, tt.wantProfiles)
			}
			if want := []string{searchCloudFront.Region}; !reflect.DeepEqual(got.regions, want) {
				t.Errorf("Execute(%v) regions = %v, want %v", tt.args, got.regions, want)
			}
			if got.opts.ShowTags != tt.wantShowTags {
				t.Errorf("Execute(%v) ShowTags = %v, want %v", tt.args, got.opts.ShowTags, tt.wantShowTags)
			}
		})
	}
}
