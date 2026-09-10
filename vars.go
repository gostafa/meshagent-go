// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshagent

import (
	"errors"

	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

var (
	// ErrUnsupportedPlatform is reported by the install and service operations
	// on anything other than Windows.
	ErrUnsupportedPlatform = agenterr.ErrUnsupportedPlatform

	// ErrNotElevated means the process is not running with an elevated token.
	ErrNotElevated = agenterr.ErrNotElevated

	// ErrNotInstalled means the agent service is not registered.
	ErrNotInstalled = agenterr.ErrNotInstalled

	// ErrDownloadUnauthorized means the server refused an anonymous agent
	// download; supply Config.AuthCookie.
	ErrDownloadUnauthorized = agenterr.ErrDownloadUnauthorized

	// ErrMissingSetting means the server's device group settings were
	// incomplete.
	ErrMissingSetting = agenterr.ErrMissingSetting

	// ErrUnknownServicePath means the service is registered but its command
	// line could not be read.
	ErrUnknownServicePath = agenterr.ErrUnknownServicePath

	// errServerURLRequired and errGroupIDRequired report absent configuration.
	errServerURLRequired = errors.New("meshagent: ServerURL is required")
	errGroupIDRequired   = errors.New("meshagent: GroupID is required")
	errBadScheme         = errors.New("meshagent: ServerURL scheme must be http or https")
	errMissingHost       = errors.New("meshagent: ServerURL is missing a host")
	errBadArch           = errors.New("meshagent: unsupported architecture")
	errBadInstallFlags   = errors.New("meshagent: unsupported install flags")
)
