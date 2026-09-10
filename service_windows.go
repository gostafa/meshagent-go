//go:build windows

package meshagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// servicePollInterval is how often Disconnect re-queries the service while
// waiting for it to reach the stopped state.
const servicePollInterval = 300 * time.Millisecond

// openService opens the MeshAgent service with exactly the access rights the
// caller needs.
//
// The service control manager is opened with SC_MANAGER_CONNECT rather than
// full access so that read-only operations such as Status work without an
// elevated process. Closing the manager handle does not invalidate the service
// handle it returned.
func (c *Client) openService(access uint32) (*mgr.Service, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("meshagent: open service control manager: %w", err)
	}
	defer windows.CloseServiceHandle(manager)

	name, err := windows.UTF16PtrFromString(c.serviceName)
	if err != nil {
		return nil, fmt.Errorf("meshagent: invalid service name %q: %w", c.serviceName, err)
	}

	handle, err := windows.OpenService(manager, name, access)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil, ErrNotInstalled
		}

		return nil, fmt.Errorf("meshagent: open service %q: %w", c.serviceName, err)
	}

	return &mgr.Service{Name: c.serviceName, Handle: handle}, nil
}

// IsInstalled reports whether the MeshAgent service is registered.
func (c *Client) IsInstalled() (bool, error) {
	service, err := c.openService(windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, ErrNotInstalled) {
			return false, nil
		}

		return false, err
	}
	defer service.Close()

	return true, nil
}

// Status reports the current state of the MeshAgent service.
func (c *Client) Status(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}

	service, err := c.openService(windows.SERVICE_QUERY_STATUS | windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		if errors.Is(err, ErrNotInstalled) {
			return Status{}, nil
		}

		return Status{}, err
	}
	defer service.Close()

	state, err := service.Query()
	if err != nil {
		return Status{}, fmt.Errorf("meshagent: query service state: %w", err)
	}

	status := Status{
		Installed: true,
		Running:   state.State == svc.Running,
		State:     serviceStateName(state.State),
	}

	// Configuration is a nice-to-have; a service we can query but not read the
	// config of should still report its state.
	if config, err := service.Config(); err == nil {
		status.BinaryPath = executableFromCommandLine(config.BinaryPathName)
	}

	return status, nil
}

// Connect starts the MeshAgent service. The device reconnects to MeshCentral on
// its own and reappears online in its device group.
//
// A service that is already running is not an error.
func (c *Client) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	service, err := c.openService(windows.SERVICE_START | windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer service.Close()

	if err := service.Start(); err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
			return nil
		}

		return fmt.Errorf("meshagent: start service %q: %w", c.serviceName, err)
	}

	return nil
}

// Disconnect stops the MeshAgent service and waits for it to reach the stopped
// state or for ctx to be cancelled.
//
// The agent stays installed and its device record stays in the MeshCentral
// device group; the device simply shows as offline. This is the counterpart to
// Connect, and is not the same as stopping a temporary session — see
// StartSession.
//
// A service that is already stopped is not an error.
func (c *Client) Disconnect(ctx context.Context) error {
	service, err := c.openService(windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer service.Close()

	state, err := service.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}

		return fmt.Errorf("meshagent: stop service %q: %w", c.serviceName, err)
	}

	ticker := time.NewTicker(servicePollInterval)
	defer ticker.Stop()

	for state.State != svc.Stopped {
		select {
		case <-ctx.Done():
			return fmt.Errorf("meshagent: waiting for service %q to stop: %w", c.serviceName, ctx.Err())
		case <-ticker.C:
		}

		state, err = service.Query()
		if err != nil {
			return fmt.Errorf("meshagent: query service state: %w", err)
		}
	}

	return nil
}

// serviceStateName renders a Windows service state as a lowercase name.
func serviceStateName(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start pending"
	case svc.StopPending:
		return "stop pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue pending"
	case svc.PausePending:
		return "pause pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown (%d)", int(state))
	}
}

// executableFromCommandLine extracts the executable path from a service's
// registered command line, which may be quoted and may carry arguments.
func executableFromCommandLine(commandLine string) string {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return ""
	}

	if strings.HasPrefix(commandLine, `"`) {
		if end := strings.Index(commandLine[1:], `"`); end >= 0 {
			return commandLine[1 : end+1]
		}

		return strings.Trim(commandLine, `"`)
	}

	// Unquoted paths containing spaces are ambiguous by definition. MeshAgent
	// registers a quoted path, so treating the first token as the executable is
	// the sane reading of the remaining cases.
	if space := strings.IndexByte(commandLine, ' '); space >= 0 {
		return commandLine[:space]
	}

	return commandLine
}
