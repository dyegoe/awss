package good

import "testing"

// TestNew is a function test.
func TestNew(t *testing.T) {}

// TestNew_emptyFilters is a function test with a scenario.
func TestNew_emptyFilters(t *testing.T) {}

// TestResults_GetRows is an exported method test.
func TestResults_GetRows(t *testing.T) {}

// TestResults_collect is an unexported method test.
func TestResults_collect(t *testing.T) {}

// TestResults_collect_badSortField is a method test with a scenario.
func TestResults_collect_badSortField(t *testing.T) {}

// TestParseRow tests an unexported function, capitalised.
func TestParseRow(t *testing.T) {}

// TestPageSize tests a constant.
func TestPageSize(t *testing.T) {}

// TestMain runs the tests: it is not a test, so it is not checked.
func TestMain(m *testing.M) { m.Run() }

// fixture returns a fresh fixture.
func fixture() []string { return []string{"a"} }

const want = "a"
