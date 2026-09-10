// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshserver

const (
	// queryID names the resource on both endpoints: the architecture on the
	// agents endpoint, the device group on the settings endpoint.
	queryID = "id"
	// queryGroup names the device group on the agents endpoint.
	queryGroup = "meshid"
	// queryFlags carries the install flags on the agents endpoint.
	queryFlags = "installflags"

	// cookieHeader carries the session for servers with lockagentdownload on.
	cookieHeader = "Cookie"

	// peHeader is the DOS magic every Windows executable starts with.
	peHeader = "MZ"

	// tempPattern names the partial download before it is validated.
	tempPattern = ".meshagent-*.download"

	// dirPerm and exePerm are the permissions of the destination directory and
	// of the downloaded agent.
	dirPerm = 0o750
	exePerm = 0o700

	// settingsLimit caps the .msh response, which is a few hundred bytes.
	settingsLimit = 1 << 20

	// statusMin and statusMax bound a successful response.
	statusMin = 200
	statusMax = 299

	// noInstallFlags is the wire value MeshCentral treats as "unset" and omits
	// from the .msh entirely.
	noInstallFlags = 0

	// emptyCookie is the absent session cookie.
	emptyCookie = ""

	// errCloseTemp is shared by the two paths that close the staging file.
	errCloseTemp = "meshserver: close temp file: %w"

	// emptyPath is the path returned alongside a download failure.
	emptyPath = ""
)
