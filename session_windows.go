//go:build windows

package meshagent

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// temporaryCapability is the agent capability bit meaning "temporary agent".
// MeshCentral deletes the device record, its network interface information,
// notes, last-connect time and system information when an agent announcing this
// bit disconnects.
const temporaryCapability = "0x00000020"

// Session is a running temporary agent connection.
//
// A session is deliberately not a managed install: the device it creates in
// MeshCentral is removed by the server the moment the session ends. Use
// Install with Connect and Disconnect for a device that should persist.
type Session struct {
	cmd     *exec.Cmd
	job     windows.Handle
	done    chan struct{}
	waitErr error

	stopOnce sync.Once
	stopErr  error
}

// StartSession runs the agent binary at exePath as a temporary connection to
// this Client's device group, and returns once the process has started.
//
// The device appears in MeshCentral for as long as the session runs and is
// deleted server-side when it ends. See Session.
//
// ctx bounds fetching the device group settings from the server, not the
// lifetime of the session; the returned Session keeps running until Stop is
// called or the process exits on its own.
func (c *Client) StartSession(ctx context.Context, exePath string) (*Session, error) {
	settings, err := c.Settings(ctx)
	if err != nil {
		return nil, err
	}

	if err := settings.Required(); err != nil {
		return nil, err
	}

	cmd := exec.Command(exePath, sessionArgs(settings)...)

	// Kill the whole process tree when the session ends. MeshAgent spawns
	// helper processes, so terminating only the process we started leaks them.
	job, err := newKillOnCloseJob()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)

		return nil, fmt.Errorf("meshagent: start temporary session: %w", err)
	}

	if err := assignToJob(job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		windows.CloseHandle(job)

		return nil, err
	}

	session := &Session{
		cmd:  cmd,
		job:  job,
		done: make(chan struct{}),
	}

	go func() {
		session.waitErr = cmd.Wait()
		close(session.done)
	}()

	return session, nil
}

// sessionArgs builds the command line for a temporary connection.
//
// --no-embedded=1 makes the binary ignore the device group settings embedded in
// it by the server, so the values below take effect instead. Values are wrapped
// in literal quotes to match how MeshCentral's own installer invokes the agent;
// the agent strips them.
func sessionArgs(settings Settings) []string {
	args := []string{
		"--no-embedded=1",
		"--disableUpdate=1",
		quotedArg("MeshID", settings["MeshID"]),
		quotedArg("ServerID", settings["ServerID"]),
		quotedArg("MeshServer", settings["MeshServer"]),
		quotedArg("AgentCapabilities", temporaryCapability),
	}

	for _, key := range []string{"MeshName", "MeshType", "displayName", "agentName"} {
		if value := settings[key]; value != "" {
			args = append(args, quotedArg(key, value))
		}
	}

	return args
}

// quotedArg renders --name="value". The quotes are literal rather than Go's %q
// escaping, because the agent expects the same byte sequence MeshCentral's own
// installer passes it and does not understand Go-style escapes.
func quotedArg(name, value string) string {
	return fmt.Sprintf(`--%s="%s"`, name, value)
}

// newKillOnCloseJob creates a job object that terminates every process in it
// once the last handle to the job is closed.
func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("meshagent: create job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)

		return 0, fmt.Errorf("meshagent: configure job object: %w", err)
	}

	return job, nil
}

// assignToJob puts an already-started process into a job object.
//
// There is a small window between the process starting and being assigned in
// which it could spawn a child that escapes the job. Closing that window needs
// CREATE_SUSPENDED and a thread handle, which os/exec does not expose; the
// agent does not spawn helpers that early, so the race is not reachable in
// practice.
func assignToJob(job windows.Handle, pid int) error {
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return fmt.Errorf("meshagent: open session process: %w", err)
	}
	defer windows.CloseHandle(process)

	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return fmt.Errorf("meshagent: assign session process to job object: %w", err)
	}

	return nil
}

// Pid returns the process id of the session.
func (s *Session) Pid() int {
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}

	return s.cmd.Process.Pid
}

// Done returns a channel closed when the session process exits, whether it was
// stopped or exited on its own.
func (s *Session) Done() <-chan struct{} { return s.done }

// Wait blocks until the session process exits and returns its exit error, if
// any. It is safe to call from multiple goroutines.
func (s *Session) Wait() error {
	<-s.done

	return s.waitErr
}

// Stop terminates the session and every process it spawned, then waits for it
// to exit.
//
// The device this session created is deleted from MeshCentral as a result. Stop
// is idempotent.
func (s *Session) Stop() error {
	s.stopOnce.Do(func() {
		// Closing the last job handle terminates every process in the job.
		if err := windows.CloseHandle(s.job); err != nil {
			s.stopErr = fmt.Errorf("meshagent: close job object: %w", err)

			return
		}

		<-s.done
	})

	return s.stopErr
}
