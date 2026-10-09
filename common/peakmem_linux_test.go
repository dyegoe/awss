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

import "testing"

// TestMaxRSSBytes checks that Linux's ru_maxrss, in kilobytes, is converted to bytes.
func TestMaxRSSBytes(t *testing.T) {
	if got := maxRSSBytes(150_000); got != 153_600_000 {
		t.Errorf("maxRSSBytes(150000) = %d, want 153600000", got)
	}
}
