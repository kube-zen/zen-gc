package controller

import "strings"

// parseFieldPath parses a dot-separated field path into a slice for nested field access
// Example: "spec.severity" -> ["spec", "severity"]
// Example: "status.lastProcessedAt" -> ["status", "lastProcessedAt"]
func parseFieldPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}
