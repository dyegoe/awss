package bad

import "testing"

// Test_parseRow uses the gotests prefix.
func Test_parseRow(t *testing.T) {}

// TestNothing names no identifier.
func TestNothing(t *testing.T) {}

// TestResults_Collect capitalises a scenario that is not a method.
func TestResults_Collect(t *testing.T) {}

// TestResults_collect_too_long has too many parts.
func TestResults_collect_too_long(t *testing.T) {}

// TestResults_nope_badSortField puts a scenario where the method goes.
func TestResults_nope_badSortField(t *testing.T) {}

// TestMain is reserved.
func TestMain(t *testing.T) {}

func TestParseRow(t *testing.T) {
	type args struct{ in string }
}

// func TestResults_Search(t *testing.T) {
// }

var shared = []string{"a"}
