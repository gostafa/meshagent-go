package meshagent

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedPlatform is returned by the install, service and session
	// operations on anything other than Windows.
	ErrUnsupportedPlatform = errors.New("meshagent: operation is only supported on windows")

	// ErrNotElevated means the current process is not running with an elevated
	// token. Registering or removing the MeshAgent service requires one.
	ErrNotElevated = errors.New("meshagent: operation requires an elevated process")

	// ErrNotInstalled means the MeshAgent service is not registered on this
	// machine.
	ErrNotInstalled = errors.New("meshagent: mesh agent service is not installed")

	// ErrDownloadUnauthorized means the server refused an anonymous agent
	// download. MeshCentral does this when lockagentdownload is enabled for the
	// server or the domain; supply Config.AuthCookie with a logged-in session.
	ErrDownloadUnauthorized = errors.New("meshagent: server requires authentication to download the agent")

	// ErrSessionNotRunning is returned when stopping or waiting on a session
	// that has already exited.
	ErrSessionNotRunning = errors.New("meshagent: session is not running")
)

// HTTPError reports a non-2xx response from the MeshCentral server.
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("meshagent: %s returned %s", e.URL, e.Status)
}

// InvalidBinaryError reports that a downloaded file is not a Windows
// executable. MeshCentral serves HTML error pages with a 200 status in some
// misconfigurations, so the body is checked rather than trusted.
type InvalidBinaryError struct {
	Path   string
	Reason string
}

func (e *InvalidBinaryError) Error() string {
	return fmt.Sprintf("meshagent: %s is not a valid windows executable: %s", e.Path, e.Reason)
}

// ExecError reports a non-zero exit from the MeshAgent binary.
type ExecError struct {
	Args     []string
	ExitCode int
	Output   string
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("meshagent: %v exited with code %d: %s", e.Args, e.ExitCode, e.Output)
}
