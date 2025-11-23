package pathutil

import "strings"

// NormalizeForDisplay normalizes a file path for display in list output.
// It ensures the path starts with "/" for consistency and familiar UX.
// Examples:
//   - "bin/sh" -> "/bin/sh"
//   - "/bin/sh" -> "/bin/sh"
//   - "./bin/sh" -> "/bin/sh"
//   - "bin/sh" -> "/bin/sh"
func NormalizeForDisplay(path string) string {
	// Remove leading "./" if present
	path = strings.TrimPrefix(path, "./")

	// Ensure the path starts with "/"
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}
