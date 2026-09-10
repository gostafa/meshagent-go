// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package meshserver

import (
	"fmt"
)

// Error implements the error interface.
func (httpErr *HTTPError) Error() string {
	return fmt.Sprintf("meshserver: %s returned %s", httpErr.URL, httpErr.Status)
}

// Error implements the error interface.
func (binErr *InvalidBinaryError) Error() string {
	return "meshserver: not a valid windows executable: " + binErr.Reason
}
