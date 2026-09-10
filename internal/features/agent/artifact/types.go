// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package artifact

import (
	"context"
)

type (
	// Arch identifies a MeshCentral agent binary. The values are the
	// architecture numbers MeshCentral uses in the id query parameter of its
	// agents endpoint; only the Windows service binaries are listed, since this
	// module installs services.
	Arch int

	// InstallFlags controls which operations the installed agent offers its
	// user through the MeshAgent tray dialog. The zero value is FlagUnset.
	//
	// It is string-backed so that its members share no numeric value with Arch
	// and so that the command line accepts them verbatim.
	InstallFlags string

	// Settings holds the key/value pairs of a device group's .msh file.
	Settings map[string]string

	// BinaryDownloader fetches the agent binary for a device group.
	BinaryDownloader interface {
		// Download writes the agent into destDir and returns its path. On
		// Windows the server embeds the device group settings into the
		// executable, so the result is self-contained.
		Download(ctx context.Context, destDir string) (string, error)
	}

	// SettingsSource reads a device group's .msh settings from the server.
	SettingsSource interface {
		// Settings returns the device group settings. Installing does not need
		// them; running a temporary session does.
		Settings(ctx context.Context) (Settings, error)
	}
)
