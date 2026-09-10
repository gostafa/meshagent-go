// Command meshagentctl manages the MeshCentral agent on this machine.
//
// The connect command is the one to reach for: it downloads and installs the
// agent if it is not present, then makes sure the service is running. It is
// safe to run repeatedly.
//
//	meshagentctl connect    -server https://mesh.example.com -group <meshid>
//	meshagentctl disconnect
//	meshagentctl status     -json
//	meshagentctl uninstall
//
// Install and uninstall require an elevated process. Status does not.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	meshagent "github.com/gostafa/meshagent-go"
)

const (
	exitOK     = 0
	exitError  = 1
	exitUsage  = 2
	exitDenied = 3
)

// Environment fallbacks, so the group id and especially the session cookie do
// not have to appear in a command line that other users can read from the
// process table.
const (
	envServer = "MESHAGENT_SERVER"
	envGroup  = "MESHAGENT_GROUP"
	envCookie = "MESHAGENT_COOKIE"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()

		return exitUsage
	}

	command := args[0]
	if command == "-h" || command == "--help" || command == "help" {
		usage()

		return exitOK
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error

	switch command {
	case "connect":
		err = runConnect(ctx, args[1:])
	case "disconnect":
		err = runDisconnect(ctx, args[1:])
	case "status":
		err = runStatus(ctx, args[1:])
	case "uninstall":
		err = runUninstall(ctx, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "meshagentctl: unknown command %q\n\n", command)
		usage()

		return exitUsage
	}

	return report(err)
}

// usageError marks a problem with the command line rather than with the
// operation, so it exits with exitUsage instead of exitError.
//
// printed records whether the details already reached stderr: the flag package
// writes its own message and usage text when parsing fails, and repeating it
// would be noise.
type usageError struct {
	err     error
	printed bool
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// flagError wraps a failure from flag.FlagSet.Parse, which has already
// reported itself.
func flagError(err error) error {
	return &usageError{err: err, printed: true}
}

// usagef reports a command line problem the flag package does not catch, such
// as a required value being absent or a value being out of range.
func usagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// report turns an error into an exit code, translating the failures a caller
// can actually act on into specific messages.
func report(err error) int {
	var usageErr *usageError

	switch {
	case err == nil:
		return exitOK

	case errors.Is(err, flag.ErrHelp):
		return exitOK

	case errors.As(err, &usageErr):
		if !usageErr.printed {
			fmt.Fprintf(os.Stderr, "meshagentctl: %v\n\n", usageErr)
			usage()
		}

		return exitUsage

	case errors.Is(err, meshagent.ErrNotElevated):
		fmt.Fprintln(os.Stderr, "meshagentctl: this command needs an elevated process.")
		fmt.Fprintln(os.Stderr, "  Run it from an Administrator prompt.")

		return exitDenied

	case errors.Is(err, meshagent.ErrDownloadUnauthorized):
		fmt.Fprintln(os.Stderr, "meshagentctl: the server refused an anonymous agent download.")
		fmt.Fprintf(os.Stderr, "  It has lockagentdownload enabled; supply a session cookie via %s.\n", envCookie)

		return exitDenied

	case errors.Is(err, meshagent.ErrNotInstalled):
		fmt.Fprintln(os.Stderr, "meshagentctl: the mesh agent is not installed on this machine.")
		fmt.Fprintln(os.Stderr, "  Run: meshagentctl connect -server <url> -group <meshid>")

		return exitError

	case errors.Is(err, meshagent.ErrUnsupportedPlatform):
		fmt.Fprintln(os.Stderr, "meshagentctl: this command is only supported on windows.")

		return exitError

	case errors.Is(err, context.Canceled):
		fmt.Fprintln(os.Stderr, "meshagentctl: cancelled.")

		return exitError

	default:
		fmt.Fprintf(os.Stderr, "meshagentctl: %v\n", err)

		return exitError
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `meshagentctl manages the MeshCentral agent on this machine.

Usage:
  meshagentctl <command> [flags]

Commands:
  connect      Install the agent if needed, then start it. Safe to re-run.
  disconnect   Stop the agent. It stays installed and the device stays
               registered in its device group, showing as offline.
  status       Report whether the agent is installed and running.
  uninstall    Stop the agent, remove the service and delete its files.

Environment:
  MESHAGENT_SERVER   default for -server
  MESHAGENT_GROUP    default for -group
  MESHAGENT_COOKIE   session cookie for servers with lockagentdownload enabled

Run "meshagentctl <command> -h" for the flags of a command.
`)
}

// options collects the flags shared by the commands.
type options struct {
	server      string
	group       string
	serviceName string
	installFlag string
	arch        int
	dir         string
	timeout     time.Duration
	asJSON      bool
}

// newFlagSet registers the flags a command needs. Commands that only touch the
// local service manager do not need server details, so they pass needsServer
// false and get a smaller, honest help output.
func newFlagSet(name string, opts *options, needsServer bool) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)

	fs.StringVar(&opts.serviceName, "service-name", meshagent.DefaultServiceName,
		"Windows service name to control")
	fs.DurationVar(&opts.timeout, "timeout", 5*time.Minute,
		"overall timeout for the command")

	if needsServer {
		fs.StringVar(&opts.server, "server", os.Getenv(envServer),
			"MeshCentral server URL, for example https://mesh.example.com")
		fs.StringVar(&opts.group, "group", os.Getenv(envGroup),
			"device group id (the gotomesh value in the group's web URL)")
		fs.IntVar(&opts.arch, "arch", 0,
			"agent architecture: 3 x86-32, 4 x86-64, 43 arm64 (default: detect)")
		fs.StringVar(&opts.installFlag, "install-flags", "background",
			"agent dialog options: background, interactive, or both")
		fs.StringVar(&opts.dir, "dir", "",
			"directory to download the agent into (default: a temporary directory)")
	}

	return fs
}

