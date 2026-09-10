// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentservice

import (
	"time"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

type (
	// Ops performs service control manager operations by service name. It is
	// the seam tests replace, since a unit test has no real service to drive.
	//
	// Implementations report a missing service as agenterr.ErrNotInstalled, an
	// already-running Start as nil, and an already-stopped Stop as nil.
	Ops interface {
		Status(name string) (lifecycle.Status, error)
		Start(name string) error
		Stop(name string) error
	}

	// Controller answers the lifecycle.Controller port.
	Controller struct {
		ops  Ops
		name string
		poll time.Duration
	}
)
