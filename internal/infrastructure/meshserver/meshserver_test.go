// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshserver

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gostafa/meshagent-go/internal/features/agent/artifact"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
	"github.com/gostafa/meshagent-go/internal/shared/meshpath"
)

// validExe is the smallest byte sequence that passes the MZ header check.
const validExe = "MZ\x90\x00fake windows binary"

var errTransport = errors.New("transport exploded")

// failingDoer fails every request, standing in for a dead network.
type failingDoer struct{}

func (failingDoer) Do(*http.Request) (*http.Response, error) { return nil, errTransport }

// doerFunc adapts a function to the Doer seam.
type doerFunc func(req *http.Request) (*http.Response, error)

func (fn doerFunc) Do(req *http.Request) (*http.Response, error) { return fn(req) }

// failingBody yields bytes then fails, exercising a mid-stream read error.
type failingBody struct{ read bool }

func (body *failingBody) Read(p []byte) (int, error) {
	if body.read {
		return 0, errTransport
	}

	body.read = true
	copy(p, validExe)

	return len(validExe), nil
}

func (body *failingBody) Close() error { return nil }

// failingCloser reports an error from Close only.
type failingCloser struct{ io.Reader }

func (failingCloser) Close() error { return errTransport }

func newClient(t *testing.T, serverURL string, mutate func(*Config)) *Client {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse %q: %v", serverURL, err)
	}

	cfg := Config{
		ServerURL:    parsed,
		Doer:         http.DefaultClient,
		GroupID:      "group$id@with/specials",
		Arch:         artifact.ArchWindows64,
		InstallFlags: artifact.FlagBackgroundOnly,
	}

	if mutate != nil {
		mutate(&cfg)
	}

	return New(&cfg)
}

func serve(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server.URL
}

func TestDownloadRequestShape(t *testing.T) {
	var (
		got       *url.URL
		gotCookie string
	)

	addr := serve(t, func(writer http.ResponseWriter, req *http.Request) {
		got = req.URL
		gotCookie = req.Header.Get(cookieHeader)

		_, _ = io.WriteString(writer, validExe)
	})

	client := newClient(t, addr, func(cfg *Config) { cfg.AuthCookie = "xid=abc" })

	path, err := client.Download(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	query := got.Query()
	if got.Path != "/"+meshpath.AgentsEndpoint || query.Get(queryID) != "4" {
		t.Errorf("path %q query %v, want /%s with id=4", got.Path, query, meshpath.AgentsEndpoint)
	}

	// The group id contains $, @ and / and must survive encoding intact.
	if query.Get(queryGroup) != "group$id@with/specials" {
		t.Errorf("meshid = %q, want the raw group id", query.Get(queryGroup))
	}

	if query.Get(queryFlags) != "2" {
		t.Errorf("installflags = %q, want 2", query.Get(queryFlags))
	}

	if gotCookie != "xid=abc" {
		t.Errorf("Cookie = %q, want xid=abc", gotCookie)
	}

	if filepath.Base(path) != "meshagent64.exe" {
		t.Errorf("saved as %q, want meshagent64.exe", filepath.Base(path))
	}
}

func TestDownloadOmitsZeroInstallFlags(t *testing.T) {
	var got *url.URL

	addr := serve(t, func(writer http.ResponseWriter, req *http.Request) {
		got = req.URL

		_, _ = io.WriteString(writer, validExe)
	})

	client := newClient(t, addr, func(cfg *Config) {
		cfg.InstallFlags = artifact.FlagInteractiveAndBackground
	})

	if _, err := client.Download(t.Context(), t.TempDir()); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if _, present := got.Query()[queryFlags]; present {
		t.Error("installflags was sent for the zero wire value, want it omitted")
	}
}

func TestDownloadUnauthorized(t *testing.T) {
	addr := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	})

	_, err := newClient(t, addr, nil).Download(t.Context(), t.TempDir())
	if !errors.Is(err, agenterr.ErrDownloadUnauthorized) {
		t.Fatalf("err = %v, want ErrDownloadUnauthorized", err)
	}
}

