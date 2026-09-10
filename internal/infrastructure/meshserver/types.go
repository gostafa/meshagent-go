// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshserver

import (
	"net/http"
	"net/url"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
)

type (
	// Doer performs HTTP requests. It is the seam tests replace to drive the
	// transport failure paths.
	Doer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	// Config describes the server and device group to fetch from.
	Config struct {
		// Doer performs the requests.
		Doer Doer
		// ServerURL is the MeshCentral root. Any path prefix is preserved, so
		// non-default MeshCentral domains keep working.
		ServerURL *url.URL
		// GroupID is the device group identifier ("meshid").
		GroupID string
		// AuthCookie is sent as the Cookie header. It is only needed when the
		// server enables lockagentdownload.
		AuthCookie string
		// InstallFlags controls which operations the installed agent offers.
		InstallFlags artifact.InstallFlags
		// Arch selects which agent binary to download.
		Arch artifact.Arch
	}

	// Client fetches from a MeshCentral server.
	//
	// Its work is split across two focused collaborators rather than held as
	// one wide field set, so that every field a type owns is used by every
	// method that type has.
	Client struct {
		transport transport
		locator   locator
	}

	// transport performs authenticated requests.
	transport struct {
		doer       Doer
		authCookie string
	}

	// locator builds the endpoint URLs for one device group.
	locator struct {
		serverURL    *url.URL
		groupID      string
		installFlags artifact.InstallFlags
		arch         artifact.Arch
	}

	// HTTPError reports a non-2xx response.
	HTTPError struct {
		Status string
		URL    string
		Code   int
	}

	// InvalidBinaryError reports that a downloaded file is not a Windows
	// executable. MeshCentral serves HTML error pages under a 200 status in
	// some misconfigurations, so the body is checked rather than trusted.
	InvalidBinaryError struct {
		Reason string
	}
)
