// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows

package winscm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type (
	// Syscalls is the seam over the service control manager. Tests replace it
	// to cover the error mapping without registering a real service.
	Syscalls interface {
		Open(name string, access uint32) (*mgr.Service, error)
		Close(service *mgr.Service) error
		Query(service *mgr.Service) (svc.Status, error)
		Config(service *mgr.Service) (mgr.Config, error)
		Start(service *mgr.Service) error
		Control(service *mgr.Service, cmd svc.Cmd) (svc.Status, error)
	}

	// Ops answers the agentservice.Ops and agentinstall.Locator seams.
	Ops struct {
		calls Syscalls
		name  string
	}

	// realSyscalls talks to the actual service control manager.
	realSyscalls struct{}
)

// New returns an Ops for the named service backed by the real service control
// manager.
func New(name string) *Ops {
	return &Ops{calls: realSyscalls{}, name: name}
}

// NewWith returns an Ops backed by the given seam. It exists so tests can
// drive the error mapping.
func NewWith(calls Syscalls, name string) *Ops {
	return &Ops{calls: calls, name: name}
}

// Name reports the service this Ops drives.
func (ops *Ops) Name() string {
	return ops.name
}

// Status reports the current state of the service.
//
// A service that is not registered is reported as a zero Status rather than an
// error, so callers can tell "absent" from "unreadable".
func (ops *Ops) Status(name string) (lifecycle.Status, error) {
	service, err := ops.calls.Open(name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return absentOrError(err)
	}
	defer ops.discard(service)

	state, err := ops.calls.Query(service)
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf("winscm: query %q: %w", name, err)
	}

	return ops.describe(service, state), nil
}

// Installed answers the agentinstall.Locator seam.
func (ops *Ops) Installed(ctx context.Context) (lifecycle.Status, error) {
	err := ctx.Err()
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf("winscm: installed: %w", err)
	}

	return ops.Status(ops.name)
}

// describe assembles the reported status, treating an unreadable configuration
// as a missing path rather than a failure.
func (ops *Ops) describe(service *mgr.Service, state svc.Status) lifecycle.Status {
	status := lifecycle.Status{
		Installed: true,
		Running:   state.State == svc.Running,
		State:     stateName(state.State),
	}

	config, err := ops.calls.Config(service)
	if err == nil {
		status.BinaryPath = ExecutableFrom(config.BinaryPathName)
	}

	return status
}

// Start starts the service. An already-running service is not an error.
func (ops *Ops) Start(name string) error {
	service, err := ops.calls.Open(name, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return notInstalledOrError(err, name)
	}
	defer ops.discard(service)

	err = ops.calls.Start(service)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("winscm: start %q: %w", name, err)
	}

	return nil
}

// Stop asks the service to stop. An already-stopped service is not an error.
func (ops *Ops) Stop(name string) error {
	service, err := ops.calls.Open(name, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return notInstalledOrError(err, name)
	}
	defer ops.discard(service)

	_, err = ops.calls.Control(service, svc.Stop)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return fmt.Errorf("winscm: stop %q: %w", name, err)
	}

	return nil
}

// discard closes a service handle, where there is nothing useful to do with a
// failure.
func (ops *Ops) discard(service *mgr.Service) {
	closeErr := ops.calls.Close(service)
	if closeErr != nil {
		return
	}
}

// absentOrError maps a failed open onto a zero status or a real error.
func absentOrError(err error) (lifecycle.Status, error) {
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return lifecycle.Status{}, nil
	}

	return lifecycle.Status{}, fmt.Errorf("winscm: open service: %w", err)
}

// notInstalledOrError maps a failed open onto the module's sentinel.
func notInstalledOrError(err error, name string) error {
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("winscm: %q: %w", name, agenterr.ErrNotInstalled)
	}

	return fmt.Errorf("winscm: open %q: %w", name, err)
}

// stateName renders a Windows service state as a lowercase name.
func stateName(state svc.State) string {
	names := map[svc.State]string{
		svc.Stopped:         "stopped",
		svc.StartPending:    "start pending",
		svc.StopPending:     "stop pending",
		svc.Running:         "running",
		svc.ContinuePending: "continue pending",
		svc.PausePending:    "pause pending",
		svc.Paused:          "paused",
	}

	name, known := names[state]
	if !known {
		return "unknown"
	}

	return name
}

// Open opens the service with exactly the rights the caller needs.
//
// The manager is opened with SC_MANAGER_CONNECT rather than full access so
// that read-only operations work without an elevated process. Closing the
// manager handle does not invalidate the service handle it returned.
func (realSyscalls) Open(name string, access uint32) (*mgr.Service, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("winscm: open service control manager: %w", err)
	}

	defer func() { _ = windows.CloseServiceHandle(manager) }()

	wide, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("winscm: invalid service name %q: %w", name, err)
	}

	handle, err := windows.OpenService(manager, wide, access)
	if err != nil {
		return nil, fmt.Errorf("winscm: open service %q: %w", name, err)
	}

	return &mgr.Service{Name: name, Handle: handle}, nil
}

// Close releases a service handle.
func (realSyscalls) Close(service *mgr.Service) error {
	err := service.Close()
	if err != nil {
		return fmt.Errorf("winscm: close service: %w", err)
	}

	return nil
}

// Query reads the service state.
func (realSyscalls) Query(service *mgr.Service) (svc.Status, error) {
	state, err := service.Query()
	if err != nil {
		return svc.Status{}, fmt.Errorf("winscm: query service: %w", err)
	}

	return state, nil
}

// Config reads the service configuration.
func (realSyscalls) Config(service *mgr.Service) (mgr.Config, error) {
	config, err := service.Config()
	if err != nil {
		return mgr.Config{}, fmt.Errorf("winscm: read service config: %w", err)
	}

	return config, nil
}

// Start starts the service.
func (realSyscalls) Start(service *mgr.Service) error {
	err := service.Start()
	if err != nil {
		return fmt.Errorf("winscm: start service: %w", err)
	}

	return nil
}

// Control sends a control code to the service.
func (realSyscalls) Control(service *mgr.Service, cmd svc.Cmd) (svc.Status, error) {
	state, err := service.Control(cmd)
	if err != nil {
		return svc.Status{}, fmt.Errorf("winscm: control service: %w", err)
	}

	return state, nil
}

// ExecutableFrom extracts the executable path from a service's registered
// command line, which may be quoted and may carry arguments.
func ExecutableFrom(commandLine string) string {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, `"`) {
		return quotedExecutable(trimmed)
	}

	// Unquoted paths containing spaces are ambiguous by definition. MeshAgent
	// registers a quoted path, so treating the first token as the executable is
	// the sane reading of the remaining cases.
	first, _, found := strings.Cut(trimmed, " ")
	if found {
		return first
	}

	return trimmed
}

// quotedExecutable reads the path out of a quoted command line.
func quotedExecutable(commandLine string) string {
	inner, _, found := strings.Cut(commandLine[1:], `"`)
	if found {
		return inner
	}

	return strings.Trim(commandLine, `"`)
}
