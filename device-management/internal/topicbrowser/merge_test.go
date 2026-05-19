package topicbrowser

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
	"google.golang.org/protobuf/proto"
)

func TestMergeFromStatusMessageContent_EmptyCatalogTopicCountZero(t *testing.T) {
	const payload = `{"core":{"topicBrowser":{"topicCount":0}}}`
	mr, err := MergeFromStatusMessageContent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !mr.FullCatalogReplace {
		t.Fatalf("expected FullCatalogReplace when topicCount is 0, got reported=%d full=%v", mr.ReportedTopicCount, mr.FullCatalogReplace)
	}
	if mr.ReportedTopicCount != 0 {
		t.Fatalf("topic count: got %d", mr.ReportedTopicCount)
	}
	if len(mr.Rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(mr.Rows))
	}
}

func TestMergeFromStatusMessageContent_BundleZeroFullSnapshot(t *testing.T) {
	info := &unsv1.TopicInfo{
		Level0:         "Enterprise",
		DataContract:   "_historian",
		Name:           "Temperature",
	}
	hash := HashTopicInfo(info)
	bundle, err := proto.Marshal(&unsv1.UnsBundle{
		UnsMap: &unsv1.TopicMap{Entries: map[string]*unsv1.TopicInfo{hash: info}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(bundle)
	payload, err := json.Marshal(map[string]interface{}{
		"core": map[string]interface{}{
			"topicBrowser": map[string]interface{}{
				"topicCount": 1,
				"unsBundles": map[string]interface{}{"0": b64},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mr, err := MergeFromStatusMessageContent(string(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !mr.FullCatalogReplace {
		t.Fatal("expected FullCatalogReplace for bundle 0 full snapshot")
	}
	if !mr.HadBundleZero {
		t.Fatal("expected HadBundleZero")
	}
	if mr.SyncMode != CatalogSyncFullReplace {
		t.Fatalf("sync mode: got %s", mr.SyncMode)
	}
	if len(mr.Rows) != 1 {
		t.Fatalf("rows: got %d", len(mr.Rows))
	}
}

func TestMergeFromStatusMessageContent_IncrementalBundleMatchingCountNoReplace(t *testing.T) {
	info := &unsv1.TopicInfo{Level0: "Enterprise", DataContract: "_historian", Name: "Temperature"}
	hash := HashTopicInfo(info)
	bundle, err := proto.Marshal(&unsv1.UnsBundle{
		UnsMap: &unsv1.TopicMap{Entries: map[string]*unsv1.TopicInfo{hash: info}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(bundle)
	payload, err := json.Marshal(map[string]interface{}{
		"core": map[string]interface{}{
			"topicBrowser": map[string]interface{}{
				"topicCount": 1,
				"unsBundles": map[string]interface{}{"1": b64},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mr, err := MergeFromStatusMessageContent(string(payload))
	if err != nil {
		t.Fatal(err)
	}
	if mr.FullCatalogReplace {
		t.Fatal("did not expect FullCatalogReplace for non-zero bundle index")
	}
	if mr.HadBundleZero {
		t.Fatal("did not expect HadBundleZero")
	}
}

func TestMergeFromStatusMessageContent_MissingTopicCountNoReplace(t *testing.T) {
	const payload = `{"core":{"topicBrowser":{}}}`
	mr, err := MergeFromStatusMessageContent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if mr.FullCatalogReplace {
		t.Fatal("did not expect FullCatalogReplace without topicCount and bundles")
	}
}
