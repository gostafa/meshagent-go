// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	meshagent "github.com/gostafa/meshagent-go"
)

// Run executes one command against the process streams and returns the exit
// code.
func Run(args []string) int {
	return Execute(&Runner{Out: os.Stdout, Err: os.Stderr}, args)
}

// Execute runs one command against run's streams.
func Execute(run *Runner, args []string) int {
	return run.Run(args)
}

// Error implements the error interface.
func (usageErr *usageError) Error() string { return usageErr.err.Error() }

// Unwrap exposes the wrapped cause.
func (usageErr *usageError) Unwrap() error { return usageErr.err }

// Run executes one command against this Runner's streams.
func (run *Runner) Run(args []string) int {
	run.log = slog.New(slog.NewTextHandler(run.Err, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if run.Out == nil && run.Agent == nil && run.Installer == nil {
		run.Out = io.Discard
	}

	return dispatch(run, args)
}

func dispatch(run *Runner, args []string) int {
	ctx := context.Background()

	if len(args) == ExitOK {
		usage(ctx, run)

		return ExitUsage
	}

	return dispatchCommand(ctx, run, args)
}

func dispatchCommand(ctx context.Context, run *Runner, args []string) int {
	command := args[ExitOK]
	if isHelp(command) {
		usage(ctx, run)

		return ExitOK
	}

	return invoke(ctx, run, args)
}

func isHelp(command string) bool {
	return command == flagHelpShort || command == flagHelpLong || command == cmdHelp
}

func invoke(ctx context.Context, run *Runner, args []string) int {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	action, known := actions(run)[args[ExitOK]]
	if !known {
		return unknownCommand(ctx, run, args[ExitOK])
	}

	return report(ctx, run, action(ctx, args[1:]))
}

func unknownCommand(ctx context.Context, run *Runner, command string) int {
	run.log.ErrorContext(ctx, msgUnknownCommand, slog.String(keyCommand, command))
	usage(ctx, run)

	return ExitUsage
}

func actions(run *Runner) map[string]func(context.Context, []string) error {
	return map[string]func(context.Context, []string) error{
		cmdConnect:    boundAction(run, connect),
		cmdDisconnect: localAction(run, cmdDisconnect),
		cmdStatus:     boundAction(run, statusCmd),
		cmdUninstall:  localAction(run, cmdUninstall),
	}
}

func boundAction(
	run *Runner,
	call func(context.Context, *Runner, []string) error,
) func(context.Context, []string) error {
	return func(ctx context.Context, args []string) error {
		err := call(ctx, run, args)
		if err != nil {
			return fmt.Errorf("cli: command: %w", err)
		}

		return nil
	}
}

func localAction(run *Runner, name string) func(context.Context, []string) error {
	return func(ctx context.Context, args []string) error {
		err := runLocal(ctx, run, &struct {
			name string
			args []string
		}{name: name, args: args})
		if err == nil {
			return nil
		}

		if name == cmdDisconnect {
			return fmt.Errorf("cli: disconnect command: %w", err)
		}

		return fmt.Errorf("cli: uninstall command: %w", err)
	}
}

func usage(ctx context.Context, run *Runner) {
	err := writeUsage(run.Err)
	if err != nil {
		run.log.ErrorContext(ctx, msgFailed, slog.String(keyError, err.Error()))
	}
}

func writeUsage(writer io.Writer) error {
	written, err := io.WriteString(writer, usageText)
	if err != nil {
		return fmt.Errorf("cli: write usage: %w", err)
	}

	if written != len(usageText) {
		return errShortWrite
	}

	return nil
}

func report(ctx context.Context, run *Runner, err error) int {
	var usageErr *usageError

	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case errors.As(err, &usageErr):
		return reportUsage(ctx, run, usageErr)
	default:
		return reportFailure(ctx, run, err)
	}
}

func reportFailure(ctx context.Context, run *Runner, err error) int {
	switch {
	case errors.Is(err, meshagent.ErrNotElevated):
		run.log.ErrorContext(ctx, msgNeedElevation)

		return ExitDenied
	case errors.Is(err, meshagent.ErrDownloadUnauthorized):
		run.log.ErrorContext(ctx, msgUnauthorized, slog.String(keySupply, EnvCookie))

		return ExitDenied
	default:
		return reportRemaining(ctx, run, err)
	}
}

func reportRemaining(ctx context.Context, run *Runner, err error) int {
	switch {
	case errors.Is(err, meshagent.ErrNotInstalled):
		run.log.ErrorContext(ctx, msgNotInstalled, slog.String(keyFix, fixConnect))
	case errors.Is(err, meshagent.ErrUnsupportedPlatform):
		run.log.ErrorContext(ctx, msgUnsupported)
	case errors.Is(err, context.Canceled):
		run.log.ErrorContext(ctx, msgCanceled)
	default:
		run.log.ErrorContext(ctx, msgFailed, slog.String(keyError, err.Error()))
	}

	return ExitError
}

func reportUsage(ctx context.Context, run *Runner, usageErr *usageError) int {
	if !usageErr.printed {
		run.log.ErrorContext(ctx, msgBadCommandLine, slog.String(keyError, usageErr.Error()))
		usage(ctx, run)
	}

	return ExitUsage
}

func flagError(err error) error {
	return &usageError{err: err, printed: true}
}

func usageDetail(detail string) error {
	return &usageError{err: fmt.Errorf("%w: %s", errUsage, detail)}
}

func ensureSeams(ctx context.Context, run *Runner, opts *Options) error {
	if run.Agent != nil {
		bindInstaller(run)

		return nil
	}

	err := wireFromOptions(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: wire options: %w", err)
	}

	return nil
}

func wireFromOptions(ctx context.Context, run *Runner, opts *Options) error {
	err := requireOptions(opts)
	if err != nil {
		return fmt.Errorf("cli: options: %w", err)
	}

	err = wireClient(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: wire client: %w", err)
	}

	return nil
}

func bindInstaller(run *Runner) {
	if run.Installer != nil {
		return
	}

	installer, ok := run.Agent.(Installer)
	if ok {
		run.Installer = installer
	}
}

func wireClient(_ context.Context, run *Runner, opts *Options) error {
	client, err := newMeshClient(opts)
	if err != nil {
		return fmt.Errorf("cli: client: %w", err)
	}

	run.Agent = client
	run.Installer = client

	return nil
}

func newMeshClient(opts *Options) (*meshagent.Client, error) {
	client, err := meshagent.New(&meshagent.Config{
		ServerURL:    opts.Server,
		GroupID:      opts.Group,
		ServiceName:  opts.ServiceName,
		AuthCookie:   os.Getenv(EnvCookie),
		Arch:         archOrDetect(opts.Arch),
		InstallFlags: opts.InstallFlags,
	})
	if err != nil {
		return nil, fmt.Errorf("cli: new client: %w", usageDetail(err.Error()))
	}

	return client, nil
}

func archOrDetect(arch int) int {
	if arch == archDetect {
		return ExitOK
	}

	return arch
}

func requireOptions(opts *Options) error {
	if opts.Server == emptyValue {
		return fmt.Errorf(
			errFmtRequire,
			usageDetail(fmt.Sprintf("-server is required (or set %s)", EnvServer)),
		)
	}

	if opts.Group == emptyValue {
		return fmt.Errorf(
			errFmtRequire,
			usageDetail(fmt.Sprintf("-group is required (or set %s)", EnvGroup)),
		)
	}

	if opts.ServiceName == emptyValue {
		return fmt.Errorf(errFmtRequire, usageDetail("-service-name must name a service"))
	}

	return nil
}

func newFlagSet(name string, opts *Options) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)

	set.StringVar(&opts.ServiceName, "service-name", meshagent.DefaultServiceName,
		"Windows service name to control")
	set.DurationVar(&opts.Timeout, "timeout", defaultTimeout, "overall timeout")

	return set
}

