package row

// testRow is in a test file, which fielddoc skips.
type testRow struct {
	// ID is never read.
	ID string `json:"id"`
}
