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
		Close(service *mgr.Service) error
		Control(service *mgr.Service, cmd svc.Cmd) error
		Open(name string, access uint32) (*mgr.Service, error)
		Snapshot(service *mgr.Service) (svc.Status, string, error)
		Start(service *mgr.Service) error
	}

	// Ops answers the agentservice.Ops and agentinstall.Locator seams.
	Ops struct {
		calls Syscalls
		name  string
	}

	// kernel talks to the service control manager through injectable functions.
	kernel struct {
		openSCManager      func(machineName *uint16, databaseName *uint16, access uint32) (windows.Handle, error)
		closeServiceHandle func(handle windows.Handle) error
		utf16FromString    func(name string) (*uint16, error)
		openWinService     func(manager windows.Handle, name *uint16, access uint32) (windows.Handle, error)
		closeService       func(service *mgr.Service) error
		queryService       func(service *mgr.Service) (svc.Status, error)
		configService      func(service *mgr.Service) (mgr.Config, error)
		startService       func(service *mgr.Service, args ...string) error
		controlService     func(service *mgr.Service, cmd svc.Cmd) (svc.Status, error)
	}
)

const (
	emptyPath            = ""
	errInstalled         = "winscm: installed: %w"
	errOpen              = "winscm: open: %w"
	errStatus            = "winscm: status: %w"
	quoteMark            = `"`
	spaceSep             = " "
	stateContinuePending = "continue pending"
	statePausePending    = "pause pending"
	statePaused          = "paused"
	stateRunning         = "running"
	stateStartPending    = "start pending"
	stateStopPending     = "stop pending"
	stateStopped         = "stopped"
	stateUnknown         = "unknown"
)

// New returns an Ops for the named service backed by the real service control
// manager.
func New(name string) *Ops {
	return NewWith(liveKernel(), name)
}

// NewWith returns an Ops backed by the given seam. It exists so tests can
// drive the error mapping.
func NewWith(calls Syscalls, name string) *Ops {
	return &Ops{calls: calls, name: name}
}

// ExecutableFrom extracts the executable path from a service's registered
// command line, which may be quoted and may carry arguments.
func ExecutableFrom(commandLine string) string {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == emptyPath {
		return emptyPath
	}

	if strings.HasPrefix(trimmed, quoteMark) {
		return quotedExecutable(trimmed)
	}

	return unquotedExecutable(trimmed)
}

// absentOrError maps a failed open onto a nil error when the service is absent.
func absentOrError(err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}

	return fmt.Errorf("winscm: open service: %w", err)
}

// describe assembles the reported status from a snapshot.
func describe(state svc.State, path string) lifecycle.Status {
	return lifecycle.Status{
		Installed:  true,
		Running:    state == svc.Running,
		State:      stateName(state),
		BinaryPath: path,
	}
}

// emptyStatus is the report for a service that is not registered.
func emptyStatus() lifecycle.Status {
	return lifecycle.Status{
		State:      emptyPath,
		BinaryPath: emptyPath,
		Installed:  false,
		Running:    false,
	}
}

// firstToken drops trailing command-line arguments from a cut executable path.
func firstToken(inner, tail string) string {
	return strings.TrimSuffix(inner+tail, tail)
}

// liveKernel wires the Windows service control manager entry points.
func liveKernel() *kernel {
	return &kernel{
		openSCManager:      windows.OpenSCManager,
		closeServiceHandle: windows.CloseServiceHandle,
		utf16FromString:    windows.UTF16PtrFromString,
		openWinService:     windows.OpenService,
		closeService:       (*mgr.Service).Close,
		queryService:       (*mgr.Service).Query,
		configService:      (*mgr.Service).Config,
		startService:       (*mgr.Service).Start,
		controlService:     (*mgr.Service).Control,
	}
}

// notInstalledOrError maps a failed open onto the module's sentinel.
func notInstalledOrError(err error, name string) error {
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("winscm: %q: %w", name, agenterr.ErrNotInstalled)
	}

	return fmt.Errorf("winscm: open %q: %w", name, err)
}

// quotedExecutable reads the path out of a quoted command line.
func quotedExecutable(commandLine string) string {
	rest := strings.TrimPrefix(commandLine, quoteMark)

	inner, tail, found := strings.Cut(rest, quoteMark)
	if found {
		return firstToken(inner, tail)
	}

	return inner
}

// stateName renders a Windows service state as a lowercase name.
func stateName(state svc.State) string {
	names := map[svc.State]string{
		svc.Stopped:         stateStopped,
		svc.StartPending:    stateStartPending,
		svc.StopPending:     stateStopPending,
		svc.Running:         stateRunning,
		svc.ContinuePending: stateContinuePending,
		svc.PausePending:    statePausePending,
		svc.Paused:          statePaused,
	}

	name, known := names[state]
	if !known {
		return stateUnknown
	}

	return name
}

// unquotedExecutable reads the first token of an unquoted command line.
func unquotedExecutable(commandLine string) string {
	inner, tail, found := strings.Cut(commandLine, spaceSep)
	if found {
		return firstToken(inner, tail)
	}

	return inner
}

// Installed answers the agentinstall.Locator seam.
func (ops *Ops) Installed(ctx context.Context) (lifecycle.Status, error) {
	err := ctx.Err()
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf(errInstalled, err)
	}

	status, statusErr := ops.Status(ops.name)
	if statusErr != nil {
		return lifecycle.Status{}, fmt.Errorf(errInstalled, statusErr)
	}

	return status, nil
}

