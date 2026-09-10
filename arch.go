package meshagent

import "runtime"

// Arch identifies a MeshCentral agent binary. The values are the architecture
// numbers MeshCentral uses in the id query parameter of /meshagents; only the
// Windows service binaries are listed, since this package installs services.
type Arch int

const (
	// ArchWindows32 is the Windows x86-32 service agent.
	ArchWindows32 Arch = 3
	// ArchWindows64 is the Windows x86-64 service agent.
	ArchWindows64 Arch = 4
	// ArchWindowsARM64 is the Windows ARM-64 service agent.
	ArchWindowsARM64 Arch = 43
)

// Valid reports whether a is an architecture this package can install.
func (a Arch) Valid() bool {
	switch a {
	case ArchWindows32, ArchWindows64, ArchWindowsARM64:
		return true
	default:
		return false
	}
}

// String implements fmt.Stringer.
func (a Arch) String() string {
	switch a {
	case ArchWindows32:
		return "windows-x86-32"
	case ArchWindows64:
		return "windows-x86-64"
	case ArchWindowsARM64:
		return "windows-arm-64"
	default:
		return "unknown"
	}
}

// DetectArch returns the Windows agent architecture matching the architecture
// this binary was compiled for. Unrecognised architectures fall back to
// ArchWindows64, which covers the overwhelming majority of Windows endpoints.
//
// Cross-compiled callers, and callers staging binaries from a build host for a
// different endpoint, should set Config.Arch explicitly rather than rely on
// this.
func DetectArch() Arch {
	switch runtime.GOARCH {
	case "386":
		return ArchWindows32
	case "arm64":
		return ArchWindowsARM64
	default:
		return ArchWindows64
	}
}

// InstallFlags controls which operations the installed agent offers its user
// through the MeshAgent tray dialog. The zero value is FlagUnset.
type InstallFlags int

const (
	// FlagUnset is the zero value. Config resolves it to FlagBackgroundOnly,
	// which is what unattended deployment wants.
	FlagUnset InstallFlags = iota
	// FlagInteractiveAndBackground offers both the connect button and
	// install/uninstall. This is MeshCentral's own default.
	FlagInteractiveAndBackground
	// FlagInteractiveOnly offers only the connect button, not install or
	// uninstall.
	FlagInteractiveOnly
	// FlagBackgroundOnly offers only install and uninstall, with no connect
	// button.
	FlagBackgroundOnly
)

// wire returns the value MeshCentral expects in the installflags query
// parameter. These differ from the Go constants above so that the zero value
// can mean "unset"; MeshCentral itself numbers them 0, 1 and 2.
func (f InstallFlags) wire() int {
	switch f {
	case FlagInteractiveOnly:
		return 1
	case FlagBackgroundOnly:
		return 2
	default:
		return 0
	}
}
