// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package agentservice starts, stops and inspects the installed agent service.
//
// It is the outbound adapter behind the lifecycle.Controller port. The
// orchestration — waiting for a stop, mapping "already running" and "already
// stopped" onto success — is portable and lives in Controller; only the service
// control manager calls behind the Ops seam are Windows-specific.
package agentservice