// client builds a meshagent.Client from the parsed flags.
func (o *options) client() (*meshagent.Client, error) {
	if strings.TrimSpace(o.server) == "" {
		return nil, usagef("-server is required (or set %s)", envServer)
	}

	if strings.TrimSpace(o.group) == "" {
		return nil, usagef("-group is required (or set %s)", envGroup)
	}

	flags, err := parseInstallFlags(o.installFlag)
	if err != nil {
		return nil, err
	}

	return meshagent.New(meshagent.Config{
		ServerURL:    o.server,
		GroupID:      o.group,
		Arch:         meshagent.Arch(o.arch),
		InstallFlags: flags,
		ServiceName:  o.serviceName,
		AuthCookie:   os.Getenv(envCookie),
	})
}

// localClient builds a Client for commands that only touch the local service
// manager. Those need no server or group, but meshagent.Config requires both,
// so placeholders stand in for fields that are never read.
func (o *options) localClient() (*meshagent.Client, error) {
	return meshagent.New(meshagent.Config{
		ServerURL:   "https://localhost",
		GroupID:     "unused",
		ServiceName: o.serviceName,
	})
}

func parseInstallFlags(value string) (meshagent.InstallFlags, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "background":
		return meshagent.FlagBackgroundOnly, nil
	case "interactive":
		return meshagent.FlagInteractiveOnly, nil
	case "both":
		return meshagent.FlagInteractiveAndBackground, nil
	default:
		return 0, usagef("invalid -install-flags %q: want background, interactive or both", value)
	}
}

func parseArch(value int) error {
	if value != 0 && !meshagent.Arch(value).Valid() {
		return usagef("invalid -arch %d: want 3, 4 or 43", value)
	}

	return nil
}

// runConnect installs the agent if it is absent, then makes sure it is running.
func runConnect(ctx context.Context, args []string) error {
	var opts options

	fs := newFlagSet("connect", &opts, true)
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}

	if err := parseArch(opts.arch); err != nil {
		return err
	}

	client, err := opts.client()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		return err
	}

	if !status.Installed {
		if err := install(ctx, client, opts); err != nil {
			return err
		}
	}

	// -fullinstall starts the service itself, and Connect is a no-op on an
	// already-running service, so this is correct in both branches.
	if err := client.Connect(ctx); err != nil {
		return err
	}

	final, err := client.Status(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("connected: service %q is %s\n", client.ServiceName(), final.State)

	return nil
}

// install downloads the agent and registers it.
//
// When the caller did not choose a download directory the binary is transient —
// the installer copies itself into its own install location — so it is removed
// afterwards. A caller-supplied -dir is left alone, since they asked for the
// file to land somewhere specific.
func install(ctx context.Context, client *meshagent.Client, opts options) error {
	destDir := opts.dir
	cleanup := false

	if destDir == "" {
		staging, err := os.MkdirTemp("", "meshagent-*")
		if err != nil {
			return fmt.Errorf("create staging directory: %w", err)
		}

		destDir = staging
		cleanup = true

		defer os.RemoveAll(staging)
	}

	fmt.Fprintf(os.Stderr, "downloading %s agent...\n", client.Arch())

	exe, err := client.Download(ctx, destDir)
	if err != nil {
		return err
	}

	if !cleanup {
		fmt.Fprintf(os.Stderr, "saved %s\n", exe)
	}

	fmt.Fprintln(os.Stderr, "installing...")

	if err := client.Install(ctx, exe); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "installed from %s\n", filepath.Base(exe))

	return nil
}

// runDisconnect stops the agent, leaving it installed.
func runDisconnect(ctx context.Context, args []string) error {
	var opts options

	fs := newFlagSet("disconnect", &opts, false)
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}

	client, err := opts.localClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	if err := client.Disconnect(ctx); err != nil {
		return err
	}

	fmt.Printf("disconnected: service %q is stopped, the device stays registered\n", client.ServiceName())

	return nil
}

// runStatus reports the local service state.
func runStatus(ctx context.Context, args []string) error {
	var opts options

	fs := newFlagSet("status", &opts, false)
	fs.BoolVar(&opts.asJSON, "json", false, "print the status as JSON")

	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}

	client, err := opts.localClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		return err
	}

	if opts.asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")

		return encoder.Encode(status)
	}

	if !status.Installed {
		fmt.Printf("not installed (service %q)\n", client.ServiceName())

		return nil
	}

	fmt.Printf("installed: %s\n", status.BinaryPath)
	fmt.Printf("service:   %s (%s)\n", client.ServiceName(), status.State)

	return nil
}

// runUninstall removes the agent entirely.
func runUninstall(ctx context.Context, args []string) error {
	var opts options

	fs := newFlagSet("uninstall", &opts, false)
	if err := fs.Parse(args); err != nil {
		return flagError(err)
	}

	client, err := opts.localClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	if err := client.Uninstall(ctx); err != nil {
		return err
	}

	fmt.Printf("uninstalled: service %q removed\n", client.ServiceName())
	fmt.Fprintln(os.Stderr, "note: the device record is left in the MeshCentral device group; remove it there if wanted")

	return nil
}
