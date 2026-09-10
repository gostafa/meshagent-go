// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshserver

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

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
	"github.com/gostafa/meshagent-go/internal/shared/meshpath"
)

type (
	// chmodDownloadedError carries the chmod used by promote as an error value.
	//
	// Package-level state is otherwise refused, and err-prefixed variables are
	// the documented exception, so the function travels as one.
	chmodDownloadedError func(name string, mode os.FileMode) error

	// createTempError carries the CreateTemp used by stage as an error value.
	createTempError func(dir string, pattern string) (*os.File, error)

	// renameDownloadedError carries the rename used by promote as an error value.
	renameDownloadedError func(oldpath string, newpath string) error
)

var (
	// errChmodDownloaded holds the chmod used by promote. Tests replace it so
	// the chmod error path runs on every GOOS.
	errChmodDownloaded error = chmodDownloadedError(os.Chmod)
	// errCreateTemp holds the CreateTemp used by stage. Tests replace it so the
	// create-temp error path runs on every GOOS.
	errCreateTemp error = createTempError(os.CreateTemp)
	// errRenameDownloaded holds the rename used by promote. Tests replace it so
	// the rename error path runs on every GOOS.
	errRenameDownloaded error = renameDownloadedError(os.Rename)
)

// New returns a Client for cfg.
func New(cfg *Config) *Client {
	return cfg.client()
}

// client assembles the collaborators a Client is made of.
func (cfg *Config) client() *Client {
	return &Client{
		transport: transport{
			doer:       cfg.Doer,
			authCookie: cfg.AuthCookie,
		},
		locator: locator{
			serverURL:    cfg.ServerURL,
			groupID:      cfg.GroupID,
			arch:         cfg.Arch,
			installFlags: cfg.InstallFlags,
		},
	}
}

// Download writes the agent for this device group into destDir and returns its
// path.
//
// On Windows the server embeds the device group settings into the executable,
// so the result is self-contained. The file is written under a temporary name
// and only renamed once validated, so a failed download leaves nothing behind.
func (client *Client) Download(ctx context.Context, destDir string) (string, error) {
	temp, err := stage(destDir)
	if err != nil {
		return emptyPath, fmt.Errorf("meshserver: stage download: %w", err)
	}

	target := filepath.Join(destDir, client.locator.binaryName())

	err = client.saveAgent(ctx, temp, target)
	if err != nil {
		return emptyPath, errors.Join(err, discardTemp(temp))
	}

	return target, nil
}

// stage prepares the destination directory and the partial download file.
func stage(destDir string) (*os.File, error) {
	err := os.MkdirAll(destDir, dirPerm)
	if err != nil {
		return nil, fmt.Errorf("meshserver: create destination directory: %w", err)
	}

	temp, err := createTempFrom(errCreateTemp)(destDir, tempPattern)
	if err != nil {
		return nil, fmt.Errorf("meshserver: create temp file: %w", err)
	}

	return temp, nil
}

// saveAgent streams the agent into temp and moves it onto target.
func (client *Client) saveAgent(ctx context.Context, temp *os.File, target string) error {
	err := client.get(ctx, client.locator.agents(), func(body io.Reader) error {
		return verifiedCopy(temp, body)
	})
	if err != nil {
		return fmt.Errorf("meshserver: fetch agent: %w", err)
	}

	err = promote(temp, target)
	if err != nil {
		return fmt.Errorf("meshserver: place agent: %w", err)
	}

	return nil
}

// Settings returns the device group's .msh settings.
//
// Install does not need them; running a temporary session does.
func (client *Client) Settings(ctx context.Context) (artifact.Settings, error) {
	var settings artifact.Settings

	err := client.get(ctx, client.locator.settings(), func(body io.Reader) error {
		raw, readErr := io.ReadAll(io.LimitReader(body, settingsLimit))

		settings = artifact.ParseSettings(raw)

		if readErr != nil {
			return fmt.Errorf("meshserver: read device group settings: %w", readErr)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("meshserver: fetch device group settings: %w", err)
	}

	return settings, nil
}

// agentsEndpoint builds the URL of the agent binary for this device group.
func (loc *locator) agents() string {
	query := url.Values{}
	query.Set(queryID, strconv.Itoa(int(loc.arch)))
	query.Set(queryGroup, loc.groupID)

	// MeshCentral omits the InstallFlags line from the .msh entirely when the
	// value is zero, so only send it when it carries meaning.
	wire := loc.installFlags.Wire()
	if wire != noInstallFlags {
		query.Set(queryFlags, strconv.Itoa(wire))
	}

	return meshpath.Endpoint(loc.serverURL, meshpath.AgentsEndpoint, query)
}

// binaryName is the filename the downloaded agent takes on disk.
func (loc *locator) binaryName() string {
	return loc.arch.BinaryName()
}

// settings builds the URL of this device group's .msh file.
func (loc *locator) settings() string {
	query := url.Values{}
	query.Set(queryID, loc.groupID)

	return meshpath.Endpoint(loc.serverURL, meshpath.SettingsEndpoint, query)
}

// get performs a request and hands the body to consume.
//
// The body is drained and closed here rather than by the caller, so every exit
// path both consumes and closes it exactly once.
func (client *Client) get(
	ctx context.Context,
	endpoint string,
	consume func(body io.Reader) error,
) (err error) {
	resp, err := client.transport.send(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("meshserver: send request: %w", err)
	}

	defer func() { err = errors.Join(err, drainClose(resp.Body)) }()

	err = readBody(resp, endpoint, consume)
	if err != nil {
		return fmt.Errorf("meshserver: read response: %w", err)
	}

	return nil
}

// readBody checks the status and hands a good body to consume.
func readBody(resp *http.Response, endpoint string, consume func(body io.Reader) error) error {
	err := checkStatus(resp, endpoint)
	if err != nil {
		return fmt.Errorf("meshserver: check response: %w", err)
	}

	err = consume(resp.Body)
	if err != nil {
		return fmt.Errorf("meshserver: consume response body: %w", err)
	}

	return nil
}

// send builds and performs the request, keeping get within its statement
// budget.
func (tpt *transport) send(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("meshserver: build request: %w", err)
	}

	if tpt.authCookie != emptyCookie {
		req.Header.Set(cookieHeader, tpt.authCookie)
	}

	resp, err := tpt.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meshserver: request %s: %w", endpoint, err)
	}

	return resp, nil
}

