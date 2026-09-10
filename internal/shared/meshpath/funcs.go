// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshpath

import (
	"net/url"
	"strings"
)

// Endpoint returns the absolute URL of a MeshCentral endpoint.
//
// Any path prefix on base is preserved, which is how MeshCentral addresses
// non-default domains.
func Endpoint(base *url.URL, name string, query url.Values) string {
	endpoint := *base

	endpoint.Path = joinPath(base.Path, name)
	endpoint.RawQuery = query.Encode()

	return endpoint.String()
}

// joinPath appends name to prefix with exactly one separator between them.
func joinPath(prefix, name string) string {
	return strings.TrimRight(prefix, separator) + separator + name
}
