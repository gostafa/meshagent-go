// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshagent

import (
	"net/http"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
)

type (
	// Arch identifies which agent binary to download.
	Arch = artifact.Arch

	// InstallFlags controls which operations the installed agent offers its
	// user through the MeshAgent tray dialog.
	InstallFlags = artifact.InstallFlags

	// Status describes the agent service on this machine.
	Status = struct {
		// State is the Windows service state name, for example "running" or
		// "stopped". Empty when the service is not installed.
		State string `json:"state"`
		// BinaryPath is the executable the service was registered with, with
		// quoting and trailing arguments stripped. Empty when not installed.
		BinaryPath string `json:"binary_path"`
		// Installed reports whether the service is registered.
		Installed bool `json:"installed"`
		// Running reports whether the service is running. A service that is
		// installed but stopped leaves the device registered in its device
		// group, showing as offline.
		Running bool `json:"running"`
	}

	// Config configures a Client. Only ServerURL and GroupID are required.
	// Arch and InstallFlags are stored as plain values so Config stays a
	// reusable data carrier.
	Config struct {
		// HTTPClient performs the downloads. Defaults to a client with a five
		// minute timeout.
		HTTPClient *http.Client
		// ServerURL is the MeshCentral server root, for example
		// "https://mesh.example.com". A path component is preserved, which is
		// how non-default MeshCentral domains are addressed.
		ServerURL string
		// GroupID is the device group identifier ("meshid"), as it appears in
		// the gotomesh query parameter of the group's web URL.
		GroupID string
		// ServiceName is the Windows service to control. Defaults to
		// DefaultServiceName. Override it when the device group customizes
		// meshServiceName, otherwise service control will not find the agent.
		ServiceName string
		// AuthCookie is sent when downloading. It is only needed when the
		// server enables lockagentdownload, which makes anonymous agent
		// downloads return 401.
		AuthCookie string
		// InstallFlags controls which operations the installed agent offers.
		// Defaults to FlagBackgroundOnly. Use the Flag* constants.
		InstallFlags string
		// Arch selects which agent binary to download. Defaults to
		// DetectArch(). Use the Arch* constants.
		Arch int
	}

	// Client downloads and manages the MeshCentral agent.
	Client struct {
		ports any
	}
)
