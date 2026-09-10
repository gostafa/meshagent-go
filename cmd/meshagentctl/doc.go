// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Command meshagentctl manages the MeshCentral agent on this machine.
//
// The connect command is the one to reach for: it downloads and installs the
// agent if it is not present, then makes sure the service is running. It is
// safe to run repeatedly.
//
//	meshagentctl connect    -server https://mesh.example.com -group <meshid>
//	meshagentctl disconnect
//	meshagentctl status     -json
//	meshagentctl uninstall
//
// Install and uninstall require an elevated process. Status does not.
//
// Every behavior lives in internal/cli; this package holds only the entry
// point, so that all of it is reachable from tests.
package main