func newServerFlagSet(name string, opts *Options) *flag.FlagSet {
	set := newFlagSet(name, opts)

	set.StringVar(&opts.Server, "server", os.Getenv(EnvServer),
		"MeshCentral server URL, for example https://mesh.example.com")
	set.StringVar(&opts.Group, "group", os.Getenv(EnvGroup),
		"device group id (the gotomesh value in the group's web URL)")
	set.IntVar(&opts.Arch, "arch", archDetect,
		"agent architecture: 3 x86-32, 4 x86-64, 43 arm64 (default: detect)")
	set.StringVar(&opts.InstallFlags, "install-flags", string(meshagent.FlagBackgroundOnly),
		"agent dialog options: background, interactive, or both")
	set.StringVar(&opts.Dir, "dir", emptyValue,
		"directory to download the agent into (default: a temporary directory)")

	return set
}

func parseInto(set *flag.FlagSet, args []string) error {
	err := set.Parse(args)
	if err != nil {
		return fmt.Errorf("cli: parse flags: %w", flagError(err))
	}

	return nil
}

func connect(ctx context.Context, run *Runner, args []string) error {
	opts := &Options{Arch: archDetect}

	err := parseInto(newServerFlagSet(cmdConnect, opts), args)
	if err != nil {
		return fmt.Errorf("cli: connect flags: %w", err)
	}

	err = connectAgent(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: connect: %w", err)
	}

	return nil
}

