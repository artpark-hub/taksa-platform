package topicbrowser

import (
	"encoding/hex"

	"github.com/cespare/xxhash/v2"
	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
)

// HashTopicInfo matches umh-core GraphQL hashUNSTableEntry (xxhash + NUL delimiters, hex-encoded).
func HashTopicInfo(info *unsv1.TopicInfo) string {
	if info == nil {
		return ""
	}
	hasher := xxhash.New()
	write := func(s string) {
		_, _ = hasher.Write(append([]byte(s), 0))
	}
	write(info.GetLevel0())
	for _, level := range info.GetLocationSublevels() {
		write(level)
	}
	write(info.GetDataContract())
	if info.VirtualPath != nil {
		write(info.GetVirtualPath())
	}
	write(info.GetName())
	return hex.EncodeToString(hasher.Sum(nil))
}
