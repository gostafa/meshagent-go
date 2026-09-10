// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package meshagent manages the lifecycle of the official MeshCentral agent
// (MeshAgent) on Windows endpoints.
//
// It deliberately does not implement the MeshCentral agent wire protocol. The
// protocol on /agent.ashx is an undocumented binary handshake, and the server
// pushes a JavaScript "meshcore" to the agent that the agent executes in an
// embedded engine; almost all agent functionality lives in that pushed core
// rather than in the binary. Reimplementing it would mean reimplementing the
// core and tracking it forever. This package therefore drives the official
// binary: it downloads it from a MeshCentral server and manages it through the
// binary's own command-line switches and the Windows service manager.
//
// # Managed, not ad-hoc
//
// Install registers the MeshAgent Windows service; Connect and Disconnect
// start and stop it. The device stays registered in its MeshCentral device
// group across a Disconnect — it simply shows as offline. That is the model
// for a fleet.
//
// It is not the same as the "Connect" button in the MeshAgent tray dialog,
// which runs a temporary agent announcing capability 0x20. When such an agent
// disconnects the server deletes the device record outright.
//
// # Platform support
//
// Downloading works on any platform, so a build host can stage the binary.
// Install, Uninstall, Connect, Disconnect and Status are Windows-only and
// report ErrUnsupportedPlatform elsewhere.
//
// # Privileges
//
// Install and Uninstall register and remove a Windows service and require an
// elevated process. They report ErrNotElevated rather than failing opaquely.
package meshagent
