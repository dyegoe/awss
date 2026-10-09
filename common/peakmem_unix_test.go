//go:build linux || darwin

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

// TestPeakMemory checks that the peak memory of the running test is reported and plausible: more
// than 1 MB, which a Go binary always holds, and less than 64 GB, which a kilobyte count read as
// bytes, or the reverse, would miss.
func TestPeakMemory(t *testing.T) {
	got, ok := PeakMemory()
	if !ok {
		t.Fatal("PeakMemory() ok = false, want true")
	}
	if got < 1<<20 || got > 64<<30 {
		t.Errorf("PeakMemory() = %d bytes, want between 1 MB and 64 GB", got)
	}
}
