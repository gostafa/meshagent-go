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
// # Two connection models
//
// The package exposes two distinct and non-interchangeable models.
//
// Install plus Connect/Disconnect is the managed model. Install registers the
// MeshAgent Windows service; Connect and Disconnect start and stop that service.
// The device stays registered in its MeshCentral device group across a
// Disconnect — it simply shows as offline. This is what you want for a fleet.
//
// StartSession is the ad-hoc model, equivalent to the "Connect" button in the
// MeshAgent tray dialog. It runs the binary in the foreground announcing agent
// capability 0x20 (Temporary). When such an agent disconnects, the server
// deletes the device record outright, along with its network interface
// information, notes, last-connect time and system information. Use it for a
// one-off support session on a machine you do not manage. Never use it for a
// device that is supposed to persist.
//
// # Platform support
//
// Downloading and reading device group settings work on any platform, so a
// build host or provisioning server can stage the binary. Install, Uninstall,
// Connect, Disconnect, Status and StartSession are Windows-only and return
// ErrUnsupportedPlatform elsewhere.
//
// # Privileges
//
// Install and Uninstall register and remove a Windows service and require an
// elevated process. They return ErrNotElevated rather than failing opaquely
// when the current process is not running with an elevated token.
package meshagent
