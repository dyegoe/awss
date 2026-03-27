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

package common

// BaseResults contains the common fields shared by all resource result types.
type BaseResults struct {
	// Profile is the profile used to search.
	Profile string `json:"profile"`

	// Region is the region used to search.
	Region string `json:"region"`

	// Errors contains the errors found during the search.
	Errors []string `json:"errors,omitempty"`

	// SortField is the field used to sort the results.
	SortField string `json:"-"`
}

// GetProfile returns the profile used to search.
func (b *BaseResults) GetProfile() string { return b.Profile }

// GetRegion returns the region used to search.
func (b *BaseResults) GetRegion() string { return b.Region }

// GetErrors returns the errors found during the search.
func (b *BaseResults) GetErrors() []string { return b.Errors }

// GetSortField returns the field used to sort the results.
func (b *BaseResults) GetSortField() string { return b.SortField }
