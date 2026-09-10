package meshagent

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// peHeader is the DOS "MZ" magic every Windows executable starts with.
var peHeader = []byte{'M', 'Z'}

// BinaryName returns the conventional filename for this Client's architecture.
func (c *Client) BinaryName() string {
	switch c.arch {
	case ArchWindows32:
		return "meshagent32.exe"
	case ArchWindowsARM64:
		return "meshagentarm64.exe"
	default:
		return "meshagent64.exe"
	}
}

// Download fetches the agent binary for this Client's device group into destDir
// and returns the path it was written to.
//
// On Windows the server embeds the device group's settings (the .msh) directly
// into the executable, so the downloaded file is self-contained: Install needs
// nothing else.
//
// The file is written to a temporary name and only renamed into place once its
// contents have been validated, so a failed or truncated download never leaves
// a half-written executable behind.
func (c *Client) Download(ctx context.Context, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("meshagent: create destination directory: %w", err)
	}

	query := url.Values{}
	query.Set("id", strconv.Itoa(int(c.arch)))
	query.Set("meshid", c.groupID)

	// MeshCentral omits the InstallFlags line from the .msh entirely when the
	// value is zero, so only send it when it carries meaning.
	if wire := c.installFlags.wire(); wire != 0 {
		query.Set("installflags", strconv.Itoa(wire))
	}

	endpoint := c.endpoint("meshagents", query)

	body, err := c.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	defer body.Close()

	temp, err := os.CreateTemp(destDir, ".meshagent-*.download")
	if err != nil {
		return "", fmt.Errorf("meshagent: create temp file: %w", err)
	}

	tempPath := temp.Name()

	// Clean up unless we successfully rename below.
	defer func() {
		temp.Close()
		os.Remove(tempPath)
	}()

	if err := verifiedCopy(temp, body, tempPath); err != nil {
		return "", err
	}

	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("meshagent: close temp file: %w", err)
	}

	if err := os.Chmod(tempPath, 0o755); err != nil {
		return "", fmt.Errorf("meshagent: chmod downloaded binary: %w", err)
	}

	target := filepath.Join(destDir, c.BinaryName())
	if err := os.Rename(tempPath, target); err != nil {
		return "", fmt.Errorf("meshagent: move downloaded binary into place: %w", err)
	}

	return target, nil
}

// verifiedCopy streams src into dst, rejecting anything that is not a Windows
// executable. MeshCentral can answer with an HTML error page under a 200
// status, so the body is inspected rather than trusted.
func verifiedCopy(dst io.Writer, src io.Reader, path string) error {
	buffered := bufio.NewReader(src)

	magic, err := buffered.Peek(len(peHeader))
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &InvalidBinaryError{Path: path, Reason: "response body was empty"}
		}

		return fmt.Errorf("meshagent: read response body: %w", err)
	}

	if !bytes.Equal(magic, peHeader) {
		return &InvalidBinaryError{
			Path:   path,
			Reason: fmt.Sprintf("expected an MZ header, got %q", magic),
		}
	}

	if _, err := io.Copy(dst, buffered); err != nil {
		return fmt.Errorf("meshagent: write binary: %w", err)
	}

	return nil
}

// Settings holds the key/value pairs of a device group's .msh file.
type Settings map[string]string

// Required reports whether the settings contain everything a temporary session
// needs to reach the server.
func (s Settings) Required() error {
	for _, key := range []string{"MeshID", "ServerID", "MeshServer"} {
		if strings.TrimSpace(s[key]) == "" {
			return fmt.Errorf("meshagent: device group settings are missing %s", key)
		}
	}

	return nil
}

// Settings fetches the device group's .msh settings from the server.
//
// Install does not need this — the Windows binary carries its settings
// embedded. It exists for StartSession, which passes the values to the binary
// as command-line switches instead.
func (c *Client) Settings(ctx context.Context) (Settings, error) {
	query := url.Values{}
	query.Set("id", c.groupID)

	body, err := c.get(ctx, c.endpoint("meshsettings", query))
	if err != nil {
		return nil, err
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("meshagent: read device group settings: %w", err)
	}

	return parseSettings(raw), nil
}

// parseSettings reads the "key=value" lines of a .msh file. Values may
// themselves contain "=" (MeshServer is a URL), so only the first separator
// splits.
func parseSettings(raw []byte) Settings {
	settings := Settings{}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		settings[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return settings
}

// get issues a GET and returns the body on success. The caller closes it.
func (c *Client) get(ctx context.Context, endpoint string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("meshagent: build request: %w", err)
	}

	if c.authCookie != "" {
		req.Header.Set("Cookie", c.authCookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meshagent: request %s: %w", endpoint, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()

		return nil, ErrDownloadUnauthorized
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()

		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        endpoint,
		}
	}

	return resp.Body, nil
}
