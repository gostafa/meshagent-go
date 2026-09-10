// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentservice

import (
	"time"
)

const (
	// defaultPoll is how often Disconnect re-queries a stopping service.
	defaultPoll = 300 * time.Millisecond

	// StateStopped and StateRunning are the states Controller reasons about.
	// StateStopped is the service state after a successful Disconnect.
	StateStopped = "stopped"
	// StateRunning is the service state while the agent is online.
	StateRunning = "running"

	// errQuery wraps a Status failure from the service control manager.
	errQuery = "agentservice: query %q: %w"

	// errStatus wraps a Status failure including context cancellation.
	errStatus = "agentservice: status: %w"
)
