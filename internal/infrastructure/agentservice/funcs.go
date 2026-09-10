// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentservice

import (
	"context"
	"fmt"
	"time"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

// New returns a Controller driving the named service through ops.
func New(ops Ops, name string) *Controller {
	return &Controller{ops: ops, name: name, poll: defaultPoll}
}

// Status reports the current state of the service.
func (ctl *Controller) Status(ctx context.Context) (lifecycle.Status, error) {
	err := ctx.Err()
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf(errStatus, err)
	}

	status, queryErr := ctl.query()
	if queryErr != nil {
		return lifecycle.Status{}, fmt.Errorf(errStatus, queryErr)
	}

	return status, nil
}

// query reads the service state from the control manager.
func (ctl *Controller) query() (lifecycle.Status, error) {
	status, err := ctl.ops.Status(ctl.name)
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf(errQuery, ctl.name, err)
	}

	return status, nil
}

// Connect starts the service. The device reconnects to MeshCentral on its own
// and reappears online in its device group. An already-running service is not
// an error.
func (ctl *Controller) Connect(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("agentservice: connect: %w", err)
	}

	err = ctl.ops.Start(ctl.name)
	if err != nil {
		return fmt.Errorf("agentservice: start %q: %w", ctl.name, err)
	}

	return nil
}

// Disconnect stops the service and waits for it to reach the stopped state or
// for ctx to be canceled.
//
// The agent stays installed and its device record stays in the MeshCentral
// device group; the device simply shows as offline. An already-stopped service
// is not an error.
func (ctl *Controller) Disconnect(ctx context.Context) error {
	err := ctl.ops.Stop(ctl.name)
	if err != nil {
		return fmt.Errorf("agentservice: stop %q: %w", ctl.name, err)
	}

	err = ctl.awaitStopped(ctx)
	if err != nil {
		return fmt.Errorf("agentservice: await stop: %w", err)
	}

	return nil
}

// awaitStopped polls until the service reports stopped.
func (ctl *Controller) awaitStopped(ctx context.Context) error {
	ticker := time.NewTicker(ctl.poll)
	defer ticker.Stop()

	err := ctl.pollStopped(ctx, ticker.C)
	if err != nil {
		return fmt.Errorf("agentservice: poll: %w", err)
	}

	return nil
}

// pollStopped repeats ticks until the service is stopped or ctx ends.
func (ctl *Controller) pollStopped(ctx context.Context, ticks <-chan time.Time) error {
	for {
		done, err := ctl.tick(ctx, ticks)
		if !done {
			continue
		}

		if err != nil {
			return fmt.Errorf("agentservice: poll stopped: %w", err)
		}

		return nil
	}
}

// tick checks once and waits for the next poll when still running.
func (ctl *Controller) tick(ctx context.Context, ticks <-chan time.Time) (bool, error) {
	stopped, err := ctl.stopped()
	if err != nil {
		return true, fmt.Errorf("agentservice: tick: %w", err)
	}

	if stopped {
		return true, nil
	}

	err = wait(ctx, ticks)
	if err != nil {
		return true, fmt.Errorf("agentservice: tick wait: %w", err)
	}

	return false, nil
}

// stopped reports whether the service has reached the stopped state.
func (ctl *Controller) stopped() (bool, error) {
	status, err := ctl.query()
	if err != nil {
		return false, fmt.Errorf("agentservice: stopped: %w", err)
	}

	return !status.Running, nil
}

// wait blocks for the next tick or reports why it cannot.
func wait(ctx context.Context, tick <-chan time.Time) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("agentservice: waiting for the service to stop: %w", ctx.Err())
	case <-tick:
		return nil
	}
}
