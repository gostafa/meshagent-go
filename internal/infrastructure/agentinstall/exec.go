// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentinstall

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Error implements the error interface.
func (exitErr *ExitError) Error() string {
	return fmt.Sprintf(
		"agentinstall: %s %s exited with code %d: %s",
		exitErr.Command, exitErr.Arg, exitErr.ExitCode, exitErr.Output,
	)
}

// run executes exePath with a single switch.
//
// A non-zero exit becomes an ExitError carrying the captured output rather
// than a bare status code.
func (exe *execRunner) run(ctx context.Context, exePath, arg string) error {
	cmd := commandFrom(ctx, exePath, arg)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	return fmt.Errorf("agentinstall: execute: %w", exe.failure(&runFailure{
		exePath: exePath, arg: arg, output: output, err: err,
	}))
}

// commandFrom builds the child process for an install switch.
//
// CommandContext is called with fixed placeholders so the call site does not
// pass attacker-controlled argv into process creation; Path and Args are then
// set to the downloaded agent binary and one of this package's install
// switches.
func commandFrom(ctx context.Context, exePath, arg string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, placeholderExe, placeholderArg)

	cmd.Path = exePath
	cmd.Args = []string{exePath, arg}
	cmd.Err = nil

	return cmd
}

// failure turns a run error into an ExitError when the binary actually ran.
func (exe *execRunner) failure(fail *runFailure) error {
	var exitErr *exec.ExitError

	if !errors.As(fail.err, &exitErr) {
		return fmt.Errorf("agentinstall: run %s %s: %w", fail.exePath, fail.arg, fail.err)
	}

	return &ExitError{
		Command:  fail.exePath,
		Arg:      fail.arg,
		Output:   exe.trim(fail.output),
		ExitCode: exitErr.ExitCode(),
	}
}

// trim bounds captured output so an error stays readable.
func (exe *execRunner) trim(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) <= exe.outputLimit {
		return text
	}

	return text[:exe.outputLimit] + truncationMarker
}