func connectAgent(ctx context.Context, run *Runner, opts *Options) error {
	err := ensureSeams(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: resolve: %w", err)
	}

	timed, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	err = ensureRunning(timed, run, opts)
	if err != nil {
		return fmt.Errorf("cli: ensure running: %w", err)
	}

	return nil
}

func ensureRunning(ctx context.Context, run *Runner, opts *Options) error {
	err := ensureInstalled(ctx, run, opts)
	if err != nil {
		return fmt.Errorf(errFmtEnsureInstalled, err)
	}

	err = startAgent(ctx, run)
	if err != nil {
		return fmt.Errorf("cli: start: %w", err)
	}

	return nil
}

func ensureInstalled(ctx context.Context, run *Runner, opts *Options) error {
	err := withAgent(run, func(agent Agent) error {
		status, statusErr := agent.Status(ctx)
		if statusErr != nil {
			return fmt.Errorf(errReadStatus, statusErr)
		}

		if status.Installed {
			return nil
		}

		return install(ctx, run, opts)
	})
	if err != nil {
		return fmt.Errorf(errFmtEnsureInstalled, err)
	}

	return nil
}

func startAgent(ctx context.Context, run *Runner) error {
	err := withAgent(run, func(agent Agent) error {
		connectErr := agent.Connect(ctx)
		if connectErr != nil {
			return fmt.Errorf("cli: connect service: %w", connectErr)
		}

		run.log.InfoContext(ctx, msgConnected, slog.String(keyService, agent.ServiceName()))

		return nil
	})
	if err != nil {
		return fmt.Errorf("cli: start agent: %w", err)
	}

	return nil
}

func install(ctx context.Context, run *Runner, opts *Options) error {
	destDir, cleanup, err := staging(opts.Dir)
	if err != nil {
		return fmt.Errorf("cli: staging: %w", err)
	}

	if cleanup != nil {
		defer cleanup()
	}

	err = downloadAndRegister(ctx, run, destDir)
	if err != nil {
		return fmt.Errorf(errFmtDownloadRegister, err)
	}

	return nil
}

func downloadAndRegister(ctx context.Context, run *Runner, destDir string) error {
	run.log.InfoContext(ctx, msgDownloading)

	err := withAgent(run, func(agent Agent) error {
		exePath, downloadErr := agent.Download(ctx, destDir)
		if downloadErr != nil {
			return fmt.Errorf("cli: download: %w", downloadErr)
		}

		return register(ctx, run, exePath)
	})
	if err != nil {
		return fmt.Errorf(errFmtDownloadRegister, err)
	}

	return nil
}

