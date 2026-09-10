// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package artifact

const (
	// ArchWindows32 is the Windows x86-32 service agent.
	ArchWindows32 Arch = 3
	// ArchWindows64 is the Windows x86-64 service agent.
	ArchWindows64 Arch = 4
	// ArchWindowsARM64 is the Windows ARM-64 service agent.
	ArchWindowsARM64 Arch = 43

	// FlagUnset is the zero value; Wire resolves it to the background-only
	// wire value, which is what unattended deployment wants.
	FlagUnset InstallFlags = ""
	// FlagInteractiveAndBackground offers both the connect button and
	// install/uninstall. This is MeshCentral's own default.
	FlagInteractiveAndBackground InstallFlags = "both"
	// FlagInteractiveOnly offers only the connect button.
	FlagInteractiveOnly InstallFlags = "interactive"
	// FlagBackgroundOnly offers only install and uninstall, with no connect
	// button.
	FlagBackgroundOnly InstallFlags = "background"

	// KeyMeshID names the device group identifier in a .msh file.
	KeyMeshID = "MeshID"
	// KeyServerID names the server's agent certificate hash in a .msh file.
	KeyServerID = "ServerID"
	// KeyMeshServer names the agent WebSocket URL in a .msh file.
	KeyMeshServer = "MeshServer"

	// TemporaryCapability is the agent capability bit meaning "temporary
	// agent". MeshCentral deletes the device record when an agent announcing
	// it disconnects.
	TemporaryCapability = "0x00000020"

	// Wire values MeshCentral expects in its installflags query parameter.
	// They are separate from the InstallFlags constants so that the Go zero
	// value can mean "unset"; MeshCentral numbers these 0, 1 and 2.
	wireBoth        = 0
	wireInteractive = 1
	wireBackground  = 2

	archGOARCH32    = "386"
	archGOARCHARM64 = "arm64"

	archName32      = "windows-x86-32"
	archName64      = "windows-x86-64"
	archNameARM64   = "windows-arm-64"
	archNameUnknown = "unknown"

	binaryName32    = "meshagent32.exe"
	binaryName64    = "meshagent64.exe"
	binaryNameARM64 = "meshagentarm64.exe"

	// emptyValue is the absent .msh value.
	emptyValue = ""
)
