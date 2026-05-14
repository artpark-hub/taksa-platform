package topicbrowser

import "testing"

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
