// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package artifact

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
	"github.com/gostafa/meshagent-go/internal/shared/mshparse"
)

// ParseSettings reads a .msh file into typed device group settings.
func ParseSettings(raw []byte) Settings {
	return mshparse.Pairs(raw)
}

// DetectArch returns the Windows agent architecture matching the architecture
// this binary was compiled for. Unrecognized architectures fall back to
// ArchWindows64, which covers the overwhelming majority of Windows endpoints.
//
// Callers staging binaries from a build host for a different endpoint should
// choose the architecture explicitly rather than rely on this.
func DetectArch() Arch {
	return archForGOARCH(runtime.GOARCH)
}

// archForGOARCH maps a GOARCH value to an agent architecture. It is separate
// from DetectArch so every branch is reachable from a test on any host.
func archForGOARCH(goarch string) Arch {
	switch goarch {
	case archGOARCH32:
		return ArchWindows32
	case archGOARCHARM64:
		return ArchWindowsARM64
	default:
		return ArchWindows64
	}
}

// Valid reports whether arch is an architecture this module can install.
func (arch Arch) Valid() bool {
	switch arch {
	case ArchWindows32, ArchWindows64, ArchWindowsARM64:
		return true
	default:
		return false
	}
}

// String implements fmt.Stringer.
func (arch Arch) String() string {
	switch arch {
	case ArchWindows32:
		return archName32
	case ArchWindows64:
		return archName64
	case ArchWindowsARM64:
		return archNameARM64
	default:
		return archNameUnknown
	}
}

// BinaryName returns the conventional filename for arch.
//
// This is a map rather than a switch because the 64-bit case and the fallback
// share an answer, and a switch that must both list every member and carry a
// default cannot express that without two identical branches.
func (arch Arch) BinaryName() string {
	names := map[Arch]string{
		ArchWindows32:    binaryName32,
		ArchWindows64:    binaryName64,
		ArchWindowsARM64: binaryNameARM64,
	}

	name, known := names[arch]
	if !known {
		return binaryName64
	}

	return name
}

// Wire returns the value MeshCentral expects in its installflags query
// parameter. FlagUnset resolves to the background-only value.
func (flags InstallFlags) Wire() int {
	wire := map[InstallFlags]int{
		FlagUnset:                    wireBackground,
		FlagBackgroundOnly:           wireBackground,
		FlagInteractiveOnly:          wireInteractive,
		FlagInteractiveAndBackground: wireBoth,
	}

	value, known := wire[flags]
	if !known {
		return wireBackground
	}

	return value
}

// Valid reports whether flags is a recognized value.
func (flags InstallFlags) Valid() bool {
	switch flags {
	case FlagUnset, FlagInteractiveAndBackground, FlagInteractiveOnly, FlagBackgroundOnly:
		return true
	default:
		return false
	}
}

// Required reports whether the settings contain everything a temporary session
// needs to reach the server.
func (settings Settings) Required() error {
	keys := [...]string{KeyMeshID, KeyServerID, KeyMeshServer}

	for index := range keys {
		if strings.TrimSpace(settings[keys[index]]) == emptyValue {
			return fmt.Errorf("%w: missing %s", agenterr.ErrMissingSetting, keys[index])
		}
	}

	return nil
}
