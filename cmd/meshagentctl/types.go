// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

type (
	// mainRuntime is what main does, held behind swappable function values so
	// that main itself is executed by a test rather than left uncovered.
	mainRuntime = struct {
		run  func(args []string) int
		exit func(code int)
	}

	// mainRuntimeError carries the runtime as an error value.
	//
	// Package-level state is otherwise refused, and err-prefixed variables are
	// the documented exception, so the runtime travels as one.
	mainRuntimeError func() mainRuntime
)
