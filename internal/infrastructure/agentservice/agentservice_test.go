// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
)

const serviceName = "Mesh Agent"

var errOps = errors.New("scm refused")

// fakeOps scripts a sequence of statuses so a stop can be observed in progress
// and then complete.
type fakeOps struct {
	statuses  []lifecycle.Status
	statusErr error
	startErr  error
	stopErr   error
	calls     int
}

func (ops *fakeOps) Status(string) (lifecycle.Status, error) {
	if ops.statusErr != nil {
		return lifecycle.Status{}, ops.statusErr
	}

	index := min(ops.calls, len(ops.statuses)-1)
	ops.calls++

	return ops.statuses[index], nil
}

func (ops *fakeOps) Start(string) error { return ops.startErr }
func (ops *fakeOps) Stop(string) error  { return ops.stopErr }

func newController(ops Ops) *Controller {
	ctl := New(ops, serviceName)
	// Keep the polling loop from dominating the test runtime.
	ctl.poll = time.Millisecond

	return ctl
}

func running(isRunning bool) lifecycle.Status {
	state := StateStopped
	if isRunning {
		state = StateRunning
	}

	return lifecycle.Status{Installed: true, Running: isRunning, State: state}
}

func TestStatus(t *testing.T) {
	ops := &fakeOps{statuses: []lifecycle.Status{running(true)}}

	status, err := newController(ops).Status(t.Context())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !status.Running || status.State != StateRunning {
		t.Errorf("status = %+v, want a running service", status)
	}
}

func TestStatusFailure(t *testing.T) {
	ops := &fakeOps{statusErr: errOps}

	_, err := newController(ops).Status(t.Context())
	if !errors.Is(err, errOps) {
		t.Fatalf("err = %v, want errOps", err)
	}
}

func TestStatusCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	ops := &fakeOps{statuses: []lifecycle.Status{running(true)}}

	_, err := newController(ops).Status(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestConnect(t *testing.T) {
	ops := &fakeOps{statuses: []lifecycle.Status{running(true)}}

	if err := newController(ops).Connect(t.Context()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestConnectFailure(t *testing.T) {
	ops := &fakeOps{statuses: []lifecycle.Status{running(false)}, startErr: errOps}

	err := newController(ops).Connect(t.Context())
	if !errors.Is(err, errOps) {
		t.Fatalf("err = %v, want errOps", err)
	}
}

func TestConnectCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	ops := &fakeOps{statuses: []lifecycle.Status{running(false)}}

	err := newController(ops).Connect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestDisconnectWaitsForStopped(t *testing.T) {
	// Running, still running, then stopped: the loop must poll past the first
	// two before returning.
	ops := &fakeOps{statuses: []lifecycle.Status{running(true), running(true), running(false)}}

	if err := newController(ops).Disconnect(t.Context()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if ops.calls < 3 {
		t.Errorf("polled %d times, want at least 3", ops.calls)
	}
}

func TestDisconnectAlreadyStopped(t *testing.T) {
	ops := &fakeOps{statuses: []lifecycle.Status{running(false)}}

	if err := newController(ops).Disconnect(t.Context()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestDisconnectStopFailure(t *testing.T) {
	ops := &fakeOps{statuses: []lifecycle.Status{running(true)}, stopErr: errOps}

	err := newController(ops).Disconnect(t.Context())
	if !errors.Is(err, errOps) {
		t.Fatalf("err = %v, want errOps", err)
	}
}

func TestDisconnectStatusFailureWhileWaiting(t *testing.T) {
	ops := &fakeOps{statusErr: errOps}

	err := newController(ops).Disconnect(t.Context())
	if !errors.Is(err, errOps) {
		t.Fatalf("err = %v, want errOps", err)
	}
}

func TestDisconnectCancelledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	// Never reaches stopped, so the wait is what ends the call.
	ops := &fakeOps{statuses: []lifecycle.Status{running(true)}}

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := newController(ops).Disconnect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
