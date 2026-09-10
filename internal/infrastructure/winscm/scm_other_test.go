// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build !windows

package winscm

import (
	"errors"
	"testing"

	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

const serviceName = "Mesh Agent"

func TestEveryOperationRefusesOffWindows(t *testing.T) {
	ops := New(serviceName)

	if ops.Name() != serviceName {
		t.Errorf("Name() = %q, want %q", ops.Name(), serviceName)
	}

	_, statusErr := ops.Status(serviceName)
	_, installedErr := ops.Installed(t.Context())

	errs := map[string]error{
		"Status":    statusErr,
		"Installed": installedErr,
		"Start":     ops.Start(serviceName),
		"Stop":      ops.Stop(serviceName),
	}

	for name, err := range errs {
		if !errors.Is(err, agenterr.ErrUnsupportedPlatform) {
			t.Errorf("%s err = %v, want ErrUnsupportedPlatform", name, err)
		}
	}
}
