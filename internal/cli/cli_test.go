// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	meshagent "github.com/gostafa/meshagent-go"
)

const (
	serverURL = "https://mesh.example.com"
	groupID   = "group-id"
	service   = "Mesh Agent"
)

var errAgent = errors.New("agent refused")

// fakeAgent records what the commands asked of it.
type fakeAgent struct {
	status        meshagent.Status
	statusErr     error
	downloadErr   error
	installErr    error
	uninstallErr  error
	connectErr    error
	disconnectErr error
	downloaded    string
	installed     string
	connected     bool
	disconnected  bool
	uninstalled   bool
}

func (agent *fakeAgent) Download(_ context.Context, destDir string) (string, error) {
	agent.downloaded = destDir

	return destDir + "/meshagent64.exe", agent.downloadErr
}

func (agent *fakeAgent) Install(_ context.Context, exePath string) error {
	agent.installed = exePath

	return agent.installErr
}

func (agent *fakeAgent) Uninstall(context.Context) error {
	agent.uninstalled = true

	return agent.uninstallErr
}

func (agent *fakeAgent) Connect(context.Context) error {
	agent.connected = true

	return agent.connectErr
}

func (agent *fakeAgent) Disconnect(context.Context) error {
	agent.disconnected = true

	return agent.disconnectErr
}

func (agent *fakeAgent) Status(context.Context) (meshagent.Status, error) {
	return agent.status, agent.statusErr
}

func (agent *fakeAgent) ServiceName() string { return service }

// newRunner returns a Runner writing into buffers, driving agent.
func newRunner(agent *fakeAgent) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	return &Runner{Agent: agent, Installer: agent, Out: out, Err: errOut}, out, errOut
}

func TestRunWithoutArgumentsPrintsUsage(t *testing.T) {
	runner, _, errOut := newRunner(&fakeAgent{})

	if code := runner.Run(nil); code != ExitUsage {
		t.Errorf("code = %d, want ExitUsage", code)
	}

	if !strings.Contains(errOut.String(), "meshagentctl manages") {
		t.Error("usage was not printed")
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{cmdHelp, flagHelpShort, flagHelpLong} {
		runner, _, _ := newRunner(&fakeAgent{})

		if code := runner.Run([]string{arg}); code != ExitOK {
			t.Errorf("%q: code = %d, want ExitOK", arg, code)
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	runner, _, _ := newRunner(&fakeAgent{})

	if code := runner.Run([]string{"bogus"}); code != ExitUsage {
		t.Errorf("code = %d, want ExitUsage", code)
	}
}

func TestConnectInstallsWhenAbsent(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: false}}
	runner, _, _ := newRunner(agent)

	code := runner.Run([]string{cmdConnect, "-server", serverURL, "-group", groupID})
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if agent.installed == "" || !agent.connected {
		t.Errorf("installed=%q connected=%v, want both", agent.installed, agent.connected)
	}
}

func TestConnectSkipsInstallWhenPresent(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: true}}
	runner, _, _ := newRunner(agent)

	code := runner.Run([]string{cmdConnect, "-server", serverURL, "-group", groupID})
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if agent.installed != "" {
		t.Errorf("installed %q despite the agent already being present", agent.installed)
	}

	if !agent.connected {
		t.Error("an installed agent was not connected")
	}
}

func TestConnectUsesTheSuppliedDirectory(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: false}}
	runner, _, _ := newRunner(agent)
	dir := t.TempDir()

	code := runner.Run([]string{cmdConnect, "-server", serverURL, "-group", groupID, "-dir", dir})
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if agent.downloaded != dir {
		t.Errorf("downloaded into %q, want %q", agent.downloaded, dir)
	}
}

