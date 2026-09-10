// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"context"
	"io"
	"log/slog"
	"time"

	meshagent "github.com/gostafa/meshagent-go"
)

type (
	// Agent is the subset of the library the commands drive, excluding install
	// switches so the interface stays within the method budget.
	Agent interface {
		Download(ctx context.Context, destDir string) (string, error)
		Connect(ctx context.Context) error
		Disconnect(ctx context.Context) error
		Status(ctx context.Context) (meshagent.Status, error)
		ServiceName() string
	}

	// Installer registers and removes the agent service.
	Installer interface {
		Install(ctx context.Context, exePath string) error
		Uninstall(ctx context.Context) error
	}

	// Options collects the flags the commands share.
	Options struct {
		// Server is the MeshCentral server URL.
		Server string
		// Group is the device group id.
		Group string
		// ServiceName is the Windows service to control.
		ServiceName string
		// InstallFlags are the agent dialog options.
		InstallFlags string
		// Dir is where the agent binary is downloaded.
		Dir string
		// Arch is the agent architecture number, or archDetect.
		Arch int
		// Timeout bounds a command.
		Timeout time.Duration
		// AsJSON requests machine-readable status output.
		AsJSON bool
	}

	// Runner executes one command. Agent and Installer are stored as any so
	// the type stays reusable; tests inject fakes that satisfy Agent/Installer.
	Runner struct {
		// Agent overrides the client the commands drive. When nil, one is
		// built from the parsed options.
		Agent any
		// Installer overrides the install/uninstall seam. When nil, an Agent
		// that also implements Installer is reused.
		Installer any
		// Out receives machine-readable output.
		Out io.Writer
		// Err receives human messages.
		Err io.Writer
		log *slog.Logger
	}

	// usageError marks a problem with the command line rather than with the
	// operation, so it exits with ExitUsage instead of ExitError.
	usageError struct {
		err     error
		printed bool
	}

	// stagingResult is the download destination and its cleanup.
	stagingResult struct {
		cleanup func()
		dir     string
	}
)
