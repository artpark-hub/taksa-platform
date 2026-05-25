package topicbrowser

import (
	"strings"

	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
)

// BuildCanonicalTopic matches umh-core graphql buildTopicName.
func BuildCanonicalTopic(topicInfo *unsv1.TopicInfo) string {
	if topicInfo == nil {
		return ""
	}
	var parts []string
	parts = append(parts, topicInfo.GetLevel0())
	parts = append(parts, topicInfo.GetLocationSublevels()...)
	parts = append(parts, topicInfo.GetDataContract())
	if topicInfo.VirtualPath != nil && topicInfo.GetVirtualPath() != "" {
		parts = append(parts, topicInfo.GetVirtualPath())
	}
	parts = append(parts, topicInfo.GetName())
	return strings.Join(parts, ".")
}
