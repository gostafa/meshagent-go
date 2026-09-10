package meshagent

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultServiceName is the Windows service name MeshAgent registers unless the
// device group's settings override it via the meshServiceName key.
const DefaultServiceName = "Mesh Agent"

const defaultDownloadTimeout = 5 * time.Minute

// Config configures a Client. Only ServerURL and GroupID are required.
type Config struct {
	// ServerURL is the MeshCentral server root, for example
	// "https://mesh.example.com". A path component is preserved, which is how
	// non-default MeshCentral domains are addressed.
	ServerURL string

	// GroupID is the device group identifier ("meshid"), as it appears in the
	// gotomesh query parameter of the group's web URL.
	GroupID string

	// Arch selects which agent binary to download. Defaults to DetectArch().
	Arch Arch

	// InstallFlags controls which operations the installed agent offers its
	// user. Defaults to FlagBackgroundOnly.
	InstallFlags InstallFlags

	// ServiceName is the Windows service to control. Defaults to
	// DefaultServiceName. Override it when the device group customises
	// meshServiceName, otherwise service control will not find the agent.
	ServiceName string

	// HTTPClient is used for downloads. Defaults to a client with a five
	// minute timeout.
	HTTPClient *http.Client

	// AuthCookie is sent as the request Cookie header when downloading. It is
	// only needed when the server or domain enables lockagentdownload, which
	// makes anonymous agent downloads return 401.
	AuthCookie string
}

// Client downloads and manages the MeshCentral agent.
type Client struct {
	serverURL    *url.URL
	groupID      string
	arch         Arch
	installFlags InstallFlags
	serviceName  string
	httpClient   *http.Client
	authCookie   string
}

// New validates cfg, applies defaults and returns a Client.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.ServerURL) == "" {
		return nil, errors.New("meshagent: ServerURL is required")
	}

	if strings.TrimSpace(cfg.GroupID) == "" {
		return nil, errors.New("meshagent: GroupID is required")
	}

	parsed, err := url.Parse(strings.TrimRight(cfg.ServerURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("meshagent: parse ServerURL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("meshagent: ServerURL scheme must be http or https, got %q", parsed.Scheme)
	}

	if parsed.Host == "" {
		return nil, errors.New("meshagent: ServerURL is missing a host")
	}

	arch := cfg.Arch
	if arch == 0 {
		arch = DetectArch()
	}

	if !arch.Valid() {
		return nil, fmt.Errorf("meshagent: unsupported architecture %d", int(arch))
	}

	flags := cfg.InstallFlags
	if flags == FlagUnset {
		flags = FlagBackgroundOnly
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = DefaultServiceName
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultDownloadTimeout}
	}

	return &Client{
		serverURL:    parsed,
		groupID:      cfg.GroupID,
		arch:         arch,
		installFlags: flags,
		serviceName:  serviceName,
		httpClient:   httpClient,
		authCookie:   cfg.AuthCookie,
	}, nil
}

// ServiceName returns the Windows service name this Client controls.
func (c *Client) ServiceName() string { return c.serviceName }

// Arch returns the agent architecture this Client downloads.
func (c *Client) Arch() Arch { return c.arch }

// endpoint builds an absolute URL for a MeshCentral endpoint, preserving any
// path prefix in ServerURL so custom domains keep working.
func (c *Client) endpoint(name string, query url.Values) string {
	u := *c.serverURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + name
	u.RawQuery = query.Encode()

	return u.String()
}