// checkStatus turns a refusal into a typed error.
func checkStatus(resp *http.Response, endpoint string) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("meshserver: %s: %w", endpoint, agenterr.ErrDownloadUnauthorized)
	}

	if resp.StatusCode < statusMin || resp.StatusCode > statusMax {
		return &HTTPError{Code: resp.StatusCode, Status: resp.Status, URL: endpoint}
	}

	return nil
}

// verifiedCopy streams src into dst, rejecting anything that is not a Windows
// executable.
func verifiedCopy(dst io.Writer, src io.Reader) error {
	buffered := bufio.NewReader(src)

	magic, err := buffered.Peek(len(peHeader))
	if err != nil {
		return fmt.Errorf("meshserver: inspect binary: %w", invalidBinary(err, magic))
	}

	if !bytes.Equal(magic, []byte(peHeader)) {
		return &InvalidBinaryError{Reason: fmt.Sprintf("expected an MZ header, got %q", magic)}
	}

	written, err := io.Copy(dst, buffered)
	if err != nil {
		return fmt.Errorf("meshserver: write binary after %d bytes: %w", written, err)
	}

	return nil
}

// invalidBinary explains why the leading bytes could not be read.
func invalidBinary(err error, magic []byte) error {
	if errors.Is(err, io.EOF) {
		return &InvalidBinaryError{Reason: fmt.Sprintf("response body held %d bytes", len(magic))}
	}

	return fmt.Errorf("meshserver: read response body: %w", err)
}

// chmodFrom unwraps the chmod carried by err, falling back to os.Chmod.
func chmodFrom(err error) chmodDownloadedError {
	chmod, ok := errors.AsType[chmodDownloadedError](err)
	if !ok {
		return os.Chmod
	}

	return chmod
}

// createTempFrom unwraps the CreateTemp carried by err, falling back to
// os.CreateTemp.
func createTempFrom(err error) createTempError {
	createTemp, ok := errors.AsType[createTempError](err)
	if !ok {
		return os.CreateTemp
	}

	return createTemp
}

// renameFrom unwraps the rename carried by err, falling back to os.Rename.
func renameFrom(err error) renameDownloadedError {
	rename, ok := errors.AsType[renameDownloadedError](err)
	if !ok {
		return os.Rename
	}

	return rename
}

// promote closes temp, marks it executable and moves it onto target.
func promote(temp *os.File, target string) error {
	err := temp.Close()
	if err != nil {
		return fmt.Errorf(errCloseTemp, err)
	}

	// #nosec G302 -- the agent is an executable and must carry the execute bit;
	// 0700 keeps it owner-only, which is the tightest workable mode.
	err = chmodFrom(errChmodDownloaded)(temp.Name(), exePerm)
	if err != nil {
		return fmt.Errorf("meshserver: chmod downloaded binary: %w", err)
	}

	err = renameFrom(errRenameDownloaded)(temp.Name(), target)
	if err != nil {
		return fmt.Errorf("meshserver: move downloaded binary into place: %w", err)
	}

	return nil
}

// discardTemp removes a partial download. The file may already be closed or
// gone depending on how far the download got, and neither is a failure.
func discardTemp(temp *os.File) error {
	return errors.Join(
		ignoreClosed(temp.Close()),
		ignoreMissing(os.Remove(temp.Name())),
	)
}

// ignoreClosed drops an "already closed" error and wraps anything else.
func ignoreClosed(err error) error {
	if err == nil || errors.Is(err, os.ErrClosed) {
		return nil
	}

	return fmt.Errorf(errCloseTemp, err)
}

// ignoreMissing drops an "already gone" error and wraps anything else.
func ignoreMissing(err error) error {
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("meshserver: remove temp file: %w", err)
}

// drainClose consumes the remainder of a body and closes it, which is what the
// response-lifecycle check requires.
func drainClose(body io.ReadCloser) error {
	discarded, copyErr := io.Copy(io.Discard, body)
	closeErr := body.Close()

	if copyErr != nil {
		return fmt.Errorf("meshserver: drain response body after %d bytes: %w", discarded, copyErr)
	}

	if closeErr != nil {
		return fmt.Errorf("meshserver: close response body: %w", closeErr)
	}

	return nil
}

// Error implements the error interface for the chmod carrier.
func (chmodDownloadedError) Error() string {
	return "meshserver: chmod downloaded"
}

// Unwrap terminates the error chain.
func (chmodDownloadedError) Unwrap() error {
	return nil
}

// Error implements the error interface for the CreateTemp carrier.
func (createTempError) Error() string {
	return "meshserver: create temp"
}

// Unwrap terminates the error chain.
func (createTempError) Unwrap() error {
	return nil
}

// Error implements the error interface for the rename carrier.
func (renameDownloadedError) Error() string {
	return "meshserver: rename downloaded"
}

// Unwrap terminates the error chain.
func (renameDownloadedError) Unwrap() error {
	return nil
}
