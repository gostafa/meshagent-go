// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package mshparse

const (
	// lineSeparator ends a .msh record. Carriage returns are trimmed with the
	// surrounding whitespace.
	lineSeparator = "\n"

	// pairSeparator divides a key from its value. Only the first occurrence
	// splits, because values such as MeshServer are URLs.
	pairSeparator = "="

	// commentPrefix marks a line to ignore.
	commentPrefix = "#"

	// emptyLine is a line with no content left after trimming.
	emptyLine = ""
)
