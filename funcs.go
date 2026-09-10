// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshagent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/infrastructure/agentinstall"
	"github.com/gostafa/meshagent-go/internal/infrastructure/agentports"
	"github.com/gostafa/meshagent-go/internal/infrastructure/agentservice"
	"github.com/gostafa/meshagent-go/internal/infrastructure/meshserver"
	"github.com/gostafa/meshagent-go/internal/infrastructure/winscm"
)

// DetectArch returns the Windows agent architecture matching the architecture
// this binary was compiled for.
func DetectArch() Arch {
	return artifact.DetectArch()
}

// New validates cfg, applies defaults and wires a Client.
func New(cfg *Config) (*Client, error) {
	serverURL, err := parseConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("meshagent: configure: %w", err)
	}

	return newClient(cfg, serverURL), nil
}

// newClient assembles the adapters behind the ports.
func newClient(cfg *Config, serverURL *url.URL) *Client {
	name := resolveServiceName(cfg.ServiceName)
	scm := winscm.New(name)
	server := meshserver.New(&meshserver.Config{
		ServerURL:    serverURL,
		Doer:         resolveDoer(cfg.HTTPClient),
		GroupID:      cfg.GroupID,
		AuthCookie:   cfg.AuthCookie,
		Arch:         resolveArch(cfg.Arch),
		InstallFlags: resolveFlags(cfg.InstallFlags),
	})

	return &Client{ports: &agentports.Ports{
		Downloader:  server,
		Settings:    server,
		Installer:   agentinstall.New(scm),
		Controller:  agentservice.New(scm, name),
		ServiceName: name,
	}}
}

// parseConfig validates the configured server URL.
func parseConfig(cfg *Config) (*url.URL, error) {
	err := requireConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("meshagent: require: %w", err)
	}

	parsed, err := parseServerURL(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("meshagent: server url: %w", err)
	}

	return parsed, nil
}

// requireConfig reports absent or invalid configuration fields.
func requireConfig(cfg *Config) error {
	if strings.TrimSpace(cfg.ServerURL) == emptyValue {
		return errServerURLRequired
	}

	if strings.TrimSpace(cfg.GroupID) == emptyValue {
		return errGroupIDRequired
	}

	err := validateConfig(cfg)
	if err != nil {
		return fmt.Errorf("meshagent: validate: %w", err)
	}

	return nil
}

// validateConfig checks the enumerated fields.
func validateConfig(cfg *Config) error {
	if !resolveArch(cfg.Arch).Valid() {
		return fmt.Errorf("%w: %d", errBadArch, cfg.Arch)
	}

	if !resolveFlags(cfg.InstallFlags).Valid() {
		return fmt.Errorf("%w: %q", errBadInstallFlags, cfg.InstallFlags)
	}

	return nil
}

// parseServerURL turns the configured root into a URL.
func parseServerURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(raw, pathSeparator))
	if err != nil {
		return nil, fmt.Errorf("meshagent: parse ServerURL: %w", err)
	}

	checked, err := checkServerURL(parsed)
	if err != nil {
		return nil, fmt.Errorf("meshagent: check url: %w", err)
	}

	return checked, nil
}

// checkServerURL reports a URL that cannot address a MeshCentral server.
func checkServerURL(parsed *url.URL) (*url.URL, error) {
	if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
		return nil, fmt.Errorf("%w, got %q", errBadScheme, parsed.Scheme)
	}

	if parsed.Host == emptyValue {
		return nil, errMissingHost
	}

	return parsed, nil
}

// resolveArch resolves the configured architecture.
func resolveArch(value int) Arch {
	if value == int(unsetArch) {
		return artifact.DetectArch()
	}

	return Arch(value)
}

// resolveFlags resolves the configured install flags.
func resolveFlags(value string) InstallFlags {
	if value == string(artifact.FlagUnset) {
		return artifact.FlagBackgroundOnly
	}

	return InstallFlags(value)
}

// resolveServiceName resolves the configured service name.
func resolveServiceName(name string) string {
	if name == emptyValue {
		return DefaultServiceName
	}

	return name
}

// resolveDoer resolves the HTTP client used for downloads.
func resolveDoer(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Timeout: defaultTimeout}
	}

	return client
}

// ServiceName returns the Windows service this Client controls.
func (cli *Client) ServiceName() string {
	return serviceNameOf(cli.ports)
}

