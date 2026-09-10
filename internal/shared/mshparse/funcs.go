// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package mshparse

import (
	"strings"
)

// Pairs reads the "key=value" lines of a .msh file. Blank lines, comments and
// lines without a separator are ignored.
func Pairs(raw []byte) map[string]string {
	pairs := make(map[string]string)

	for line := range strings.SplitSeq(string(raw), lineSeparator) {
		key, value, ok := splitPair(line)
		if ok {
			pairs[key] = value
		}
	}

	return pairs
}

// splitPair trims a line and divides it at the first separator.
func splitPair(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == emptyLine || strings.HasPrefix(trimmed, commentPrefix) {
		return emptyLine, emptyLine, false
	}

	key, value, found := strings.Cut(trimmed, pairSeparator)
	if !found {
		return emptyLine, emptyLine, false
	}

	return strings.TrimSpace(key), strings.TrimSpace(value), true
}
