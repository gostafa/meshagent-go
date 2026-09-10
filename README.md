# meshagent-go

[![LICENSE](https://img.shields.io/github/license/task-otter/Taskotter)](/LICENSE) [![codecov](https://codecov.io/gh/task-otter/Taskotter/graph/badge.svg)](https://codecov.io/gh/task-otter/Taskotter)

Manage the official MeshCentral agent (MeshAgent) on Windows endpoints from Go:
download it from your server, install it as a service, and start or stop it.

This package **wraps the official binary**. It does not implement the MeshCentral
agent wire protocol — see [Why not a native Go agent](#why-not-a-native-go-agent).

```bash
go get github.com/gostafa/meshagent-go
```

## Command line

```bash
go build -o meshagentctl ./cmd/meshagentctl
```

`connect` is the "make it so" verb — it downloads and installs the agent if it is
absent, then makes sure the service is running. Re-running it is a no-op.

```bash
meshagentctl connect -server https://remote.example.com -group <meshid>
meshagentctl disconnect
meshagentctl status -json
meshagentctl uninstall
```

`connect` and `uninstall` need an Administrator prompt; `status` and
`disconnect` do not.

| Flag | Commands | Default |
|---|---|---|
| `-server` | connect | `$MESHAGENT_SERVER` |
| `-group` | connect | `$MESHAGENT_GROUP` |
| `-arch` | connect | detected (3, 4 or 43) |
| `-install-flags` | connect | `background` (`interactive`, `both`) |
| `-dir` | connect | a temporary directory, removed after install |
| `-service-name` | all | `Mesh Agent` |
| `-timeout` | all | `5m` |
| `-json` | status | off |

Set `MESHAGENT_COOKIE` rather than passing a session cookie as a flag — command
lines are readable by other users through the process table.

Exit codes: `0` success, `1` failure, `2` bad command line, `3` refused for lack
of privileges or credentials. `status -json` prints the `Status` struct, so the
tool is usable as a subprocess from another agent.

## Library usage

```go
client, err := meshagent.New(meshagent.Config{
    ServerURL: "https://remote.example.com",
    GroupID:   "the meshid from the group URL's gotomesh parameter",
})
if err != nil {
    return err
}

exe, err := client.Download(ctx, `C:\ProgramData\myapp`)
if err != nil {
    return err
}

if err := client.Install(ctx, exe); err != nil {
    return err
}
```

Then toggle remote access without touching the install:

```go
err := client.Disconnect(ctx) // service stops, device shows offline
err = client.Connect(ctx)     // service starts, device comes back online

status, err := client.Status(ctx)
// status.Installed, status.Running, status.State, status.BinaryPath
```

## The two connection models

They are not interchangeable.

| | Managed | Ad-hoc |
|---|---|---|
| API | `Install` + `Connect` / `Disconnect` | `StartSession` / `Session.Stop` |
| Mechanism | Windows service | Foreground process, capability `0x20` |
| On disconnect | Device stays registered, shows offline | **Server deletes the device record** |
| Survives reboot | Yes | No |
| Use for | A fleet you manage | One-off support on a machine you don't |

`StartSession` is the equivalent of the Connect button in the MeshAgent tray
dialog. When a temporary agent disconnects, MeshCentral removes the device row
along with its interface information, notes, last-connect time and system
information. If you use it and your device keeps vanishing, that is the design,
not a bug — use `Install` instead.

```go
session, err := client.StartSession(ctx, exe)
if err != nil {
    return err
}
defer session.Stop() // terminates the agent and every process it spawned

<-session.Done()
```

## Platform support

Downloading and reading device group settings work everywhere, so a build host or
provisioning server can stage binaries. Everything else is Windows-only and
returns `ErrUnsupportedPlatform` elsewhere, so the package builds and vets on any
host without build tags at the call site.

| API | Windows | Other |
|---|---|---|
| `Download`, `Settings` | yes | yes |
| `Install`, `Uninstall` | yes (elevated) | `ErrUnsupportedPlatform` |
| `Connect`, `Disconnect`, `Status`, `IsInstalled` | yes | `ErrUnsupportedPlatform` |
| `StartSession` | yes | `ErrUnsupportedPlatform` |

## Notes

**Elevation.** `Install` and `Uninstall` register and remove a Windows service.
They check the process token first and return `ErrNotElevated` rather than
failing with an opaque exit code. `Status` and `IsInstalled` deliberately open
the service control manager with `SC_MANAGER_CONNECT` only, so they work
unelevated.

**Service name.** Defaults to `Mesh Agent`. If your device group customises
`meshServiceName`, set `Config.ServiceName` to match or service control will not
find the agent.

**Locked downloads.** A server with `lockagentdownload` enabled rejects anonymous
agent downloads with 401, surfaced as `ErrDownloadUnauthorized`. Supply a
logged-in session via `Config.AuthCookie`.

**InstallFlags.** Defaults to `FlagBackgroundOnly`, which hides the Connect
button in the tray dialog — normally what unattended deployment wants. The Go
constants are not the wire values; the zero value means "unset" so the default
can apply.

**Download integrity.** The response is checked for an `MZ` header before being
renamed into place. MeshCentral can answer with an HTML error page under a 200
status, and a `Content-Length` check alone would not catch it.

## Why not a native Go agent

MeshCentral pushes `meshcore.js` down to the agent, which runs it in an embedded
JavaScript engine. Almost all agent functionality — terminal, KVM, file transfer,
the console commands — lives in that pushed core rather than in the binary. A Go
reimplementation would need the undocumented binary handshake on `/agent.ashx`
*and* the entire meshcore runtime, then track upstream changes to both
indefinitely.

This is the same conclusion TacticalRMM reached: it has a Go agent, and it ships
the stock C MeshAgent alongside it for remote control rather than reimplementing
it.

## License

TBD.
