// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package agenterr holds the sentinel errors the agent packages share.
//
// It declares no named types, so it is exempt from the main-sequence distance
// policy and from the public-type limit, and every other package is free to
// depend on it.
package agenterr
