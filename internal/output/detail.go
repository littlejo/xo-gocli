package output

import (
	"fmt"
	"strings"
)

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

// JoinNames renders a list of names for a detail sheet. Up to limit names are
// joined with commas; beyond that, a count is shown instead of an unbounded
// list ("3, 42, …  (+39 more)"), keeping the sheet readable. An empty list
// renders as "-".
func JoinNames(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	const limit = 10
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:limit], ", ") + fmt.Sprintf("  (+%d more)", len(names)-limit)
}
