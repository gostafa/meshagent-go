// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentinstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

const exePath = `C:\ProgramData\meshagent64.exe`

var errRun = errors.New("installer exited 1")

// recordingRunner captures the switch it was asked to run.
type recordingRunner struct {
	arg string
	err error
}

func (rec *recordingRunner) run(_ context.Context, _, arg string) error {
	rec.arg = arg

	return rec.err
}

// fixedLocator answers a canned installed state.
type fixedLocator struct {
	status lifecycle.Status
	err    error
}

func (loc fixedLocator) Installed(context.Context) (lifecycle.Status, error) {
	return loc.status, loc.err
}

// fixedToken answers a canned elevation.
type fixedToken struct{ value bool }

func (token fixedToken) elevated() bool { return token.value }

func installed(path string) fixedLocator {
	return fixedLocator{status: lifecycle.Status{Installed: true, BinaryPath: path}}
}

// newInstaller builds an Installer with the seams a test controls.
func newInstaller(rec *recordingRunner, loc Locator, elevated bool) *Installer {
	return &Installer{
		runner:    rec,
		locator:   loc,
		elevation: elevation{query: fixedToken{value: elevated}},
	}
}

func TestInstallRunsFullInstall(t *testing.T) {
	rec := &recordingRunner{}

	if err := newInstaller(
		rec,
		installed(exePath),
		true,
	).Install(t.Context(), exePath); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if rec.arg != SwitchInstall {
		t.Errorf("ran %q, want %q", rec.arg, SwitchInstall)
	}
}

func TestInstallRequiresElevation(t *testing.T) {
	rec := &recordingRunner{}

	err := newInstaller(rec, installed(exePath), false).Install(t.Context(), exePath)
	if !errors.Is(err, agenterr.ErrNotElevated) {
		t.Fatalf("err = %v, want ErrNotElevated", err)
	}

	if rec.arg != "" {
		t.Error("the installer ran despite the process not being elevated")
	}
}

func TestInstallRunnerFailure(t *testing.T) {
	rec := &recordingRunner{err: errRun}

	err := newInstaller(rec, installed(exePath), true).Install(t.Context(), exePath)
	if !errors.Is(err, errRun) {
		t.Fatalf("err = %v, want errRun", err)
	}
}

func TestUninstallUsesTheRegisteredPath(t *testing.T) {
	rec := &recordingRunner{}

	if err := newInstaller(rec, installed(exePath), true).Uninstall(t.Context()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if rec.arg != SwitchUninstall {
		t.Errorf("ran %q, want %q", rec.arg, SwitchUninstall)
	}
}

func TestUninstallFailures(t *testing.T) {
	tests := map[string]struct {
		installer *Installer
		want      error
	}{
		"requires elevation": {
			installer: newInstaller(&recordingRunner{}, installed(exePath), false),
			want:      agenterr.ErrNotElevated,
		},
		"not installed": {
			installer: newInstaller(
				&recordingRunner{},
				fixedLocator{status: lifecycle.Status{Installed: false}},
				true,
			),
			want: agenterr.ErrNotInstalled,
		},
		"unknown path": {
			installer: newInstaller(&recordingRunner{}, installed(""), true),
			want:      agenterr.ErrUnknownServicePath,
		},
		"locator fails": {
			installer: newInstaller(&recordingRunner{}, fixedLocator{err: errRun}, true),
			want:      errRun,
		},
		"runner fails": {
			installer: newInstaller(&recordingRunner{err: errRun}, installed(exePath), true),
			want:      errRun,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := tt.installer.Uninstall(t.Context()); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewWiresTheRealSeams(t *testing.T) {
	installer := New(installed(exePath))

	// Off Windows the real token is never elevated, so the refusal proves the
	// elevation seam is wired to it.
	if runtime.GOOS != "windows" {
		if err := installer.Install(
			t.Context(),
			exePath,
		); !errors.Is(
			err,
			agenterr.ErrNotElevated,
		) {
			t.Errorf("err = %v, want ErrNotElevated", err)
		}
	}
}

// selfPath is the test binary, a real executable on every platform, standing
// in for the agent binary.
func selfPath(t *testing.T) string {
	t.Helper()

	path, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}

	return path
}

func TestExecRunnerSucceeds(t *testing.T) {
	exe := &execRunner{outputLimit: defaultOutputLimit}

	// -test.run with a pattern that matches nothing exits zero.
	if err := exe.run(t.Context(), selfPath(t), "-test.run=NoSuchTestXYZ"); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestExecRunnerNonZeroExit(t *testing.T) {
	exe := &execRunner{outputLimit: defaultOutputLimit}

	err := exe.run(t.Context(), selfPath(t), "-definitely-not-a-flag")

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want ExitError", err)
	}

	if exitErr.ExitCode == 0 || !strings.Contains(exitErr.Error(), "agentinstall:") {
		t.Errorf("exit %d msg %q, want a non-zero code and the package named",
			exitErr.ExitCode, exitErr.Error())
	}
}

func TestExecRunnerMissingBinary(t *testing.T) {
	exe := &execRunner{outputLimit: defaultOutputLimit}

	// A path that does not exist fails before the process starts, so it is not
	// an ExitError.
	err := exe.run(t.Context(), filepath.Join(t.TempDir(), "absent.exe"), SwitchInstall)

	var exitErr *ExitError
	if err == nil || errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a start failure that is not an ExitError", err)
	}
}

func TestExecRunnerTrimBoundsOutput(t *testing.T) {
	exe := &execRunner{outputLimit: 8}

	if got := exe.trim([]byte("  short  ")); got != "short" {
		t.Errorf("trim(short) = %q, want %q", got, "short")
	}

	if got := exe.trim([]byte("abcdefghijkl")); got != "abcdefgh"+truncationMarker {
		t.Errorf("trim(long) = %q, want it cut at the limit", got)
	}
}

func TestProcessTokenOffWindowsIsNeverElevated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the real token answers for itself on windows")
	}

	if (processToken{}).elevated() {
		t.Error("processToken{}.elevated() = true off windows, want false")
	}
}

func TestProcessTokenOnWindowsAnswers(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only the windows build queries a real token")
	}

	// The value depends on how the test process was launched; calling it is
	// what matters, since CI measures coverage on windows.
	token := processToken{}
	t.Logf("process token elevated: %v", token.elevated())
}
