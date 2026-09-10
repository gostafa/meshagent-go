// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package agentports holds the collaborator bundle a meshagent.Client drives.
//
// Ports is a pure data carrier; Named is the narrow surface other packages
// reason about when they only need the service name.
package agentports

import (
	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

type (
	// Named reports the Windows service a Ports bundle controls.
	Named interface {
		// Name returns the Windows service name.
		Name() string
	}

	// Ports gathers the adapters a Client delegates to.
	Ports struct {
		// Downloader fetches the agent binary for a device group.
		Downloader artifact.BinaryDownloader
		// Settings reads a device group's .msh settings.
		Settings artifact.SettingsSource
		// Installer registers and removes the agent service.
		Installer lifecycle.Installer
		// Controller starts and stops the installed agent.
		Controller lifecycle.Controller
		// ServiceName is the Windows service the adapters control.
		ServiceName string
	}
)

// Name returns the Windows service name.
func (ports *Ports) Name() string {
	return ports.ServiceName
}
