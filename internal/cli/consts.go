// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"time"
)

const (
	// ExitOK is the process exit code for success.
	ExitOK = 0
	// ExitError is the process exit code for an operational failure.
	ExitError = 1
	// ExitUsage is the process exit code for a command-line problem.
	ExitUsage = 2
	// ExitDenied is the process exit code for a refusal (elevation or auth).
	ExitDenied = 3

	// EnvServer is the environment fallback for -server.
	EnvServer = "MESHAGENT_SERVER"
	// EnvGroup is the environment fallback for -group.
	EnvGroup = "MESHAGENT_GROUP"
	// EnvCookie is the environment fallback for the session cookie, so the
	// cookie need not appear in a command line that other users can read from
	// the process table.
	EnvCookie = "MESHAGENT_COOKIE"

	cmdConnect    = "connect"
	cmdDisconnect = "disconnect"
	cmdStatus     = "status"
	cmdUninstall  = "uninstall"
	cmdHelp       = "help"
	flagHelpShort = "-h"
	flagHelpLong  = "--help"

	defaultTimeout = 5 * time.Minute
	stagingPattern = "meshagent-*"
	emptyValue     = ""

	placeholderServer = "https://localhost"
	placeholderGroup  = "local"

	// archDetect is the unset -arch value, meaning "detect".
	archDetect = -1

	keyCommand   = "command"
	keyService   = "service"
	keyError     = "err"
	keyFrom      = "from"
	keyFix       = "fix"
	keySupply    = "supply"
	keyInstalled = "installed"
	keyState     = "state"
	keyPath      = "path"

	msgUnknownCommand = "unknown command"
	msgNeedElevation  = "this command needs an elevated process; use an Administrator prompt"
	msgUnauthorized   = "the server refused an anonymous download"
	msgNotInstalled   = "the mesh agent is not installed"
	msgUnsupported    = "this command is only supported on windows"
	msgCanceled       = "canceled"
	msgFailed         = "command failed"
	msgBadCommandLine = "invalid command line"
	msgDownloading    = "downloading agent"
	msgInstalling     = "installing"
	msgConnected      = "connected"
	msgDisconnected   = "disconnected; the device stays registered"
	msgUninstalled    = "uninstalled; the device record is left in the device group"
	msgStatus         = "agent status"

	fixConnect = "meshagentctl connect"

	errReadStatus          = "cli: read status: %w"
	errFmtRequire          = "cli: require: %w"
	errFmtEnsureInstalled  = "cli: ensure installed: %w"
	errFmtDownloadRegister = "cli: download and register: %w"
	errFmtDisconnect       = "cli: disconnect: %w"
	errFmtUninstall        = "cli: uninstall: %w"
	errFmtAgent            = "cli: agent: %w"
	errFmtInstaller        = "cli: installer: %w"
	errFmtEncode           = "cli: encode status: %w"
	errFmtApplyLocal       = "cli: apply local: %w"

	usageText = `meshagentctl manages the MeshCentral agent on this machine.

Usage:
  meshagentctl <command> [flags]

Commands:
  connect      Install the agent if needed, then start it. Safe to re-run.
  disconnect   Stop the agent. It stays installed and the device stays
               registered in its device group, showing as offline.
  status       Report whether the agent is installed and running.
  uninstall    Stop the agent, remove the service and delete its files.

Environment:
  MESHAGENT_SERVER   default for -server
  MESHAGENT_GROUP    default for -group
  MESHAGENT_COOKIE   session cookie for servers with lockagentdownload enabled

Run "meshagentctl <command> -h" for the flags of a command.
`
)
