//go:build windows

package meshagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Install registers the MeshAgent Windows service from the binary at exePath
// and starts it.
//
// exePath must be a binary downloaded for this device group — the server embeds
// the group's settings into the executable, so the installed service knows
// which server and group to join without any further configuration.
//
// The process must be elevated; Install returns ErrNotElevated otherwise rather
// than letting the installer fail with an opaque exit code.
func (c *Client) Install(ctx context.Context, exePath string) error {
	if err := requireElevation(); err != nil {
		return err
	}

	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("meshagent: agent binary at %s: %w", exePath, err)
	}

	return runAgent(ctx, exePath, "-fullinstall")
}

// Uninstall stops the MeshAgent service, removes it, and deletes the installed
// files.
//
// The binary to run is taken from the service's own registered command line, so
// this works regardless of where the agent was installed from and does not
// assume a default install directory.
//
// Uninstalling is not the same as Disconnect: the device record in MeshCentral
// is left behind for an installed agent and must be removed from the server
// side if that is wanted.
func (c *Client) Uninstall(ctx context.Context) error {
	if err := requireElevation(); err != nil {
		return err
	}

	status, err := c.Status(ctx)
	if err != nil {
		return err
	}

	if !status.Installed {
		return ErrNotInstalled
	}

	if status.BinaryPath == "" {
		return errors.New("meshagent: could not determine the installed agent path from the service configuration")
	}

	return runAgent(ctx, status.BinaryPath, "-fulluninstall")
}

// runAgent executes the MeshAgent binary with a single switch and turns a
// non-zero exit into an ExecError carrying the output, which is where the
// binary reports what actually went wrong.
func runAgent(ctx context.Context, exePath string, args ...string) error {
	cmd := exec.CommandContext(ctx, exePath, args...)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ExecError{
			Args:     append([]string{exePath}, args...),
			ExitCode: exitErr.ExitCode(),
			Output:   strings.TrimSpace(string(output)),
		}
	}

	return fmt.Errorf("meshagent: run %s %s: %w", exePath, strings.Join(args, " "), err)
}
