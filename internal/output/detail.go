package output

import "fmt"

// detailLabelWidth is the aligned label column of a `get` detail sheet. It is
// shared by every resource so the detail views look consistent (the longest
// label currently used, "Description", fits with room to spare).
const detailLabelWidth = 13

// DetailField renders one line of a `get` detail sheet: "Label: value", with
// the label column aligned so the values line up down the sheet. It is the
// human counterpart of a single-row table: `list` stays a flat table used to
// pick a resource, while `get` is this detail view used to inspect one.
func DetailField(label, value string) string {
	return fmt.Sprintf("%-*s%s", detailLabelWidth, label+":", value)
}

// OrDash renders an empty value as "-" so a detail sheet stays complete
// instead of leaving dangling "Label:" lines.
func OrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
