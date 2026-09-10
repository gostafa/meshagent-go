// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentinstall

import (
	"context"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

type (
	// Locator reports where the installed agent lives, so uninstalling does
	// not assume a default directory.
	Locator interface {
		Installed(ctx context.Context) (lifecycle.Status, error)
	}

	// ExitError reports a non-zero exit from the agent binary, carrying the
	// output, which is where the binary explains what went wrong.
	ExitError struct {
		Command  string
		Arg      string
		Output   string
		ExitCode int
	}

	// Installer answers the lifecycle.Installer port.
	Installer struct {
		runner    runner
		locator   Locator
		elevation elevation
	}

	// runner executes the agent binary with a single switch. It is the seam a
	// test replaces, since running a real installer needs a real machine.
	runner interface {
		run(ctx context.Context, exePath, arg string) error
	}

	// tokenQuery reports the elevation of the current process token. It is the
	// seam a test replaces, since a test process cannot choose its own token.
	tokenQuery interface {
		elevated() bool
	}

	// execRunner runs the agent binary as a child process.
	execRunner struct {
		outputLimit int
	}

	// elevation refuses privileged operations without an elevated token.
	elevation struct {
		query tokenQuery
	}

	// runFailure carries the inputs for turning a process error into an ExitError.
	runFailure struct {
		err     error
		exePath string
		arg     string
		output  []byte
	}
)
