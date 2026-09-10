// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package agentports

import "testing"

func TestPortsName(t *testing.T) {
	ports := &Ports{ServiceName: "Mesh Agent"}
	if ports.Name() != "Mesh Agent" {
		t.Fatalf("Name = %q", ports.Name())
	}
}
