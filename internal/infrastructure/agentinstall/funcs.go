// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentinstall

import (
	"context"
	"fmt"

	"github.com/gostafa/meshagent-go/internal/features/agent/lifecycle"
	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

// New returns an Installer that runs the agent binary and refuses privileged
// operations without an elevated token.
func New(locator Locator) *Installer {
	return &Installer{
		runner:    &execRunner{outputLimit: defaultOutputLimit},
		locator:   locator,
		elevation: elevation{query: processToken{}},
	}
}

// Install registers the service from the binary at exePath and starts it.
//
// exePath must be a binary downloaded for the target device group: the server
// embeds the group's settings into the executable, so the installed service
// knows which server and group to join without further configuration.
func (ins *Installer) Install(ctx context.Context, exePath string) error {
	err := ins.elevation.require()
	if err != nil {
		return fmt.Errorf(errWrap, err)
	}

	err = ins.runner.run(ctx, exePath, SwitchInstall)
	if err != nil {
		return fmt.Errorf("agentinstall: install from %s: %w", exePath, err)
	}

	return nil
}

// Uninstall stops the service, removes it and deletes the installed files.
//
// Uninstalling is not the same as disconnecting: the device record in
// MeshCentral is left behind and must be removed from the server side.
func (ins *Installer) Uninstall(ctx context.Context) error {
	err := ins.elevation.require()
	if err != nil {
		return fmt.Errorf(errWrap, err)
	}

	err = ins.uninstallAt(ctx)
	if err != nil {
		return fmt.Errorf(errWrap, err)
	}

	return nil
}

// uninstallAt runs the uninstall switch against the installed binary.
func (ins *Installer) uninstallAt(ctx context.Context) error {
	exePath, err := ins.installedPath(ctx)
	if err != nil {
		return fmt.Errorf(errWrap, err)
	}

	err = ins.runner.run(ctx, exePath, SwitchUninstall)
	if err != nil {
		return fmt.Errorf("agentinstall: uninstall %s: %w", exePath, err)
	}

	return nil
}

// installedPath reports where the registered service was installed from.
func (ins *Installer) installedPath(ctx context.Context) (string, error) {
	status, err := ins.locator.Installed(ctx)
	if err != nil {
		return emptyPath, fmt.Errorf("agentinstall: locate installed agent: %w", err)
	}

	path, pathErr := installedBinary(&status)
	if pathErr != nil {
		return emptyPath, fmt.Errorf(errWrap, pathErr)
	}

	return path, nil
}

// installedBinary reads the path out of a located status.
func installedBinary(status *lifecycle.Status) (string, error) {
	if !status.Installed {
		return emptyPath, fmt.Errorf(errWrap, agenterr.ErrNotInstalled)
	}

	if status.BinaryPath == emptyPath {
		return emptyPath, fmt.Errorf(errWrap, agenterr.ErrUnknownServicePath)
	}

	return status.BinaryPath, nil
}

// require refuses early when the process cannot register services, so callers
// can tell "not an administrator" from "the installer failed".
func (elev elevation) require() error {
	if !elev.query.elevated() {
		return fmt.Errorf(errWrap, agenterr.ErrNotElevated)
	}

	return nil
}
