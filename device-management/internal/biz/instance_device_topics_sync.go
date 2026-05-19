package biz

import (
	"context"
	"fmt"

	"github.com/artpark-hub/taksa-platform/device-management/internal/topicbrowser"
)

// syncDeviceTopicsFromStatusMessage materializes UNS topics from core.topicBrowser (UnsBundle frames).
func (uc *InstanceUsecase) syncDeviceTopicsFromStatusMessage(ctx context.Context, tenantID, deviceID, messageContent string) error {
	if uc.deviceTopicRepo == nil {
		return nil
	}
	mr, err := topicbrowser.MergeFromStatusMessageContent(messageContent)
	if err != nil {
		fmt.Printf("Warning: topic browser merge from status: %v\n", err)
		return nil
	}

	recordCatalog := func() {
		if err := uc.deviceTopicRepo.UpsertDeviceTopicCatalog(ctx, tenantID, deviceID, mr.ReportedTopicCount, mr.SyncMode, mr.HadBundleZero); err != nil {
			fmt.Printf("Warning: failed to update device topic catalog metadata: %v\n", err)
		}
	}

	if mr.FullCatalogReplace {
		if mr.ReportedTopicCount == 0 {
			if err := uc.deviceTopicRepo.ClearDeviceTopics(ctx, tenantID, deviceID); err != nil {
				fmt.Printf("Warning: failed to clear device topics: %v\n", err)
			}
			recordCatalog()
			return nil
		}
		if err := uc.deviceTopicRepo.ReplaceAllDeviceTopics(ctx, tenantID, deviceID, mr.Rows); err != nil {
			fmt.Printf("Warning: failed to replace device topics: %v\n", err)
			return nil
		}
		recordCatalog()
		return nil
	}

	if len(mr.Rows) == 0 {
		return nil
	}
	if err := uc.deviceTopicRepo.UpsertDeviceTopics(ctx, tenantID, deviceID, mr.Rows); err != nil {
		fmt.Printf("Warning: failed to upsert device topics: %v\n", err)
		return nil
	}
	recordCatalog()
	return nil
}
