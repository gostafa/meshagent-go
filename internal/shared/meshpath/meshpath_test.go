// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshpath

import (
	"net/url"
	"testing"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}

	return parsed
}

func TestEndpoint(t *testing.T) {
	tests := map[string]struct {
		base  string
		name  string
		query url.Values
		want  string
	}{
		"bare host": {
			base:  "https://mesh.example.com",
			name:  AgentsEndpoint,
			query: url.Values{"id": {"4"}},
			want:  "https://mesh.example.com/meshagents?id=4",
		},
		"trailing slash is not doubled": {
			base:  "https://mesh.example.com/",
			name:  SettingsEndpoint,
			query: url.Values{},
			want:  "https://mesh.example.com/meshsettings",
		},
		"domain path prefix is preserved": {
			base:  "https://mesh.example.com/customer1",
			name:  AgentsEndpoint,
			query: url.Values{"meshid": {"g"}},
			want:  "https://mesh.example.com/customer1/meshagents?meshid=g",
		},
		"query values are escaped": {
			base:  "https://mesh.example.com",
			name:  AgentsEndpoint,
			query: url.Values{"meshid": {"a b&c"}},
			want:  "https://mesh.example.com/meshagents?meshid=a+b%26c",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Endpoint(mustParse(t, tt.base), tt.name, tt.query)
			if got != tt.want {
				t.Errorf("Endpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}
