// Package row is a fixture of internal/fielddoc.
package row

// dataRow has every kind of field comment fielddoc reads.
type dataRow struct {
	// ID is the ID of the row. A second sentence is dropped.
	ID string `json:"id" header:"ID"`

	// Names are the names of the row, e.g. a and b.
	Names []string `json:"names"`

	// Public reports whether the row is public.
	Public bool `json:"public"`

	// Size is 1.5 GB or more,
	// on two lines.
	Size string `json:"size"`

	// Free text that does not start with the field name.
	Free string `json:"free"`

	// Skipped is left out of the JSON output.
	Skipped string `json:"-"`

	// NoTag has no tag.
	NoTag string

	// Other has no json tag.
	Other string `header:"Other"`

	Uncommented string `json:"uncommented"`

	// A and B share a line.
	A, B string `json:"a"`
}
