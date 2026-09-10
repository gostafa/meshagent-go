// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows

package winscm

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "Mesh Agent"

var errSCM = errors.New("scm refused")

// fakeSyscalls scripts the service control manager so the error mapping can be
// covered without registering a real service.
type fakeSyscalls struct {
	openErr    error
	queryErr   error
	configErr  error
	startErr   error
	controlErr error
	closeErr   error
	state      svc.State
	binaryPath string
}

func (calls *fakeSyscalls) Open(name string, _ uint32) (*mgr.Service, error) {
	if calls.openErr != nil {
		return nil, calls.openErr
	}

	return &mgr.Service{Name: name}, nil
}

func (calls *fakeSyscalls) Close(*mgr.Service) error { return calls.closeErr }

func (calls *fakeSyscalls) Query(*mgr.Service) (svc.Status, error) {
	if calls.queryErr != nil {
		return svc.Status{}, calls.queryErr
	}

	return svc.Status{State: calls.state}, nil
}

func (calls *fakeSyscalls) Config(*mgr.Service) (mgr.Config, error) {
	if calls.configErr != nil {
		return mgr.Config{}, calls.configErr
	}

	return mgr.Config{BinaryPathName: calls.binaryPath}, nil
}

func (calls *fakeSyscalls) Start(*mgr.Service) error { return calls.startErr }

func (calls *fakeSyscalls) Control(*mgr.Service, svc.Cmd) (svc.Status, error) {
	if calls.controlErr != nil {
		return svc.Status{}, calls.controlErr
	}

	return svc.Status{State: svc.StopPending}, nil
}

