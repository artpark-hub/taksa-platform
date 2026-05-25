package biz

import (
	"testing"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestResolveStatusSubscriptionSettings(t *testing.T) {
	def := ResolveStatusSubscriptionSettings(nil)
	if !def.AutoResubscribeOnPull {
		t.Fatal("expected auto resubscribe on by default")
	}
	if def.CatalogStaleThreshold != 2*time.Minute {
		t.Fatalf("catalog threshold: got %v", def.CatalogStaleThreshold)
	}

	falseVal := false
	cfg := &conf.DeviceStatusSubscription{
		AutoResubscribeStatusMessages: &falseVal,
		CatalogStaleThreshold:         durationpb.New(90 * time.Second),
	}
	off := ResolveStatusSubscriptionSettings(cfg)
	if off.AutoResubscribeOnPull {
		t.Fatal("expected auto resubscribe off")
	}

	// Block present but field omitted => default true.
	cfgOmitted := &conf.DeviceStatusSubscription{
		CatalogStaleThreshold: durationpb.New(60 * time.Second),
	}
	defOmitted := ResolveStatusSubscriptionSettings(cfgOmitted)
	if !defOmitted.AutoResubscribeOnPull {
		t.Fatal("expected auto resubscribe on when field omitted")
	}
	if off.CatalogStaleThreshold != 90*time.Second {
		t.Fatalf("catalog threshold: got %v", off.CatalogStaleThreshold)
	}
}