// Download fetches the agent binary for this device group into destDir and
// returns the path it was written to.
func (cli *Client) Download(ctx context.Context, destDir string) (string, error) {
	path, err := downloadOf(ctx, cli.ports, destDir)
	if err != nil {
		return emptyValue, fmt.Errorf(errFmtDownload, err)
	}

	return path, nil
}

// Settings returns the device group's .msh settings.
func (cli *Client) Settings(ctx context.Context) (map[string]string, error) {
	settings, err := settingsOf(ctx, cli.ports)
	if err != nil {
		return nil, fmt.Errorf(errFmtSettings, err)
	}

	return settings, nil
}

// Install registers the agent service from the binary at exePath and starts it.
func (cli *Client) Install(ctx context.Context, exePath string) error {
	err := installOf(ctx, cli.ports, exePath)
	if err != nil {
		return fmt.Errorf(errFmtInstall, err)
	}

	return nil
}

// Uninstall stops the agent service, removes it and deletes its files.
func (cli *Client) Uninstall(ctx context.Context) error {
	err := uninstallOf(ctx, cli.ports)
	if err != nil {
		return fmt.Errorf(errFmtUninstall, err)
	}

	return nil
}

// Connect starts the agent service.
func (cli *Client) Connect(ctx context.Context) error {
	err := connectOf(ctx, cli.ports)
	if err != nil {
		return fmt.Errorf(errFmtConnect, err)
	}

	return nil
}

// Disconnect stops the agent service, leaving it installed and the device
// registered in its device group.
func (cli *Client) Disconnect(ctx context.Context) error {
	err := disconnectOf(ctx, cli.ports)
	if err != nil {
		return fmt.Errorf(errFmtDisconnect, err)
	}

	return nil
}

// Status reports whether the agent is installed and running.
func (cli *Client) Status(ctx context.Context) (Status, error) {
	status, err := statusOf(ctx, cli.ports)
	if err != nil {
		return Status{}, fmt.Errorf(errFmtStatus, err)
	}

	return status, nil
}

func portsOf(value any) *agentports.Ports {
	ports, ok := value.(*agentports.Ports)
	if !ok {
		return &agentports.Ports{}
	}

	return ports
}

func serviceNameOf(value any) string {
	return portsOf(value).ServiceName
}

func downloadOf(ctx context.Context, value any, destDir string) (string, error) {
	path, err := portsOf(value).Downloader.Download(ctx, destDir)
	if err != nil {
		return emptyValue, fmt.Errorf(errFmtDownload, err)
	}

	return path, nil
}

func settingsOf(ctx context.Context, value any) (map[string]string, error) {
	settings, err := portsOf(value).Settings.Settings(ctx)
	if err != nil {
		return nil, fmt.Errorf(errFmtSettings, err)
	}

	return map[string]string(settings), nil
}

func installOf(ctx context.Context, value any, exePath string) error {
	err := portsOf(value).Installer.Install(ctx, exePath)
	if err != nil {
		return fmt.Errorf(errFmtInstall, err)
	}

	return nil
}

func uninstallOf(ctx context.Context, value any) error {
	err := portsOf(value).Installer.Uninstall(ctx)
	if err != nil {
		return fmt.Errorf(errFmtUninstall, err)
	}

	return nil
}

func connectOf(ctx context.Context, value any) error {
	err := portsOf(value).Controller.Connect(ctx)
	if err != nil {
		return fmt.Errorf(errFmtConnect, err)
	}

	return nil
}

func disconnectOf(ctx context.Context, value any) error {
	err := portsOf(value).Controller.Disconnect(ctx)
	if err != nil {
		return fmt.Errorf(errFmtDisconnect, err)
	}

	return nil
}

func statusOf(ctx context.Context, value any) (Status, error) {
	status, err := portsOf(value).Controller.Status(ctx)
	if err != nil {
		return Status{}, fmt.Errorf(errFmtQueryStatus, err)
	}

	return mapStatus(&status), nil
}

// mapStatus converts a lifecycle status onto the public Status.
func mapStatus(status *lifecycle.Status) Status {
	return Status{
		State:      status.State,
		BinaryPath: status.BinaryPath,
		Installed:  status.Installed,
		Running:    status.Running,
	}
}
