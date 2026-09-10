// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshagent

import (
	"time"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

const (
	// ArchWindows32 is the Windows x86-32 service agent.
	ArchWindows32 = artifact.ArchWindows32
	// ArchWindows64 is the Windows x86-64 service agent.
	ArchWindows64 = artifact.ArchWindows64
	// ArchWindowsARM64 is the Windows ARM-64 service agent.
	ArchWindowsARM64 = artifact.ArchWindowsARM64

	// FlagUnset is the zero value; it resolves to FlagBackgroundOnly.
	FlagUnset = artifact.FlagUnset
	// FlagInteractiveAndBackground offers both the connect button and
	// install/uninstall. This is MeshCentral's own default.
	FlagInteractiveAndBackground = artifact.FlagInteractiveAndBackground
	// FlagInteractiveOnly offers only the connect button.
	FlagInteractiveOnly = artifact.FlagInteractiveOnly
	// FlagBackgroundOnly offers only install and uninstall, with no connect
	// button.
	FlagBackgroundOnly = artifact.FlagBackgroundOnly

	// DefaultServiceName is the Windows service name MeshAgent registers
	// unless the device group overrides it.
	DefaultServiceName = lifecycle.DefaultServiceName

	// defaultTimeout bounds a download when the caller supplies no client.
	defaultTimeout = 5 * time.Minute

	// emptyValue is the absent configuration string.
	emptyValue = ""

	errFmtDownload    = "meshagent: download: %w"
	errFmtSettings    = "meshagent: settings: %w"
	errFmtInstall     = "meshagent: install: %w"
	errFmtUninstall   = "meshagent: uninstall: %w"
	errFmtConnect     = "meshagent: connect: %w"
	errFmtDisconnect  = "meshagent: disconnect: %w"
	errFmtStatus      = "meshagent: status: %w"
	errFmtQueryStatus = "meshagent: query status: %w"

	// unsetArch is the zero Arch, meaning "detect".
	unsetArch = Arch(0)

	// schemeHTTP and schemeHTTPS are the accepted ServerURL schemes.
	schemeHTTP  = "http"
	schemeHTTPS = "https"

	// pathSeparator ends a server URL that needs trimming.
	pathSeparator = "/"
)
