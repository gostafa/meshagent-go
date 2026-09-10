// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"errors"
	"os"

	"github.com/gostafa/meshagent-go/internal/cli"
)

func main() {
	start(runtimeFrom(errMainRuntime), os.Args[1:])
}

// defaultRuntime is the real runtime: run the command line, exit with its code.
func defaultRuntime() mainRuntime {
	return mainRuntime{run: cli.Run, exit: os.Exit}
}

// runtimeFrom unwraps the runtime carried by an error value.
func runtimeFrom(err error) mainRuntime {
	var provider mainRuntimeError

	if !errors.As(err, &provider) {
		return defaultRuntime()
	}

	return provider()
}

// start runs the command line and exits with its code.
func start(runtime mainRuntime, args []string) {
	runtime.exit(runtime.run(args))
}

// Error implements the error interface for the runtime carrier.
func (mainRuntimeError) Error() string {
	return "meshagentctl: main runtime"
}

// Unwrap terminates the error chain.
func (mainRuntimeError) Unwrap() error {
	return nil
}
