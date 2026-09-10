// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build !windows

package agentinstall

type (
	// processToken answers from the real process token. Off Windows there is no
	// elevation to report.
	processToken struct{}
)

// elevated always reports false off Windows.
func (processToken) elevated() bool {
	return false
}
