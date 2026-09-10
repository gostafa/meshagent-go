// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package agentinstall registers and removes the agent's Windows service by
// running the agent binary's own install switches.
//
// It is the outbound adapter behind the lifecycle.Installer port. Running the
// binary sits behind the Runner seam and locating the installed copy behind
// Locator, so the elevation check and the error mapping are testable on any
// host.
package agentinstall