// Name reports the service this Ops drives.
func (ops *Ops) Name() string {
	return ops.name
}

// Start starts the service. An already-running service is not an error.
func (ops *Ops) Start(name string) error {
	service, err := ops.calls.Open(name, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return fmt.Errorf("winscm: start: %w", notInstalledOrError(err, name))
	}

	defer ops.discard(service)

	err = ops.calls.Start(service)
	if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("winscm: start %q: %w", name, err)
	}

	return nil
}

// Status reports the current state of the service.
//
// A service that is not registered is reported as a zero Status rather than an
// error, so callers can tell "absent" from "unreadable".
func (ops *Ops) Status(name string) (lifecycle.Status, error) {
	service, err := ops.calls.Open(name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)

	mapped := absentOrError(err)
	if mapped != nil {
		return lifecycle.Status{}, fmt.Errorf(errStatus, mapped)
	}

	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return emptyStatus(), nil
	}

	status, loadErr := ops.loadedStatus(service, name)
	if loadErr != nil {
		return lifecycle.Status{}, fmt.Errorf(errStatus, loadErr)
	}

	return status, nil
}

// Stop asks the service to stop. An already-stopped service is not an error.
func (ops *Ops) Stop(name string) error {
	service, err := ops.calls.Open(name, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return fmt.Errorf("winscm: stop: %w", notInstalledOrError(err, name))
	}

	defer ops.discard(service)

	err = ops.calls.Control(service, svc.Stop)
	if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return nil
	}

	if err != nil {
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

// loadedStatus snapshots an opened service and always closes it.
func (ops *Ops) loadedStatus(service *mgr.Service, name string) (lifecycle.Status, error) {
	defer ops.discard(service)

	state, path, err := ops.calls.Snapshot(service)
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf("winscm: query %q: %w", name, err)
	}

	return describe(state.State, path), nil
}

// Close releases a service handle.
func (api *kernel) Close(service *mgr.Service) error {
	err := api.closeService(service)
	if err != nil {
		return fmt.Errorf("winscm: close service: %w", err)
	}

	return nil
}

// Control sends a control code to the service.
func (api *kernel) Control(service *mgr.Service, cmd svc.Cmd) error {
	status, err := api.controlService(service, cmd)
	if err != nil {
		return fmt.Errorf("winscm: control service: %w", err)
	}

	label := stateName(status.State)

	return map[string]error{label: nil}[label]
}

// Open opens the service with exactly the rights the caller needs.
func (api *kernel) Open(name string, access uint32) (service *mgr.Service, err error) {
	manager, err := api.connectManager()
	if err != nil {
		return nil, fmt.Errorf(errOpen, err)
	}

	defer func() {
		service, err = api.afterOpen(*manager, service, err)
	}()

	service, err = api.openService(*manager, name, access)
	if err != nil {
		return nil, fmt.Errorf(errOpen, err)
	}

	return service, nil
}

// Snapshot reads the service state and executable path.
func (api *kernel) Snapshot(service *mgr.Service) (svc.Status, string, error) {
	state, err := api.queryService(service)
	if err != nil {
		return svc.Status{}, emptyPath, fmt.Errorf("winscm: query service: %w", err)
	}

	return state, api.binaryPath(service), nil
}

// Start starts the service.
func (api *kernel) Start(service *mgr.Service) error {
	err := api.startService(service)
	if err != nil {
		return fmt.Errorf("winscm: start service: %w", err)
	}

	return nil
}

// abandon closes a service handle opened before the manager handle failed.
func (api *kernel) abandon(service *mgr.Service) {
	closeErr := api.Close(service)
	if closeErr != nil {
		return
	}
}

// afterOpen closes the manager handle and keeps the first useful error.
func (api *kernel) afterOpen(
	manager windows.Handle,
	service *mgr.Service,
	result error,
) (*mgr.Service, error) {
	closeErr := api.closeServiceHandle(manager)
	if closeErr == nil && result == nil {
		return service, nil
	}

	if result != nil {
		return nil, fmt.Errorf(errOpen, result)
	}

	api.abandon(service)

	return nil, fmt.Errorf("winscm: close service control manager: %w", closeErr)
}

// binaryPath reads the registered executable, treating config failure as empty.
func (api *kernel) binaryPath(service *mgr.Service) string {
	config, err := api.configService(service)
	if err != nil {
		return emptyPath
	}

	return ExecutableFrom(config.BinaryPathName)
}

// connectManager opens the service control manager with connect rights.
func (api *kernel) connectManager() (*windows.Handle, error) {
	manager, err := api.openSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("winscm: open service control manager: %w", err)
	}

	return &manager, nil
}

// openService opens the named service on an already-connected manager.
func (api *kernel) openService(
	manager windows.Handle,
	name string,
	access uint32,
) (*mgr.Service, error) {
	wide, err := api.utf16FromString(name)
	if err != nil {
		return nil, fmt.Errorf("winscm: invalid service name %q: %w", name, err)
	}

	handle, err := api.openWinService(manager, wide, access)
	if err != nil {
		return nil, fmt.Errorf("winscm: open service %q: %w", name, err)
	}

	return &mgr.Service{Name: name, Handle: handle}, nil
}
