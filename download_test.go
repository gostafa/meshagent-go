package meshagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validExe is the smallest byte sequence that passes the MZ header check.
var validExe = append([]byte("MZ"), []byte("\x90\x00fake windows binary")...)

func newTestClient(t *testing.T, serverURL string, mutate func(*Config)) *Client {
	t.Helper()

	cfg := Config{
		ServerURL: serverURL,
		GroupID:   "group$id@with/specials",
		Arch:      ArchWindows64,
	}

	if mutate != nil {
		mutate(&cfg)
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

func TestDownloadRequestShape(t *testing.T) {
	var got *url.URL

	var gotCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL
		gotCookie = r.Header.Get("Cookie")
		w.Write(validExe)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, func(c *Config) {
		c.InstallFlags = FlagBackgroundOnly
		c.AuthCookie = "xid=abc"
	})

	path, err := client.Download(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if got.Path != "/meshagents" {
		t.Errorf("path = %q, want /meshagents", got.Path)
	}

	query := got.Query()

	if query.Get("id") != "4" {
		t.Errorf("id = %q, want 4", query.Get("id"))
	}

	// The group id contains $, @ and / and must survive encoding intact.
	if query.Get("meshid") != "group$id@with/specials" {
		t.Errorf("meshid = %q, want the raw group id", query.Get("meshid"))
	}

	if query.Get("installflags") != "2" {
		t.Errorf("installflags = %q, want 2 for FlagBackgroundOnly", query.Get("installflags"))
	}

	if gotCookie != "xid=abc" {
		t.Errorf("Cookie = %q, want xid=abc", gotCookie)
	}

	if filepath.Base(path) != "meshagent64.exe" {
		t.Errorf("saved as %q, want meshagent64.exe", filepath.Base(path))
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}

	if string(contents) != string(validExe) {
		t.Error("downloaded contents do not match what the server sent")
	}
}

func TestDownloadOmitsZeroInstallFlags(t *testing.T) {
	var got *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL
		w.Write(validExe)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, func(c *Config) {
		c.InstallFlags = FlagInteractiveAndBackground
	})

	if _, err := client.Download(context.Background(), t.TempDir()); err != nil {
		t.Fatalf("Download: %v", err)
	}

	// MeshCentral drops the InstallFlags line from the .msh when the value is
	// zero, so sending it would be noise.
	if _, present := got.Query()["installflags"]; present {
		t.Error("installflags was sent for the zero value, want it omitted")
	}
}

func TestDownloadUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil)

	_, err := client.Download(context.Background(), t.TempDir())
	if !errors.Is(err, ErrDownloadUnauthorized) {
		t.Fatalf("err = %v, want ErrDownloadUnauthorized", err)
	}
}

func TestDownloadRejectsNonExecutable(t *testing.T) {
	tests := map[string]string{
		"html error page under 200": "<html><body>Not found</body></html>",
		"empty body":                "",
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(body))
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, nil)
			destDir := t.TempDir()

			_, err := client.Download(context.Background(), destDir)

			var invalid *InvalidBinaryError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want InvalidBinaryError", err)
			}

			// A rejected download must not leave anything behind.
			entries, err := os.ReadDir(destDir)
			if err != nil {
				t.Fatalf("read dest dir: %v", err)
			}

			if len(entries) != 0 {
				t.Errorf("destination has %d leftover entries, want 0", len(entries))
			}
		})
	}
}

func TestDownloadHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil)

	_, err := client.Download(context.Background(), t.TempDir())

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want HTTPError", err)
	}

	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", httpErr.StatusCode)
	}
}

func TestSettings(t *testing.T) {
	const msh = "\r\nMeshName=base test\r\nMeshType=2\r\nMeshID=0xDEADBEEF\r\n" +
		"ServerID=0xCAFE\r\nMeshServer=wss://remote.example.com:443/agent.ashx\r\n"

	var got *url.URL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL
		w.Write([]byte(msh))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, nil)

	settings, err := client.Settings(context.Background())
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}

	if got.Path != "/meshsettings" {
		t.Errorf("path = %q, want /meshsettings", got.Path)
	}

	if settings["MeshName"] != "base test" {
		t.Errorf("MeshName = %q, want %q", settings["MeshName"], "base test")
	}

	// MeshServer is a URL and contains "=" free colons and slashes; only the
	// first separator may split.
	if settings["MeshServer"] != "wss://remote.example.com:443/agent.ashx" {
		t.Errorf("MeshServer = %q, want the full URL", settings["MeshServer"])
	}

	if err := settings.Required(); err != nil {
		t.Errorf("Required: %v", err)
	}
}

func TestSettingsRequiredReportsMissingKeys(t *testing.T) {
	settings := Settings{"MeshID": "0x1", "ServerID": "  "}

	err := settings.Required()
	if err == nil {
		t.Fatal("Required returned nil for settings missing ServerID and MeshServer")
	}

	if !strings.Contains(err.Error(), "ServerID") {
		t.Errorf("error = %q, want it to name ServerID", err)
	}
}

func TestParseSettingsIgnoresJunk(t *testing.T) {
	settings := parseSettings([]byte("# comment\n\nMeshID=0x1\nnot-a-pair\n  Spaced = value \n"))

	if len(settings) != 2 {
		t.Fatalf("parsed %d keys, want 2: %v", len(settings), settings)
	}

	if settings["Spaced"] != "value" {
		t.Errorf("Spaced = %q, want %q", settings["Spaced"], "value")
	}
}

func TestNewValidation(t *testing.T) {
	tests := map[string]Config{
		"missing server url": {GroupID: "g"},
		"missing group id":   {ServerURL: "https://example.com"},
		"bad scheme":         {ServerURL: "ftp://example.com", GroupID: "g"},
		"missing host":       {ServerURL: "https://", GroupID: "g"},
		"bad arch":           {ServerURL: "https://example.com", GroupID: "g", Arch: Arch(99)},
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := New(cfg); err == nil {
				t.Error("New returned nil error, want a validation failure")
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	client, err := New(Config{ServerURL: "https://example.com/", GroupID: "g"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if client.ServiceName() != DefaultServiceName {
		t.Errorf("ServiceName = %q, want %q", client.ServiceName(), DefaultServiceName)
	}

	if client.Arch() != DetectArch() {
		t.Errorf("Arch = %v, want DetectArch() = %v", client.Arch(), DetectArch())
	}

	if client.installFlags != FlagBackgroundOnly {
		t.Errorf("installFlags = %v, want FlagBackgroundOnly", client.installFlags)
	}
}

func TestEndpointPreservesDomainPath(t *testing.T) {
	// MeshCentral serves non-default domains under a path prefix; it must not
	// be dropped when building endpoints.
	client, err := New(Config{ServerURL: "https://example.com/customer1", GroupID: "g"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	endpoint := client.endpoint("meshagents", url.Values{"id": {"4"}})

	if !strings.HasPrefix(endpoint, "https://example.com/customer1/meshagents?") {
		t.Errorf("endpoint = %q, want the domain path preserved", endpoint)
	}
}
