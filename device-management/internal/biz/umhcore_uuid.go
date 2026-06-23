package biz

import "github.com/google/uuid"

// GenerateUUIDFromName matches umh-core deterministic UUID generation for named components.
func GenerateUUIDFromName(name string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(name)).String()
}