func TestDownloadHTTPError(t *testing.T) {
	addr := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	})

	_, err := newClient(t, addr, nil).Download(t.Context(), t.TempDir())

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want HTTPError", err)
	}

	if httpErr.Code != http.StatusInternalServerError || httpErr.Error() == "" {
		t.Errorf("Code = %d, want 500 with a message", httpErr.Code)
	}
}

func TestDownloadRejectsNonExecutable(t *testing.T) {
	tests := map[string]string{
		"html error page under 200": "<html><body>Not found</body></html>",
		"empty body":                "",
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			addr := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, body)
			})

			destDir := t.TempDir()

			_, err := newClient(t, addr, nil).Download(t.Context(), destDir)

			var invalid *InvalidBinaryError
			if !errors.As(err, &invalid) || invalid.Error() == "" {
				t.Fatalf("err = %v, want InvalidBinaryError with a message", err)
			}

			// A rejected download must not leave anything behind.
			entries, readErr := os.ReadDir(destDir)
			if readErr != nil {
				t.Fatalf("read dest dir: %v", readErr)
			}

			if len(entries) != 0 {
				t.Errorf("destination has %d leftover entries, want 0", len(entries))
			}
		})
	}
}

func TestDownloadTransportFailure(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", func(cfg *Config) { cfg.Doer = failingDoer{} })

	_, err := client.Download(t.Context(), t.TempDir())
	if !errors.Is(err, errTransport) {
		t.Fatalf("err = %v, want errTransport", err)
	}
}

func TestDownloadBodyReadFailure(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", func(cfg *Config) {
		cfg.Doer = doerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: &failingBody{}}, nil
		})
	})

	_, err := client.Download(t.Context(), t.TempDir())
	if !errors.Is(err, errTransport) {
		t.Fatalf("err = %v, want errTransport", err)
	}
}

func TestDownloadCloseFailureSurfaces(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", func(cfg *Config) {
		cfg.Doer = doerFunc(func(*http.Request) (*http.Response, error) {
			body := failingCloser{Reader: strings.NewReader(validExe)}

			return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
		})
	})

	_, err := client.Download(t.Context(), t.TempDir())
	if !errors.Is(err, errTransport) {
		t.Fatalf("err = %v, want the close error joined in", err)
	}
}

func TestDownloadUncreatableDestination(t *testing.T) {
	// A regular file cannot become a directory.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	_, err := newClient(t, "https://mesh.invalid", nil).Download(t.Context(), blocker)
	if err == nil {
		t.Fatal("Download into a file path returned nil error")
	}
}

func TestDownloadBadRequestURL(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", nil)
	// A scheme containing a space cannot be turned into a request.
	client.locator.serverURL.Scheme = "ht tp"

	_, err := client.Download(t.Context(), t.TempDir())
	if err == nil {
		t.Fatal("Download with an unbuildable URL returned nil error")
	}
}

func TestDownloadUncreatableTempFile(t *testing.T) {
	// A read-only directory admits no temp file.
	destDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(destDir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := newClient(t, "https://mesh.invalid", nil).Download(t.Context(), destDir)
	if err == nil {
		t.Fatal("Download into a read-only directory returned nil error")
	}
}

func TestInvalidBinary(t *testing.T) {
	var invalid *InvalidBinaryError
	if err := invalidBinary(io.EOF, nil); !errors.As(err, &invalid) {
		t.Errorf("invalidBinary(io.EOF) = %v, want InvalidBinaryError", err)
	}

	// Anything other than EOF is a transport problem, not a bad binary.
	err := invalidBinary(errTransport, nil)
	if errors.As(err, &invalid) || !errors.Is(err, errTransport) {
		t.Errorf("invalidBinary(errTransport) = %v, want the transport error wrapped", err)
	}
}

func TestPromoteFailures(t *testing.T) {
	tests := map[string]func(t *testing.T) (*os.File, string){
		"close fails on an already closed file": func(t *testing.T) (*os.File, string) {
			t.Helper()

			temp := mustTemp(t)
			_ = temp.Close()

			return temp, filepath.Join(t.TempDir(), "agent.exe")
		},
		"chmod fails when the file is gone": func(t *testing.T) (*os.File, string) {
			t.Helper()

			temp := mustTemp(t)
			_ = os.Remove(temp.Name())

			return temp, filepath.Join(t.TempDir(), "agent.exe")
		},
		"rename fails into a missing directory": func(t *testing.T) (*os.File, string) {
			t.Helper()

			return mustTemp(t), filepath.Join(t.TempDir(), "absent", "agent.exe")
		},
	}

	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			temp, target := setup(t)
			if err := promote(temp, target); err == nil {
				t.Error("promote returned nil error")
			}
		})
	}
}

