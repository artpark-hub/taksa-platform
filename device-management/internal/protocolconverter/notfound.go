package protocolconverter

import "strings"

// IsNotFoundError reports whether umh-core indicated the target resource is absent on the edge.
// Delete and rollback treat this as idempotent success (catalog should not retain ghosts).
func IsNotFoundError(message string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(message)), "not found")
}