func TestConnectFailures(t *testing.T) {
	tests := map[string]struct {
		agent *fakeAgent
		want  int
	}{
		"status fails":   {agent: &fakeAgent{statusErr: errAgent}, want: ExitError},
		"download fails": {agent: &fakeAgent{downloadErr: errAgent}, want: ExitError},
		"install fails":  {agent: &fakeAgent{installErr: errAgent}, want: ExitError},
		"connect fails": {
			agent: &fakeAgent{status: meshagent.Status{Installed: true}, connectErr: errAgent},
			want:  ExitError,
		},
		"not elevated": {
			agent: &fakeAgent{installErr: meshagent.ErrNotElevated},
			want:  ExitDenied,
		},
		"unauthorized download": {
			agent: &fakeAgent{downloadErr: meshagent.ErrDownloadUnauthorized},
			want:  ExitDenied,
		},
		"unsupported platform": {
			agent: &fakeAgent{statusErr: meshagent.ErrUnsupportedPlatform},
			want:  ExitError,
		},
		"not installed": {
			agent: &fakeAgent{statusErr: meshagent.ErrNotInstalled},
			want:  ExitError,
		},
		"canceled": {agent: &fakeAgent{statusErr: context.Canceled}, want: ExitError},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			runner, _, _ := newRunner(tt.agent)

			code := runner.Run([]string{cmdConnect, "-server", serverURL, "-group", groupID})
			if code != tt.want {
				t.Errorf("code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestDisconnect(t *testing.T) {
	agent := &fakeAgent{}
	runner, _, _ := newRunner(agent)

	if code := runner.Run([]string{cmdDisconnect}); code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if !agent.disconnected {
		t.Error("the agent was not disconnected")
	}
}

func TestDisconnectFailure(t *testing.T) {
	runner, _, _ := newRunner(&fakeAgent{disconnectErr: errAgent})

	if code := runner.Run([]string{cmdDisconnect}); code != ExitError {
		t.Errorf("code = %d, want ExitError", code)
	}
}

func TestUninstall(t *testing.T) {
	agent := &fakeAgent{}
	runner, _, _ := newRunner(agent)

	if code := runner.Run([]string{cmdUninstall}); code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if !agent.uninstalled {
		t.Error("the agent was not uninstalled")
	}
}

func TestUninstallFailure(t *testing.T) {
	runner, _, _ := newRunner(&fakeAgent{uninstallErr: errAgent})

	if code := runner.Run([]string{cmdUninstall}); code != ExitError {
		t.Errorf("code = %d, want ExitError", code)
	}
}

func TestStatusHumanReadable(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: true, Running: true, State: "running"}}
	runner, _, errOut := newRunner(agent)

	if code := runner.Run([]string{cmdStatus}); code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	if !strings.Contains(errOut.String(), "agent status") {
		t.Errorf("stderr = %q, want a status line", errOut.String())
	}
}

func TestStatusJSON(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: true, State: "stopped"}}
	runner, out, _ := newRunner(agent)

	if code := runner.Run([]string{cmdStatus, "-json"}); code != ExitOK {
		t.Fatalf("code = %d, want ExitOK", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, out.String())
	}

	if decoded["state"] != "stopped" {
		t.Errorf("state = %v, want stopped", decoded["state"])
	}
}

func TestStatusFailure(t *testing.T) {
	runner, _, _ := newRunner(&fakeAgent{statusErr: errAgent})

	if code := runner.Run([]string{cmdStatus}); code != ExitError {
		t.Errorf("code = %d, want ExitError", code)
	}
}

func TestBadFlagsExitUsage(t *testing.T) {
	commands := []string{cmdConnect, cmdDisconnect, cmdStatus, cmdUninstall}

	for _, command := range commands {
		runner, _, _ := newRunner(&fakeAgent{})

		if code := runner.Run([]string{command, "-nope"}); code != ExitUsage {
			t.Errorf("%s: code = %d, want ExitUsage", command, code)
		}
	}
}

func TestCommandHelpExitsClean(t *testing.T) {
	runner, _, _ := newRunner(&fakeAgent{})

	if code := runner.Run([]string{cmdConnect, flagHelpShort}); code != ExitOK {
		t.Errorf("code = %d, want ExitOK", code)
	}
}