func TestPromoteSucceeds(t *testing.T) {
	temp := mustTemp(t)
	target := filepath.Join(t.TempDir(), "agent.exe")

	if err := promote(temp, target); err != nil {
		t.Fatalf("promote: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}

	if info.Mode().Perm() != exePerm {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), os.FileMode(exePerm))
	}
}

func TestIgnoreClosedAndMissing(t *testing.T) {
	if ignoreClosed(nil) != nil || ignoreClosed(os.ErrClosed) != nil {
		t.Error("ignoreClosed dropped neither nil nor ErrClosed")
	}

	if err := ignoreClosed(errTransport); !errors.Is(err, errTransport) {
		t.Errorf("ignoreClosed(errTransport) = %v, want it wrapped", err)
	}

	if ignoreMissing(nil) != nil || ignoreMissing(os.ErrNotExist) != nil {
		t.Error("ignoreMissing dropped neither nil nor ErrNotExist")
	}

	if err := ignoreMissing(errTransport); !errors.Is(err, errTransport) {
		t.Errorf("ignoreMissing(errTransport) = %v, want it wrapped", err)
	}
}

func mustTemp(t *testing.T) *os.File {
	t.Helper()

	temp, err := os.CreateTemp(t.TempDir(), "promote-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}

	return temp
}

func TestSettings(t *testing.T) {
	const msh = "\r\nMeshName=base test\r\nMeshID=0xDEADBEEF\r\n" +
		"ServerID=0xCAFE\r\nMeshServer=wss://remote.example.com:443/agent.ashx\r\n"

	var got *url.URL

	addr := serve(t, func(writer http.ResponseWriter, req *http.Request) {
		got = req.URL

		_, _ = io.WriteString(writer, msh)
	})

	settings, err := newClient(t, addr, nil).Settings(t.Context())
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}

	if got.Path != "/"+meshpath.SettingsEndpoint {
		t.Errorf("path = %q, want /%s", got.Path, meshpath.SettingsEndpoint)
	}

	if settings[artifact.KeyMeshServer] != "wss://remote.example.com:443/agent.ashx" {
		t.Errorf("MeshServer = %q, want the full URL", settings[artifact.KeyMeshServer])
	}

	if err := settings.Required(); err != nil {
		t.Errorf("Required: %v", err)
	}
}

func TestDownloadPromoteFailure(t *testing.T) {
	addr := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, validExe)
	})

	destDir := t.TempDir()

	// A directory already occupying the target name cannot be renamed onto.
	blocker := filepath.Join(destDir, artifact.ArchWindows64.BinaryName())
	if err := os.Mkdir(blocker, 0o750); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}

	if err := os.WriteFile(filepath.Join(blocker, "occupied"), []byte("x"), 0o600); err != nil {
		t.Fatalf("fill blocker: %v", err)
	}

	_, err := newClient(t, addr, nil).Download(t.Context(), destDir)
	if err == nil {
		t.Fatal("Download onto an occupied directory returned nil error")
	}
}

func TestSettingsBodyReadFailure(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", func(cfg *Config) {
		cfg.Doer = doerFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: &failingBody{}}, nil
		})
	})

	_, err := client.Settings(t.Context())
	if !errors.Is(err, errTransport) {
		t.Fatalf("err = %v, want errTransport", err)
	}
}

func TestSettingsFailure(t *testing.T) {
	client := newClient(t, "https://mesh.invalid", func(cfg *Config) { cfg.Doer = failingDoer{} })

	_, err := client.Settings(t.Context())
	if !errors.Is(err, errTransport) {
		t.Fatalf("err = %v, want errTransport", err)
	}
}
