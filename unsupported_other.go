//go:build !windows

package meshagent

import "context"

// This file provides the non-Windows half of the API so the package builds,
// vets and tests on any host. Downloading and reading device group settings are
// genuinely cross-platform and live in download.go; everything that touches the
// Windows service manager or spawns the agent is stubbed here.

// IsElevated always reports false off Windows, where the concept does not
// apply to this package's operations.
func IsElevated() bool { return false }

// Install is not supported on this platform.
func (c *Client) Install(_ context.Context, _ string) error { return ErrUnsupportedPlatform }

// Uninstall is not supported on this platform.
func (c *Client) Uninstall(_ context.Context) error { return ErrUnsupportedPlatform }

// IsInstalled is not supported on this platform.
func (c *Client) IsInstalled() (bool, error) { return false, ErrUnsupportedPlatform }

// Status is not supported on this platform.
func (c *Client) Status(_ context.Context) (Status, error) { return Status{}, ErrUnsupportedPlatform }

// Connect is not supported on this platform.
func (c *Client) Connect(_ context.Context) error { return ErrUnsupportedPlatform }

// Disconnect is not supported on this platform.
func (c *Client) Disconnect(_ context.Context) error { return ErrUnsupportedPlatform }

// Session is the non-Windows placeholder for a temporary agent connection. It
// is never constructed off Windows.
type Session struct{}

// Pid returns zero.
func (s *Session) Pid() int { return 0 }

// Done returns a nil channel, which blocks forever.
func (s *Session) Done() <-chan struct{} { return nil }

// Wait is not supported on this platform.
func (s *Session) Wait() error { return ErrUnsupportedPlatform }

// Stop is not supported on this platform.
func (s *Session) Stop() error { return ErrUnsupportedPlatform }

// StartSession is not supported on this platform.
func (c *Client) StartSession(_ context.Context, _ string) (*Session, error) {
	return nil, ErrUnsupportedPlatform
}
