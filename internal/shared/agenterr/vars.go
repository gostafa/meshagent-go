// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agenterr

import (
	"errors"
)

var (
	// ErrUnsupportedPlatform is returned by the install, service and session
	// operations on anything other than Windows.
	ErrUnsupportedPlatform = errors.New("operation is only supported on windows")

	// ErrNotElevated means the current process is not running with an elevated
	// token. Registering or removing the agent service requires one.
	ErrNotElevated = errors.New("operation requires an elevated process")

	// ErrNotInstalled means the agent service is not registered on this
	// machine.
	ErrNotInstalled = errors.New("mesh agent service is not installed")

	// ErrDownloadUnauthorized means the server refused an anonymous agent
	// download. MeshCentral does this when lockagentdownload is enabled;
	// supply a logged-in session cookie.
	ErrDownloadUnauthorized = errors.New("server requires authentication to download the agent")

	// ErrSessionNotRunning is returned when stopping or waiting on a session
	// that has already exited.
	ErrSessionNotRunning = errors.New("session is not running")

	// ErrMissingSetting means a device group setting a temporary session needs
	// was absent from the server's response.
	ErrMissingSetting = errors.New("device group settings are incomplete")

	// ErrUnknownServicePath means the service is registered but its command
	// line could not be read, so the installed binary cannot be located.
	ErrUnknownServicePath = errors.New("cannot determine the installed agent path")
)
