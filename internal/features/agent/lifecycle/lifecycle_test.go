// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package lifecycle

import (
	"encoding/json"
	"testing"
)

// TestStatusJSONContract pins the wire names, which the command line prints
// with its -json flag and other tools parse.
func TestStatusJSONContract(t *testing.T) {
	status := Status{
		State:      "running",
		BinaryPath: `C:\Program Files\Mesh Agent\MeshAgent.exe`,
		Installed:  true,
		Running:    true,
	}

	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"state", "binary_path", "installed", "running"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("encoded status is missing %q: %s", key, encoded)
		}
	}
}
