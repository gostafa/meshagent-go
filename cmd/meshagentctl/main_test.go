// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"errors"
	"os"
	"testing"
)

var errNotARuntime = errors.New("not a runtime")

// TestMainRunsThroughTheSwappableRuntime covers main itself, which is
// otherwise a statement no test can reach.
func TestMainRunsThroughTheSwappableRuntime(t *testing.T) {
	var (
		gotArgs []string
		gotCode int
	)

	original := errMainRuntime

	t.Cleanup(func() { errMainRuntime = original })

	errMainRuntime = mainRuntimeError(func() mainRuntime {
		return mainRuntime{
			run: func(args []string) int {
				gotArgs = args

				return 7
			},
			exit: func(code int) { gotCode = code },
		}
	})

	os.Args = []string{"meshagentctl", "status", "-json"}

	main()

	if len(gotArgs) != 2 || gotArgs[0] != "status" {
		t.Errorf("args = %v, want the arguments after the program name", gotArgs)
	}

	if gotCode != 7 {
		t.Errorf("exit code = %d, want 7", gotCode)
	}
}

func TestRuntimeFromFallsBackWhenTheErrorIsUnrelated(t *testing.T) {
	runtime := runtimeFrom(errNotARuntime)

	if runtime.run == nil || runtime.exit == nil {
		t.Fatal("runtimeFrom returned an incomplete runtime for an unrelated error")
	}
}

func TestDefaultRuntimeIsWired(t *testing.T) {
	runtime := defaultRuntime()

	if runtime.run == nil || runtime.exit == nil {
		t.Fatal("defaultRuntime returned an incomplete runtime")
	}
}

func TestRuntimeCarrierBehavesLikeAnError(t *testing.T) {
	carrier := mainRuntimeError(defaultRuntime)

	if carrier.Error() == "" {
		t.Error("Error() is empty")
	}

	if carrier.Unwrap() != nil {
		t.Error("Unwrap() did not terminate the chain")
	}
}
