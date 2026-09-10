// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"errors"
)

var (
	errUsage       = errors.New("cli: invalid command line")
	errShortWrite  = errors.New("cli: short usage write")
	errNoInstaller = errors.New("cli: installer seam is required")
	errNoAgent     = errors.New("cli: agent seam is required")
)
