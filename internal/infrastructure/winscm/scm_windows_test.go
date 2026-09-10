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

const (
	quotedBinary = `"C:\Mesh\MeshAgent.exe" -run`
	serviceName  = "Mesh Agent"
	unquotedPath = `C:\Mesh\MeshAgent.exe`
)

var (
	errClose = errors.New("close refused")
	errSCM   = errors.New("scm refused")
)

func stubKernel() *kernel {
	return &kernel{
		openSCManager: func(*uint16, *uint16, uint32) (windows.Handle, error) {
			return windows.Handle(1), nil
		},
		closeServiceHandle: func(windows.Handle) error { return nil },
		utf16FromString:    windows.UTF16PtrFromString,
		openWinService: func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
			return windows.Handle(2), nil
		},
		closeService: func(*mgr.Service) error { return nil },
		queryService: func(*mgr.Service) (svc.Status, error) {
			return svc.Status{State: svc.Running}, nil
		},
		configService: func(*mgr.Service) (mgr.Config, error) {
			return mgr.Config{BinaryPathName: quotedBinary}, nil
		},
		startService: func(*mgr.Service, ...string) error { return nil },
		controlService: func(*mgr.Service, svc.Cmd) (svc.Status, error) {
			return svc.Status{State: svc.StopPending}, nil
		},
	}
}

