// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshpath

const (
	// separator joins the server path prefix to an endpoint name.
	separator = "/"

	// AgentsEndpoint serves agent binaries with the device group settings
	// embedded.
	AgentsEndpoint = "meshagents"

	// SettingsEndpoint serves a device group's .msh settings.
	SettingsEndpoint = "meshsettings"
)
