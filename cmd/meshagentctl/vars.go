// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

// errMainRuntime holds the runtime main executes. Tests replace it to observe
// the arguments and the exit code without ending the test process.
var errMainRuntime error = mainRuntimeError(defaultRuntime)
