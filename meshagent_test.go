// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshagent

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/infrastructure/agentports"
)

const (
	serverURL = "https://mesh.example.com"
	groupID   = "group-id"
)

func valid() *Config {
	return &Config{ServerURL: serverURL, GroupID: groupID}
}

func TestNewDefaults(t *testing.T) {
	client, err := New(valid())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if client.ServiceName() != DefaultServiceName {
		t.Errorf("ServiceName = %q, want %q", client.ServiceName(), DefaultServiceName)
	}
}

func TestNewHonoursOverrides(t *testing.T) {
	cfg := valid()
	cfg.ServiceName = "Custom Agent"
	cfg.Arch = int(ArchWindows32)
	cfg.InstallFlags = string(FlagInteractiveOnly)
	cfg.HTTPClient = &http.Client{}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if client.ServiceName() != "Custom Agent" {
		t.Errorf("ServiceName = %q, want the override", client.ServiceName())
	}
}

func TestNewValidation(t *testing.T) {
	tests := map[string]struct {
		mutate func(cfg *Config)
		want   error
	}{
		"missing server url": {
			mutate: func(cfg *Config) { cfg.ServerURL = "" },
			want:   errServerURLRequired,
		},
		"missing group id": {
			mutate: func(cfg *Config) { cfg.GroupID = "" },
			want:   errGroupIDRequired,
		},
		"bad scheme": {
			mutate: func(cfg *Config) { cfg.ServerURL = "ftp://x" },
			want:   errBadScheme,
		},
		"missing host": {
			mutate: func(cfg *Config) { cfg.ServerURL = "https://" },
			want:   errMissingHost,
		},
		"bad arch": {mutate: func(cfg *Config) { cfg.Arch = 99 }, want: errBadArch},
		"bad install flags": {
			mutate: func(cfg *Config) { cfg.InstallFlags = "sideways" },
			want:   errBadInstallFlags,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(cfg)

			_, err := New(cfg)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewRejectsUnparseableURL(t *testing.T) {
	cfg := valid()
	cfg.ServerURL = "https://exa mple.com/\x7f"

	if _, err := New(cfg); err == nil {
		t.Fatal("New accepted an unparseable ServerURL")
	}
}

func TestDetectArchIsValid(t *testing.T) {
	if !DetectArch().Valid() {
		t.Errorf("DetectArch() = %v, which is not a valid architecture", DetectArch())
	}
}

// TestServiceOperationsRefuseOffWindows pins the platform contract. The
// operations delegate to adapters that report ErrUnsupportedPlatform, and the
// wrapping must keep that matchable.
func TestServiceOperationsRefuseOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these operations are supported on windows")
	}

	client, err := New(valid())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, statusErr := client.Status(t.Context())

	errs := map[string]error{
		"Status":     statusErr,
		"Connect":    client.Connect(t.Context()),
		"Disconnect": client.Disconnect(t.Context()),
	}

	for name, opErr := range errs {
		if !errors.Is(opErr, ErrUnsupportedPlatform) {
			t.Errorf("%s err = %v, want ErrUnsupportedPlatform", name, opErr)
		}
	}
}

func TestInstallOperationsRefuseWithoutElevation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("elevation on windows depends on how the test was launched")
	}

	client, err := New(valid())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Off Windows the elevation check always reports false.
	if installErr := client.Install(
		t.Context(),
		"agent.exe",
	); !errors.Is(
		installErr,
		ErrNotElevated,
	) {
		t.Errorf("Install err = %v, want ErrNotElevated", installErr)
	}

	if uninstallErr := client.Uninstall(t.Context()); !errors.Is(uninstallErr, ErrNotElevated) {
		t.Errorf("Uninstall err = %v, want ErrNotElevated", uninstallErr)
	}
}

// fakePorts satisfies every port the Client delegates to, so the success path
// of each method is reachable without a server or a service manager.
type fakePorts struct{}

func (fakePorts) Download(context.Context, string) (string, error) {
	return "agent.exe", nil
}

func (fakePorts) Settings(context.Context) (artifact.Settings, error) {
	return artifact.Settings{artifact.KeyMeshID: "0x1"}, nil
}

func (fakePorts) Install(context.Context, string) error { return nil }
func (fakePorts) Uninstall(context.Context) error       { return nil }
func (fakePorts) Connect(context.Context) error         { return nil }
func (fakePorts) Disconnect(context.Context) error      { return nil }

