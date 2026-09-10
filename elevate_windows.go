//go:build windows

package meshagent

import "golang.org/x/sys/windows"

// IsElevated reports whether the current process is running with an elevated
// token. Install and Uninstall require one.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// requireElevation returns ErrNotElevated unless the process is elevated. It is
// checked before shelling out so callers can distinguish "not an administrator"
// from "the installer failed", which the binary's exit code alone does not
// make clear.
func requireElevation() error {
	if !IsElevated() {
		return ErrNotElevated
	}

	return nil
}
