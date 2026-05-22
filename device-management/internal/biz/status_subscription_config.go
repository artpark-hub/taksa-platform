package biz

import (
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
)

// StatusSubscriptionSettings is resolved DM config for edge status subscribe keepalive.
type StatusSubscriptionSettings struct {
	AutoResubscribeOnPull         bool
	CatalogStaleThreshold         time.Duration
	StatusHeartbeatStaleThreshold time.Duration
}

// ResolveStatusSubscriptionSettings applies defaults and proto/env overrides.
func ResolveStatusSubscriptionSettings(cfg *conf.DeviceStatusSubscription) StatusSubscriptionSettings {
	out := StatusSubscriptionSettings{
		AutoResubscribeOnPull:         true,
		CatalogStaleThreshold:         2 * time.Minute,
		StatusHeartbeatStaleThreshold: 2 * time.Minute,
	}
	if cfg != nil {
		out.AutoResubscribeOnPull = cfg.AutoResubscribeStatusMessages
		if d := cfg.CatalogStaleThreshold; d != nil {
			out.CatalogStaleThreshold = d.AsDuration()
		}
		if d := cfg.StatusHeartbeatStaleThreshold; d != nil {
			out.StatusHeartbeatStaleThreshold = d.AsDuration()
		}
	}
	if out.CatalogStaleThreshold <= 0 {
		out.CatalogStaleThreshold = 2 * time.Minute
	}
	if out.StatusHeartbeatStaleThreshold <= 0 {
		out.StatusHeartbeatStaleThreshold = 2 * time.Minute
	}
	return out
}