func (fakePorts) Status(context.Context) (lifecycle.Status, error) {
	return lifecycle.Status{Installed: true, Running: true, State: "running"}, nil
}

func TestEveryMethodDelegates(t *testing.T) {
	ports := fakePorts{}
	client := &Client{ports: &agentports.Ports{
		Downloader:  ports,
		Settings:    ports,
		Installer:   ports,
		Controller:  ports,
		ServiceName: DefaultServiceName,
	}}

	path, err := client.Download(t.Context(), t.TempDir())
	if err != nil || path != "agent.exe" {
		t.Errorf("Download = %q, %v", path, err)
	}

	settings, err := client.Settings(t.Context())
	if err != nil || settings[artifact.KeyMeshID] != "0x1" {
		t.Errorf("Settings = %v, %v", settings, err)
	}

	status, err := client.Status(t.Context())
	if err != nil || !status.Running {
		t.Errorf("Status = %+v, %v", status, err)
	}

	errs := map[string]error{
		"Install":    client.Install(t.Context(), "agent.exe"),
		"Uninstall":  client.Uninstall(t.Context()),
		"Connect":    client.Connect(t.Context()),
		"Disconnect": client.Disconnect(t.Context()),
	}

	for name, opErr := range errs {
		if opErr != nil {
			t.Errorf("%s: %v", name, opErr)
		}
	}
}

var errPort = errors.New("port refused")

// failingPorts satisfies every port the Client delegates to, so the error
// wrap of each method is reachable without a server or a service manager.
type failingPorts struct{}

func (failingPorts) Download(context.Context, string) (string, error) {
	return "", errPort
}

func (failingPorts) Settings(context.Context) (artifact.Settings, error) {
	return nil, errPort
}

func (failingPorts) Install(context.Context, string) error { return errPort }
func (failingPorts) Uninstall(context.Context) error       { return errPort }
func (failingPorts) Connect(context.Context) error         { return errPort }
func (failingPorts) Disconnect(context.Context) error      { return errPort }

func (failingPorts) Status(context.Context) (lifecycle.Status, error) {
	return lifecycle.Status{}, errPort
}

func TestEveryMethodWrapsPortErrors(t *testing.T) {
	ports := failingPorts{}
	client := &Client{ports: &agentports.Ports{
		Downloader:  ports,
		Settings:    ports,
		Installer:   ports,
		Controller:  ports,
		ServiceName: DefaultServiceName,
	}}

	path, err := client.Download(t.Context(), t.TempDir())
	if path != "" || !errors.Is(err, errPort) {
		t.Errorf("Download = %q, %v", path, err)
	}

	settings, err := client.Settings(t.Context())
	if settings != nil || !errors.Is(err, errPort) {
		t.Errorf("Settings = %v, %v", settings, err)
	}

	status, err := client.Status(t.Context())
	if status != (Status{}) || !errors.Is(err, errPort) {
		t.Errorf("Status = %+v, %v", status, err)
	}

	errs := map[string]error{
		"Install":    client.Install(t.Context(), "agent.exe"),
		"Uninstall":  client.Uninstall(t.Context()),
		"Connect":    client.Connect(t.Context()),
		"Disconnect": client.Disconnect(t.Context()),
	}

	for name, opErr := range errs {
		if !errors.Is(opErr, errPort) {
			t.Errorf("%s: %v", name, opErr)
		}
	}
}

func TestDownloadAndSettingsReachTheServer(t *testing.T) {
	cfg := valid()
	// A server that cannot be resolved makes both operations fail in the
	// transport, which is enough to prove they are wired to it.
	cfg.ServerURL = "https://meshagent.invalid"

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, downloadErr := client.Download(t.Context(), t.TempDir()); downloadErr == nil {
		t.Error("Download against an unresolvable host returned nil error")
	}

	_, settingsErr := client.Settings(t.Context())
	if settingsErr == nil || !strings.Contains(settingsErr.Error(), "meshagent:") {
		t.Errorf("Settings err = %v, want a wrapped failure", settingsErr)
	}
}

func TestPortsOfFallback(t *testing.T) {
	if ports := portsOf("not-ports"); ports == nil {
		t.Fatal("portsOf fallback returned nil")
	}

	if name := serviceNameOf(nil); name != "" {
		t.Fatalf("serviceNameOf(nil) = %q", name)
	}
}