func register(ctx context.Context, run *Runner, exePath string) error {
	err := withInstaller(run, func(installer Installer) error {
		run.log.InfoContext(ctx, msgInstalling, slog.String(keyFrom, exePath))

		installErr := installer.Install(ctx, exePath)
		if installErr != nil {
			return fmt.Errorf("cli: install: %w", installErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("cli: register: %w", err)
	}

	return nil
}

func staging(dir string) (path string, cleanup func(), err error) {
	result, err := chooseStaging(dir)
	if err != nil {
		return emptyValue, nil, fmt.Errorf("cli: choose staging: %w", err)
	}

	return result.dir, result.cleanup, nil
}

func chooseStaging(dir string) (stagingResult, error) {
	if dir != emptyValue {
		return stagingResult{dir: dir, cleanup: func() {}}, nil
	}

	result, err := tempStaging()
	if err != nil {
		return stagingResult{}, fmt.Errorf("cli: temp staging: %w", err)
	}

	return result, nil
}

func tempStaging() (stagingResult, error) {
	temp, err := os.MkdirTemp(emptyValue, stagingPattern)
	if err != nil {
		return stagingResult{}, fmt.Errorf("cli: create staging directory: %w", err)
	}

	return stagingResult{dir: temp, cleanup: removeStaging(temp)}, nil
}

func removeStaging(temp string) func() {
	return func() {
		reportRemove(os.RemoveAll(temp))
	}
}

func reportRemove(err error) {
	if err == nil {
		return
	}
}

func runLocal(ctx context.Context, run *Runner, req *struct {
	name string
	args []string
},
) error {
	opts, err := prepareLocal(ctx, run, req)
	if err != nil {
		return fmt.Errorf("cli: prepare local: %w", err)
	}

	err = finishLocal(ctx, run, &struct {
		opts *Options
		name string
	}{opts: opts, name: req.name})
	if err != nil {
		return fmt.Errorf("cli: finish local: %w", err)
	}

	return nil
}

func prepareLocal(ctx context.Context, run *Runner, req *struct {
	name string
	args []string
},
) (*Options, error) {
	opts, err := localOptions(req.name, req.args)
	if err != nil {
		return nil, fmt.Errorf("cli: local flags: %w", err)
	}

	err = ensureSeams(ctx, run, opts)
	if err != nil {
		return nil, fmt.Errorf("cli: resolve local: %w", err)
	}

	return opts, nil
}

func finishLocal(ctx context.Context, run *Runner, work *struct {
	opts *Options
	name string
},
) error {
	timed, cancel := context.WithTimeout(ctx, work.opts.Timeout)
	defer cancel()

	err := applyLocal(timed, run, work.name)
	if err != nil {
		return fmt.Errorf(errFmtApplyLocal, err)
	}

	logLocalDone(ctx, run, work.name)

	return nil
}

func applyLocal(ctx context.Context, run *Runner, name string) error {
	err := localOp(name)(ctx, run)
	if err != nil {
		return fmt.Errorf(errFmtApplyLocal, err)
	}

	return nil
}

func localOp(name string) func(context.Context, *Runner) error {
	if name == cmdUninstall {
		return removeAgent
	}

	return stopAgent
}

func stopAgent(ctx context.Context, run *Runner) error {
	err := withAgent(run, func(agent Agent) error {
		disconnectErr := agent.Disconnect(ctx)
		if disconnectErr != nil {
			return fmt.Errorf("cli: disconnect: %w", disconnectErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("cli: stop: %w", err)
	}

	return nil
}

func removeAgent(ctx context.Context, run *Runner) error {
	err := withInstaller(run, uninstallOp(ctx))
	if err != nil {
		return fmt.Errorf("cli: remove: %w", err)
	}

	return nil
}

func uninstallOp(ctx context.Context) func(Installer) error {
	return func(installer Installer) error {
		uninstallErr := installer.Uninstall(ctx)
		if uninstallErr != nil {
			return fmt.Errorf("cli: uninstall: %w", uninstallErr)
		}

		return nil
	}
}

func localOptions(name string, args []string) (*Options, error) {
	opts := &Options{Server: placeholderServer, Group: placeholderGroup}

	err := parseInto(newFlagSet(name, opts), args)
	if err != nil {
		return nil, fmt.Errorf("cli: local options: %w", err)
	}

	return opts, nil
}

func logLocalDone(ctx context.Context, run *Runner, name string) {
	service := serviceLabel(run)

	switch name {
	case cmdDisconnect:
		run.log.InfoContext(ctx, msgDisconnected, slog.String(keyService, service))
	case cmdUninstall:
		run.log.InfoContext(ctx, msgUninstalled, slog.String(keyService, service))
	default:
	}
}

func serviceLabel(run *Runner) string {
	agent, ok := run.Agent.(Agent)
	if !ok {
		return emptyValue
	}

	return agent.ServiceName()
}

func statusCmd(ctx context.Context, run *Runner, args []string) error {
	opts := &Options{Server: placeholderServer, Group: placeholderGroup}
	set := newFlagSet(cmdStatus, opts)
	set.BoolVar(&opts.AsJSON, "json", false, "print the status as JSON")

	err := parseInto(set, args)
	if err != nil {
		return fmt.Errorf("cli: status flags: %w", err)
	}

	err = showStatus(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: status: %w", err)
	}

	return nil
}

func showStatus(ctx context.Context, run *Runner, opts *Options) error {
	err := ensureSeams(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: resolve status: %w", err)
	}

	err = renderStatus(ctx, run, opts)
	if err != nil {
		return fmt.Errorf("cli: render: %w", err)
	}

	return nil
}

func renderStatus(ctx context.Context, run *Runner, opts *Options) error {
	timed, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	err := readAndEmit(timed, run, opts)
	if err != nil {
		return fmt.Errorf("cli: render status: %w", err)
	}

	return nil
}

func readAndEmit(ctx context.Context, run *Runner, opts *Options) error {
	err := withAgent(run, func(agent Agent) error {
		pair := &struct {
			opts  *Options
			agent Agent
		}{opts: opts, agent: agent}

		err := publishStatus(ctx, run, pair)
		if err != nil {
			return fmt.Errorf("cli: publish status: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("cli: read status: %w", err)
	}

	return nil
}

func publishStatus(ctx context.Context, run *Runner, pair *struct {
	opts  *Options
	agent Agent
},
) error {
	status, err := pair.agent.Status(ctx)
	if err != nil {
		return fmt.Errorf(errReadStatus, err)
	}

	err = emitStatus(ctx, run, &struct {
		opts   *Options
		agent  Agent
		status meshagent.Status
	}{opts: pair.opts, agent: pair.agent, status: status})
	if err != nil {
		return fmt.Errorf("cli: emit status: %w", err)
	}

	return nil
}

func emitStatus(ctx context.Context, run *Runner, view *struct {
	opts   *Options
	agent  Agent
	status meshagent.Status
},
) error {
	if !view.opts.AsJSON {
		writeTextStatus(ctx, run, view)

		return nil
	}

	err := writeJSONStatus(run, &view.status)
	if err != nil {
		return fmt.Errorf("cli: json status: %w", err)
	}

	return nil
}

func writeJSONStatus(run *Runner, status *meshagent.Status) error {
	err := encode(run, status)
	if err != nil {
		return fmt.Errorf(errFmtEncode, err)
	}

	return nil
}

func writeTextStatus(ctx context.Context, run *Runner, view *struct {
	opts   *Options
	agent  Agent
	status meshagent.Status
},
) {
	run.log.InfoContext(ctx, msgStatus,
		slog.String(keyService, view.agent.ServiceName()),
		slog.Bool(keyInstalled, view.status.Installed),
		slog.String(keyState, view.status.State),
		slog.String(keyPath, view.status.BinaryPath),
	)
}

func encode(run *Runner, status *meshagent.Status) error {
	encoder := json.NewEncoder(run.Out)
	encoder.SetIndent(emptyValue, "  ")

	err := encoder.Encode(status)
	if err != nil {
		return fmt.Errorf(errFmtEncode, err)
	}

	return nil
}

func withAgent(run *Runner, call func(Agent) error) error {
	agent, ok := run.Agent.(Agent)
	if !ok {
		return errNoAgent
	}

	err := call(agent)
	if err != nil {
		return fmt.Errorf(errFmtAgent, err)
	}

	return nil
}

func withInstaller(run *Runner, call func(Installer) error) error {
	if run.Installer == nil {
		return errNoInstaller
	}

	installer, ok := run.Installer.(Installer)
	if !ok {
		return errNoInstaller
	}

	err := call(installer)
	if err != nil {
		return fmt.Errorf(errFmtInstaller, err)
	}

	return nil
}
