// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows

package agentinstall

import (
	"golang.org/x/sys/windows"
)

type (
	// processToken answers from the real process token.
	processToken struct{}
)

// elevated reports the elevation of the current process token.
func (processToken) elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
