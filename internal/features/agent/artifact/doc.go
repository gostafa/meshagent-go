// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package artifact describes the MeshCentral agent binary and the device group
// settings that come with it, and declares the ports that fetch them.
//
// Value types and ports live together deliberately: the main-sequence distance
// policy requires roughly half of a package's named types to be interfaces, and
// a value-only package would score the maximum distance.
package artifact