func TestStatusRunning(t *testing.T) {
	calls := &fakeSyscalls{state: svc.Running, binaryPath: `"C:\Mesh\MeshAgent.exe" -run`}

	status, err := NewWith(calls, serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !status.Installed || !status.Running || status.State != "running" {
		t.Errorf("status = %+v, want an installed running service", status)
	}

	if status.BinaryPath != `C:\Mesh\MeshAgent.exe` {
		t.Errorf("BinaryPath = %q, want the unquoted executable", status.BinaryPath)
	}
}

func TestStatusAbsentServiceIsNotAnError(t *testing.T) {
	calls := &fakeSyscalls{openErr: windows.ERROR_SERVICE_DOES_NOT_EXIST}

	status, err := NewWith(calls, serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if status.Installed {
		t.Error("an absent service reported Installed")
	}
}

func TestStatusOpenFailure(t *testing.T) {
	calls := &fakeSyscalls{openErr: errSCM}

	_, err := NewWith(calls, serviceName).Status(serviceName)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStatusQueryFailure(t *testing.T) {
	calls := &fakeSyscalls{queryErr: errSCM}

	_, err := NewWith(calls, serviceName).Status(serviceName)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStatusUnreadableConfigLeavesPathEmpty(t *testing.T) {
	calls := &fakeSyscalls{state: svc.Stopped, configErr: errSCM}

	status, err := NewWith(calls, serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !status.Installed || status.BinaryPath != "" {
		t.Errorf("status = %+v, want installed with no path", status)
	}
}

func TestStatusCloseFailureIsTolerated(t *testing.T) {
	calls := &fakeSyscalls{state: svc.Stopped, closeErr: errSCM}

	if _, err := NewWith(calls, serviceName).Status(serviceName); err != nil {
		t.Fatalf("Status: %v", err)
	}
}

func TestInstalled(t *testing.T) {
	calls := &fakeSyscalls{state: svc.Running}

	status, err := NewWith(calls, serviceName).Installed(t.Context())
	if err != nil {
		t.Fatalf("Installed: %v", err)
	}

	if !status.Installed {
		t.Error("Installed reported a missing service")
	}
}

func TestInstalledCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := NewWith(&fakeSyscalls{}, serviceName).Installed(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestStartAndStop(t *testing.T) {
	ops := NewWith(&fakeSyscalls{state: svc.Stopped}, serviceName)

	if err := ops.Start(serviceName); err != nil {
		t.Errorf("Start: %v", err)
	}

	if err := ops.Stop(serviceName); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestStartAlreadyRunningIsNotAnError(t *testing.T) {
	calls := &fakeSyscalls{startErr: windows.ERROR_SERVICE_ALREADY_RUNNING}

	if err := NewWith(calls, serviceName).Start(serviceName); err != nil {
		t.Fatalf("Start on a running service: %v", err)
	}
}

func TestStopAlreadyStoppedIsNotAnError(t *testing.T) {
	calls := &fakeSyscalls{controlErr: windows.ERROR_SERVICE_NOT_ACTIVE}

	if err := NewWith(calls, serviceName).Stop(serviceName); err != nil {
		t.Fatalf("Stop on a stopped service: %v", err)
	}
}

func TestStartAndStopFailures(t *testing.T) {
	tests := map[string]func(ops *Ops) error{
		"start refused": func(ops *Ops) error { return ops.Start(serviceName) },
		"stop refused":  func(ops *Ops) error { return ops.Stop(serviceName) },
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			calls := &fakeSyscalls{startErr: errSCM, controlErr: errSCM}
			if err := call(NewWith(calls, serviceName)); !errors.Is(err, errSCM) {
				t.Fatalf("err = %v, want errSCM", err)
			}
		})
	}
}

func TestStartAndStopOnAbsentService(t *testing.T) {
	tests := map[string]func(ops *Ops) error{
		"start": func(ops *Ops) error { return ops.Start(serviceName) },
		"stop":  func(ops *Ops) error { return ops.Stop(serviceName) },
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			calls := &fakeSyscalls{openErr: windows.ERROR_SERVICE_DOES_NOT_EXIST}
			if err := call(NewWith(calls, serviceName)); !errors.Is(err, agenterr.ErrNotInstalled) {
				t.Fatalf("err = %v, want ErrNotInstalled", err)
			}
		})
	}
}

func TestStartAndStopOpenFailure(t *testing.T) {
	tests := map[string]func(ops *Ops) error{
		"start": func(ops *Ops) error { return ops.Start(serviceName) },
		"stop":  func(ops *Ops) error { return ops.Stop(serviceName) },
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			calls := &fakeSyscalls{openErr: errSCM}
			if err := call(NewWith(calls, serviceName)); !errors.Is(err, errSCM) {
				t.Fatalf("err = %v, want errSCM", err)
			}
		})
	}
}

func TestStateName(t *testing.T) {
	tests := map[svc.State]string{
		svc.Stopped:         "stopped",
		svc.StartPending:    "start pending",
		svc.StopPending:     "stop pending",
		svc.Running:         "running",
		svc.ContinuePending: "continue pending",
		svc.PausePending:    "pause pending",
		svc.Paused:          "paused",
		svc.State(99):       "unknown",
	}

	for state, want := range tests {
		if got := stateName(state); got != want {
			t.Errorf("stateName(%d) = %q, want %q", state, got, want)
		}
	}
}

func TestExecutableFrom(t *testing.T) {
	tests := map[string]string{
		`"C:\Mesh\MeshAgent.exe" -run`: `C:\Mesh\MeshAgent.exe`,
		`C:\Mesh\MeshAgent.exe -run`:   `C:\Mesh\MeshAgent.exe`,
		`C:\Mesh\MeshAgent.exe`:        `C:\Mesh\MeshAgent.exe`,
		`"C:\Mesh\MeshAgent.exe`:       `C:\Mesh\MeshAgent.exe`,
		"   ":                          "",
	}

	for commandLine, want := range tests {
		if got := ExecutableFrom(commandLine); got != want {
			t.Errorf("ExecutableFrom(%q) = %q, want %q", commandLine, got, want)
		}
	}
}

func TestNewUsesTheRealSyscalls(t *testing.T) {
	ops := New(serviceName)
	if ops.Name() != serviceName {
		t.Errorf("Name() = %q, want %q", ops.Name(), serviceName)
	}
}
