// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package artifact

import (
	"errors"
	"runtime"
	"testing"

	"github.com/gostafa/meshagent-go/internal/shared/agenterr"
)

// invalidArch and invalidFlags exercise the default arm of each exhaustive
// switch, which the declared members cannot reach.
//
// These are vars rather than consts on purpose: a typed constant of an enum
// type counts as a member of that enum, and the exhaustive check would then
// demand a case for it in every switch over Arch or InstallFlags.
var (
	invalidArch  = Arch(99)
	invalidFlags = InstallFlags("sideways")
)

func TestArchValid(t *testing.T) {
	for _, arch := range []Arch{ArchWindows32, ArchWindows64, ArchWindowsARM64} {
		if !arch.Valid() {
			t.Errorf("Arch(%d).Valid() = false, want true", int(arch))
		}
	}

	if invalidArch.Valid() {
		t.Error("Arch(99).Valid() = true, want false")
	}
}

func TestArchString(t *testing.T) {
	tests := map[Arch]string{
		ArchWindows32:    archName32,
		ArchWindows64:    archName64,
		ArchWindowsARM64: archNameARM64,
		invalidArch:      archNameUnknown,
	}

	for arch, want := range tests {
		if got := arch.String(); got != want {
			t.Errorf("Arch(%d).String() = %q, want %q", int(arch), got, want)
		}
	}
}

func TestArchBinaryName(t *testing.T) {
	tests := map[Arch]string{
		ArchWindows32:    binaryName32,
		ArchWindows64:    binaryName64,
		ArchWindowsARM64: binaryNameARM64,
		invalidArch:      binaryName64,
	}

	for arch, want := range tests {
		if got := arch.BinaryName(); got != want {
			t.Errorf("Arch(%d).BinaryName() = %q, want %q", int(arch), got, want)
		}
	}
}

func TestArchForGOARCH(t *testing.T) {
	tests := map[string]Arch{
		archGOARCH32:    ArchWindows32,
		archGOARCHARM64: ArchWindowsARM64,
		"amd64":         ArchWindows64,
		"riscv64":       ArchWindows64,
	}

	for goarch, want := range tests {
		if got := archForGOARCH(goarch); got != want {
			t.Errorf("archForGOARCH(%q) = %v, want %v", goarch, got, want)
		}
	}
}

func TestDetectArch(t *testing.T) {
	if got := DetectArch(); got != archForGOARCH(runtime.GOARCH) {
		t.Errorf("DetectArch() = %v, want the mapping for GOARCH %q", got, runtime.GOARCH)
	}
}

func TestInstallFlagsWire(t *testing.T) {
	tests := map[InstallFlags]int{
		FlagUnset:                    wireBackground,
		FlagBackgroundOnly:           wireBackground,
		FlagInteractiveOnly:          wireInteractive,
		FlagInteractiveAndBackground: wireBoth,
		invalidFlags:                 wireBackground,
	}

	for flags, want := range tests {
		if got := flags.Wire(); got != want {
			t.Errorf("InstallFlags(%q).Wire() = %d, want %d", string(flags), got, want)
		}
	}
}

func TestInstallFlagsValid(t *testing.T) {
	valid := []InstallFlags{
		FlagUnset,
		FlagInteractiveAndBackground,
		FlagInteractiveOnly,
		FlagBackgroundOnly,
	}

	for _, flags := range valid {
		if !flags.Valid() {
			t.Errorf("InstallFlags(%q).Valid() = false, want true", string(flags))
		}
	}

	if invalidFlags.Valid() {
		t.Error("InstallFlags(\"sideways\").Valid() = true, want false")
	}
}

func TestSettingsRequired(t *testing.T) {
	complete := Settings{
		KeyMeshID:     "0x1",
		KeyServerID:   "0x2",
		KeyMeshServer: "wss://host/agent.ashx",
	}

	if err := complete.Required(); err != nil {
		t.Fatalf("Required() on complete settings: %v", err)
	}

	// Whitespace must not satisfy a required key.
	partial := Settings{KeyMeshID: "0x1", KeyServerID: "   "}

	err := partial.Required()
	if !errors.Is(err, agenterr.ErrMissingSetting) {
		t.Fatalf("err = %v, want ErrMissingSetting", err)
	}
}

func TestParseSettings(t *testing.T) {
	settings := ParseSettings([]byte("MeshID=0x1\nServerID=0x2\nMeshServer=wss://h/a\n"))

	if err := settings.Required(); err != nil {
		t.Fatalf("Required() after ParseSettings: %v", err)
	}

	if settings[KeyMeshServer] != "wss://h/a" {
		t.Errorf("MeshServer = %q, want %q", settings[KeyMeshServer], "wss://h/a")
	}
}