func TestStatusRunning(t *testing.T) {
	status, err := NewWith(stubKernel(), serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !status.Installed || !status.Running || status.State != stateRunning {
		t.Errorf("status = %+v, want an installed running service", status)
	}

	if status.BinaryPath != unquotedPath {
		t.Errorf("BinaryPath = %q, want the unquoted executable", status.BinaryPath)
	}
}

func TestStatusAbsentServiceIsNotAnError(t *testing.T) {
	api := stubKernel()
	api.openWinService = func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
		return 0, windows.ERROR_SERVICE_DOES_NOT_EXIST
	}

	status, err := NewWith(api, serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if status.Installed {
		t.Error("an absent service reported Installed")
	}
}

func TestStatusOpenFailure(t *testing.T) {
	api := stubKernel()
	api.openWinService = func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
		return 0, errSCM
	}

	_, err := NewWith(api, serviceName).Status(serviceName)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStatusQueryFailure(t *testing.T) {
	api := stubKernel()
	api.queryService = func(*mgr.Service) (svc.Status, error) {
		return svc.Status{}, errSCM
	}

	_, err := NewWith(api, serviceName).Status(serviceName)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStatusUnreadableConfigLeavesPathEmpty(t *testing.T) {
	api := stubKernel()
	api.queryService = func(*mgr.Service) (svc.Status, error) {
		return svc.Status{State: svc.Stopped}, nil
	}
	api.configService = func(*mgr.Service) (mgr.Config, error) {
		return mgr.Config{}, errSCM
	}

	status, err := NewWith(api, serviceName).Status(serviceName)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !status.Installed || status.BinaryPath != emptyPath {
		t.Errorf("status = %+v, want installed with no path", status)
	}
}

func TestStatusCloseFailureIsTolerated(t *testing.T) {
	api := stubKernel()
	api.queryService = func(*mgr.Service) (svc.Status, error) {
		return svc.Status{State: svc.Stopped}, nil
	}
	api.closeService = func(*mgr.Service) error { return errSCM }

	if _, err := NewWith(api, serviceName).Status(serviceName); err != nil {
		t.Fatalf("Status: %v", err)
	}
}

func TestInstalled(t *testing.T) {
	status, err := NewWith(stubKernel(), serviceName).Installed(t.Context())
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

	_, err := NewWith(stubKernel(), serviceName).Installed(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestInstalledWrapsStatusFailure(t *testing.T) {
	api := stubKernel()
	api.queryService = func(*mgr.Service) (svc.Status, error) {
		return svc.Status{}, errSCM
	}

	_, err := NewWith(api, serviceName).Installed(t.Context())
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStartAndStop(t *testing.T) {
	ops := NewWith(stubKernel(), serviceName)

	if err := ops.Start(serviceName); err != nil {
		t.Errorf("Start: %v", err)
	}

	if err := ops.Stop(serviceName); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestStartAlreadyRunningIsNotAnError(t *testing.T) {
	api := stubKernel()
	api.startService = func(*mgr.Service, ...string) error {
		return windows.ERROR_SERVICE_ALREADY_RUNNING
	}

	if err := NewWith(api, serviceName).Start(serviceName); err != nil {
		t.Fatalf("Start on a running service: %v", err)
	}
}

func TestStopAlreadyStoppedIsNotAnError(t *testing.T) {
	api := stubKernel()
	api.controlService = func(*mgr.Service, svc.Cmd) (svc.Status, error) {
		return svc.Status{}, windows.ERROR_SERVICE_NOT_ACTIVE
	}

	if err := NewWith(api, serviceName).Stop(serviceName); err != nil {
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
			api := stubKernel()
			api.startService = func(*mgr.Service, ...string) error { return errSCM }
			api.controlService = func(*mgr.Service, svc.Cmd) (svc.Status, error) {
				return svc.Status{}, errSCM
			}

			if err := call(NewWith(api, serviceName)); !errors.Is(err, errSCM) {
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
			api := stubKernel()
			api.openWinService = func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
				return 0, windows.ERROR_SERVICE_DOES_NOT_EXIST
			}

			if err := call(NewWith(api, serviceName)); !errors.Is(err, agenterr.ErrNotInstalled) {
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
			api := stubKernel()
			api.openWinService = func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
				return 0, errSCM
			}

			if err := call(NewWith(api, serviceName)); !errors.Is(err, errSCM) {
				t.Fatalf("err = %v, want errSCM", err)
			}
		})
	}
}

func TestStateName(t *testing.T) {
	tests := map[svc.State]string{
		svc.Stopped:         stateStopped,
		svc.StartPending:    stateStartPending,
		svc.StopPending:     stateStopPending,
		svc.Running:         stateRunning,
		svc.ContinuePending: stateContinuePending,
		svc.PausePending:    statePausePending,
		svc.Paused:          statePaused,
		svc.State(99):       stateUnknown,
	}

	for state, want := range tests {
		if got := stateName(state); got != want {
			t.Errorf("stateName(%d) = %q, want %q", state, got, want)
		}
	}
}

func TestExecutableFrom(t *testing.T) {
	tests := map[string]string{
		quotedBinary:                 unquotedPath,
		`"C:\Mesh\MeshAgent.exe"`:    unquotedPath,
		`C:\Mesh\MeshAgent.exe -run`: unquotedPath,
		unquotedPath:                 unquotedPath,
		`"C:\Mesh\MeshAgent.exe`:     unquotedPath,
		"   ":                        emptyPath,
	}

	for commandLine, want := range tests {
		if got := ExecutableFrom(commandLine); got != want {
			t.Errorf("ExecutableFrom(%q) = %q, want %q", commandLine, got, want)
		}
	}
}

func TestNewWiresLiveKernel(t *testing.T) {
	ops := New(serviceName)
	if ops.Name() != serviceName {
		t.Errorf("Name() = %q, want %q", ops.Name(), serviceName)
	}

	api, ok := ops.calls.(*kernel)
	if !ok {
		t.Fatalf("calls type %T, want *kernel", ops.calls)
	}

	if api.openSCManager == nil || api.closeServiceHandle == nil || api.utf16FromString == nil ||
		api.openWinService == nil || api.closeService == nil || api.queryService == nil ||
		api.configService == nil || api.startService == nil || api.controlService == nil {
		t.Error("liveKernel left a nil field")
	}
}

func TestOpenConnectManagerFailure(t *testing.T) {
	api := stubKernel()
	api.openSCManager = func(*uint16, *uint16, uint32) (windows.Handle, error) {
		return 0, errSCM
	}

	_, err := api.Open(serviceName, windows.SERVICE_QUERY_STATUS)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestOpenInvalidServiceName(t *testing.T) {
	_, err := stubKernel().Open("Mesh\x00Agent", windows.SERVICE_QUERY_STATUS)
	if err == nil {
		t.Fatal("Open accepted a name with a NUL")
	}
}

func TestOpenKeepsServiceErrorWhenManagerCloseFails(t *testing.T) {
	api := stubKernel()
	api.openWinService = func(windows.Handle, *uint16, uint32) (windows.Handle, error) {
		return 0, errSCM
	}
	api.closeServiceHandle = func(windows.Handle) error { return errClose }

	_, err := api.Open(serviceName, windows.SERVICE_QUERY_STATUS)
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}

	if errors.Is(err, errClose) {
		t.Fatal("manager close error replaced the open error")
	}
}

func TestOpenReportsManagerCloseFailure(t *testing.T) {
	api := stubKernel()
	api.closeServiceHandle = func(windows.Handle) error { return errClose }

	service, err := api.Open(serviceName, windows.SERVICE_QUERY_STATUS)
	if !errors.Is(err, errClose) {
		t.Fatalf("err = %v, want errClose", err)
	}

	if service != nil {
		t.Fatal("Open returned a service alongside a manager close error")
	}
}

func TestOpenAbandonsServiceWhenManagerCloseFails(t *testing.T) {
	api := stubKernel()
	api.closeServiceHandle = func(windows.Handle) error { return errClose }
	api.closeService = func(*mgr.Service) error { return errSCM }

	_, err := api.Open(serviceName, windows.SERVICE_QUERY_STATUS)
	if !errors.Is(err, errClose) {
		t.Fatalf("err = %v, want errClose", err)
	}
}

func TestCloseFailure(t *testing.T) {
	api := stubKernel()
	api.closeService = func(*mgr.Service) error { return errSCM }

	if err := api.Close(&mgr.Service{Name: serviceName}); !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestControlFailure(t *testing.T) {
	api := stubKernel()
	api.controlService = func(*mgr.Service, svc.Cmd) (svc.Status, error) {
		return svc.Status{}, errSCM
	}

	if err := api.Control(&mgr.Service{Name: serviceName}, svc.Stop); !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestStartFailure(t *testing.T) {
	api := stubKernel()
	api.startService = func(*mgr.Service, ...string) error { return errSCM }

	if err := api.Start(&mgr.Service{Name: serviceName}); !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}

func TestSnapshotQueryFailure(t *testing.T) {
	api := stubKernel()
	api.queryService = func(*mgr.Service) (svc.Status, error) {
		return svc.Status{}, errSCM
	}

	_, _, err := api.Snapshot(&mgr.Service{Name: serviceName})
	if !errors.Is(err, errSCM) {
		t.Fatalf("err = %v, want errSCM", err)
	}
}
