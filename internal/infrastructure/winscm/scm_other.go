// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build !windows

package winscm

import (
	"context"
	"fmt"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

type (
	// Ops answers the agentservice.Ops and agentinstall.Locator seams. Off
	// Windows there is no service control manager, so every call refuses.
	Ops struct {
		name string
	}

	// ServiceControl is the surface Ops exposes. It exists so the package sits
	// on the main sequence off Windows, where there is otherwise no interface.
	ServiceControl interface {
		Installed(ctx context.Context) (lifecycle.Status, error)
		Name() string
		Start(name string) error
		Status(name string) (lifecycle.Status, error)
		Stop(name string) error
	}
)

// New returns an Ops for the named service.
func New(name string) *Ops {
	return &Ops{name: name}
}

// Installed is not supported on this platform.
func (*Ops) Installed(context.Context) (lifecycle.Status, error) {
	return lifecycle.Status{}, fmt.Errorf("winscm: installed: %w", agenterr.ErrUnsupportedPlatform)
}

// Name reports the service this Ops drives.
func (ops *Ops) Name() string {
	return ops.name
}

// Start is not supported on this platform.
func (*Ops) Start(string) error {
	return fmt.Errorf("winscm: start: %w", agenterr.ErrUnsupportedPlatform)
}

// Status is not supported on this platform.
func (*Ops) Status(string) (lifecycle.Status, error) {
	return lifecycle.Status{}, fmt.Errorf("winscm: status: %w", agenterr.ErrUnsupportedPlatform)
}

// Stop is not supported on this platform.
func (*Ops) Stop(string) error {
	return fmt.Errorf("winscm: stop: %w", agenterr.ErrUnsupportedPlatform)
}
