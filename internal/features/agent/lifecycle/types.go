// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package lifecycle

import (
	"context"
)

type (
	// Status describes the MeshAgent service on this machine.
	Status struct {
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

	// Elevator reports whether the current process can register services.
	Elevator interface {
		// Elevated reports whether the process is running with an elevated
		// token.
		Elevated() bool
	}

	// Installer registers and removes the agent's Windows service.
	Installer interface {
		// Install registers the service from the binary at exePath and starts
		// it. It requires an elevated process.
		Install(ctx context.Context, exePath string) error
		// Uninstall stops the service, removes it and deletes its files. It
		// requires an elevated process.
		Uninstall(ctx context.Context) error
	}

	// Controller starts and stops the installed agent.
	Controller interface {
		// Connect starts the service. An already-running service is not an
		// error.
		Connect(ctx context.Context) error
		// Disconnect stops the service and waits for it to stop. The device
		// stays registered in its device group. An already-stopped service is
		// not an error.
		Disconnect(ctx context.Context) error
		// Status reports the current state of the service.
		Status(ctx context.Context) (Status, error)
	}
)