func TestConnectWithoutServerIsAUsageError(t *testing.T) {
	// No injected agent, so the real builder runs and rejects the options.
	runner := &Runner{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

	t.Setenv(EnvServer, "")
	t.Setenv(EnvGroup, "")

	if code := runner.Run([]string{cmdConnect}); code != ExitUsage {
		t.Errorf("code = %d, want ExitUsage", code)
	}
}

func TestConnectWithoutGroupIsAUsageError(t *testing.T) {
	runner := &Runner{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

	t.Setenv(EnvServer, serverURL)
	t.Setenv(EnvGroup, "")

	if code := runner.Run([]string{cmdConnect}); code != ExitUsage {
		t.Errorf("code = %d, want ExitUsage", code)
	}
}

func TestConnectWithBadServerURLIsAUsageError(t *testing.T) {
	runner := &Runner{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

	code := runner.Run([]string{cmdConnect, "-server", "ftp://mesh.example.com", "-group", groupID})
	if code != ExitUsage {
		t.Errorf("code = %d, want ExitUsage", code)
	}
}

func TestBuiltAgentDrivesTheRealClient(t *testing.T) {
	// With no injected agent the real client is built and used; off Windows
	// the service operations refuse, which is the expected exit.
	runner := &Runner{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

	code := runner.Run([]string{cmdStatus, "-service-name", service})
	if code != ExitError {
		t.Errorf("code = %d, want ExitError from an unsupported platform", code)
	}
}

// failingWriter refuses every write, so the encode failure path is reachable.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errAgent }

func TestPackageRunUsesTheProcessStreams(t *testing.T) {
	if code := Run(nil); code != ExitUsage {
		t.Errorf("Run(nil) = %d, want ExitUsage", code)
	}
}

func TestStatusJSONEncodeFailure(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: true}}
	runner := &Runner{Agent: agent, Installer: agent, Out: failingWriter{}, Err: &bytes.Buffer{}}

	if code := runner.Run([]string{cmdStatus, "-json"}); code != ExitError {
		t.Errorf("code = %d, want ExitError when stdout refuses writes", code)
	}
}

func TestEmptyServiceNameIsAUsageError(t *testing.T) {
	commands := []string{cmdDisconnect, cmdStatus, cmdUninstall}

	for _, command := range commands {
		runner := &Runner{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

		if code := runner.Run([]string{command, "-service-name="}); code != ExitUsage {
			t.Errorf("%s: code = %d, want ExitUsage", command, code)
		}
	}
}

func TestStagingFailsWhenTempIsUnusable(t *testing.T) {
	// Point the temp directory at something that cannot hold one.
	t.Setenv("TMPDIR", "/nonexistent-meshagent-staging")

	if _, _, err := staging(emptyValue); err == nil {
		t.Error("staging succeeded with an unusable temp directory")
	}
}

func TestConnectStagingFailure(t *testing.T) {
	t.Setenv("TMPDIR", "/nonexistent-meshagent-staging")

	runner, _, _ := newRunner(&fakeAgent{status: meshagent.Status{Installed: false}})

	code := runner.Run([]string{cmdConnect, "-server", serverURL, "-group", groupID})
	if code != ExitError {
		t.Errorf("code = %d, want ExitError", code)
	}
}

func TestStagingRemovesTheImplicitDirectory(t *testing.T) {
	dir, cleanup, err := staging(emptyValue)
	if err != nil {
		t.Fatalf("staging: %v", err)
	}

	cleanup()

	if _, statErr := os.Stat(dir); statErr == nil {
		t.Error("the implicit staging directory survived cleanup")
	}
}

func TestStagingKeepsTheSuppliedDirectory(t *testing.T) {
	want := t.TempDir()

	dir, cleanup, err := staging(want)
	if err != nil {
		t.Fatalf("staging: %v", err)
	}

	cleanup()

	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}

	if _, statErr := os.Stat(want); statErr != nil {
		t.Error("a caller-supplied directory was removed")
	}
}

func TestRunnerNilStreamsFallback(t *testing.T) {
	runner := &Runner{Err: &bytes.Buffer{}}
	if code := runner.Run(nil); code != ExitUsage {
		t.Fatalf("code = %d", code)
	}
}

func TestBindInstallerFromAgent(t *testing.T) {
	agent := &fakeAgent{status: meshagent.Status{Installed: true}}
	runner := &Runner{Agent: agent, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}

	if code := runner.Run([]string{cmdDisconnect}); code != ExitOK {
		t.Fatalf("code = %d", code)
	}
}

func TestWithAgentMissingSeam(t *testing.T) {
	err := withAgent(&Runner{}, func(Agent) error { return nil })
	if !errors.Is(err, errNoAgent) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithInstallerMissingSeam(t *testing.T) {
	err := withInstaller(&Runner{}, func(Installer) error { return nil })
	if !errors.Is(err, errNoInstaller) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithInstallerWrongType(t *testing.T) {
	err := withInstaller(&Runner{Installer: "nope"}, func(Installer) error { return nil })
	if !errors.Is(err, errNoInstaller) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteUsageShortWrite(t *testing.T) {
	err := writeUsage(shortWriter{})
	if !errors.Is(err, errShortWrite) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteUsageError(t *testing.T) {
	err := writeUsage(failingWriter{})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestRemoveStagingIgnoresFailure(t *testing.T) {
	reportRemove(errAgent)
}

func TestLogLocalDoneDefaultBranch(t *testing.T) {
	agent := &fakeAgent{}
	runner, _, _ := newRunner(agent)
	_ = runner.Run([]string{cmdHelp})

	logLocalDone(context.Background(), runner, "other")
}

func TestUsageWriteFailure(t *testing.T) {
	runner := &Runner{Err: &bytes.Buffer{}, Out: &bytes.Buffer{}}
	_ = runner.Run([]string{cmdHelp})
	runner.Err = failingWriter{}

	usage(context.Background(), runner)
}

func TestServiceLabelMissingAgent(t *testing.T) {
	if got := serviceLabel(&Runner{}); got != emptyValue {
		t.Fatalf("got %q", got)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return 1, nil
}
