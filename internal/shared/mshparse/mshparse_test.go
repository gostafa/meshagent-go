// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package mshparse

import "testing"

func TestPairs(t *testing.T) {
	const raw = "# comment\r\n\r\nMeshID=0xDEADBEEF\r\nnot-a-pair\r\n" +
		"  Spaced = value \r\nMeshServer=wss://host:443/agent.ashx\r\n"

	pairs := Pairs([]byte(raw))

	want := map[string]string{
		"MeshID":     "0xDEADBEEF",
		"Spaced":     "value",
		"MeshServer": "wss://host:443/agent.ashx",
	}

	if len(pairs) != len(want) {
		t.Fatalf("parsed %d keys, want %d: %v", len(pairs), len(want), pairs)
	}

	for key, value := range want {
		if pairs[key] != value {
			t.Errorf("pairs[%q] = %q, want %q", key, pairs[key], value)
		}
	}
}

func TestPairsEmpty(t *testing.T) {
	if pairs := Pairs(nil); len(pairs) != 0 {
		t.Errorf("Pairs(nil) = %v, want empty", pairs)
	}
}
