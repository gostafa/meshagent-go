// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package winscm talks to the Windows service control manager.
//
// It answers the agentservice.Ops seam and the agentinstall.Locator seam. The
// service control manager calls sit behind the Syscalls seam so that the
// mapping from Windows errors onto the module's sentinels is covered by tests
// on the Windows runner, where a real service is not available.
//
// Off Windows every operation reports agenterr.ErrUnsupportedPlatform.
package winscm
