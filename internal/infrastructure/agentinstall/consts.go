// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentinstall

const (
	// SwitchInstall registers the service from the binary it is run on and
	// starts it.
	SwitchInstall = "-fullinstall"

	// SwitchUninstall stops the service, removes it and deletes its files.
	SwitchUninstall = "-fulluninstall"

	// emptyPath is the absent installed-agent path.
	emptyPath = ""

	// defaultOutputLimit caps how much installer output is carried in an
	// error, which is otherwise unbounded.
	defaultOutputLimit = 4096

	// truncationMarker ends output that was cut at the limit.
	truncationMarker = "…"

	// errWrap prefixes package-local wrapped errors.
	errWrap = "agentinstall: %w"

	// placeholderExe and placeholderArg are constant CommandContext argv
	// replaced immediately with the real agent path and switch.
	placeholderExe = "meshagent"
	placeholderArg = "switch"
)
