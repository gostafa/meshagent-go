package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"testing"

	meshagent "github.com/gostafa/meshagent-go"
)

// silence redirects stdout and stderr for the duration of a test, so exit-code
// assertions do not bury the test output in usage text.
func silence(t *testing.T) {
	t.Helper()

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}

	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devNull, devNull

	t.Cleanup(func() {
		os.Stdout, os.Stderr = stdout, stderr
		devNull.Close()
	})
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no arguments", nil, exitUsage},
		{"unknown command", []string{"bogus"}, exitUsage},
		{"help", []string{"help"}, exitOK},
		{"help flag", []string{"--help"}, exitOK},
		// Everything wrong with the command line exits 2, whether the flag
		// package caught it or we did.
		{"connect without server", []string{"connect"}, exitUsage},
		{"bad install flags", []string{"connect", "-server", "https://x", "-group", "g", "-install-flags", "nonsense"}, exitUsage},
		{"bad arch", []string{"connect", "-server", "https://x", "-group", "g", "-arch", "7"}, exitUsage},
		{"unparseable flag", []string{"status", "-nope"}, exitUsage},
		{"command help flag", []string{"connect", "-h"}, exitOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			silence(t)

			if got := run(tt.args); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestReportMapsActionableErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, exitOK},
		{"help", flag.ErrHelp, exitOK},
		{"not elevated", meshagent.ErrNotElevated, exitDenied},
		{"unauthorized download", meshagent.ErrDownloadUnauthorized, exitDenied},
		{"not installed", meshagent.ErrNotInstalled, exitError},
		{"unsupported platform", meshagent.ErrUnsupportedPlatform, exitError},
		{"cancelled", context.Canceled, exitError},
		{"anything else", errors.New("boom"), exitError},
		// The specific-error branches must survive being wrapped on the way up.
		{"wrapped not elevated", errors.Join(errors.New("install: "), meshagent.ErrNotElevated), exitDenied},
		{"usage error", usagef("bad flag"), exitUsage},
		// flag.ErrHelp arrives wrapped by flagError and must still exit clean.
		{"wrapped help", flagError(flag.ErrHelp), exitOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			silence(t)

			if got := report(tt.err); got != tt.want {
				t.Errorf("report(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestParseInstallFlags(t *testing.T) {
	tests := map[string]meshagent.InstallFlags{
		"":             meshagent.FlagBackgroundOnly,
		"background":   meshagent.FlagBackgroundOnly,
		"BACKGROUND":   meshagent.FlagBackgroundOnly,
		" interactive": meshagent.FlagInteractiveOnly,
		"both":         meshagent.FlagInteractiveAndBackground,
	}

	for input, want := range tests {
		got, err := parseInstallFlags(input)
		if err != nil {
			t.Errorf("parseInstallFlags(%q): %v", input, err)

			continue
		}

		if got != want {
			t.Errorf("parseInstallFlags(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := parseInstallFlags("sideways"); err == nil {
		t.Error("parseInstallFlags(\"sideways\") returned nil error")
	}
}

func TestParseArch(t *testing.T) {
	// Zero means "detect", so it must pass validation.
	for _, valid := range []int{0, 3, 4, 43} {
		if err := parseArch(valid); err != nil {
			t.Errorf("parseArch(%d): %v", valid, err)
		}
	}

	for _, invalid := range []int{1, 7, -4, 44} {
		if err := parseArch(invalid); err == nil {
			t.Errorf("parseArch(%d) returned nil error", invalid)
		}
	}
}

func TestLocalClientUsesServiceName(t *testing.T) {
	opts := options{serviceName: "Custom Mesh Agent"}

	client, err := opts.localClient()
	if err != nil {
		t.Fatalf("localClient: %v", err)
	}

	if client.ServiceName() != "Custom Mesh Agent" {
		t.Errorf("ServiceName = %q, want %q", client.ServiceName(), "Custom Mesh Agent")
	}
}

func TestClientRequiresServerAndGroup(t *testing.T) {
	if _, err := (&options{group: "g"}).client(); err == nil {
		t.Error("client() with no server returned nil error")
	}

	if _, err := (&options{server: "https://x"}).client(); err == nil {
		t.Error("client() with no group returned nil error")
	}
}
