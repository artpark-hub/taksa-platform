package main

import (
	"testing"

	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
)

func TestOverrideServerPortPreservesHost(t *testing.T) {
	if got := overrideServerPort("127.0.0.1:8000", "9000"); got != "127.0.0.1:9000" {
		t.Fatalf("expected 127.0.0.1:9000, got %q", got)
	}
	if got := overrideServerPort("0.0.0.0:8000", "8080"); got != "0.0.0.0:8080" {
		t.Fatalf("expected 0.0.0.0:8080, got %q", got)
	}
	if got := overrideServerPort("", "8000"); got != "0.0.0.0:8000" {
		t.Fatalf("expected 0.0.0.0:8000, got %q", got)
	}
}

func TestApplyActionCleanupEnvOverrides_InvalidRetentionKeepsYAML(t *testing.T) {
	t.Setenv("TAKSA_DM_ACTION_RETENTION_MINUTES", "not-a-number")
	t.Setenv("TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES", "")
	t.Setenv("TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES", "")

	bc := &conf.Bootstrap{
		ActionCleanup: &conf.ActionCleanup{
			RetentionMinutes:       60,
			CleanupIntervalMinutes: 10,
		},
	}
	applyConfigEnvOverrides(bc)

	if bc.ActionCleanup.RetentionMinutes != 60 {
		t.Fatalf("retention: want 60 from yaml, got %d", bc.ActionCleanup.RetentionMinutes)
	}
	if bc.ActionCleanup.CleanupIntervalMinutes != 10 {
		t.Fatalf("interval: want 10 from yaml, got %d", bc.ActionCleanup.CleanupIntervalMinutes)
	}
	got := biz.ResolveActionCleanupSettings(bc.ActionCleanup)
	if got.CleanupIntervalMinutes != 10 {
		t.Fatalf("resolved interval: want 10, got %d", got.CleanupIntervalMinutes)
	}
}

func TestApplyActionCleanupEnvOverrides_InvalidRetentionNoYAMLUsesResolveDefaults(t *testing.T) {
	t.Setenv("TAKSA_DM_ACTION_RETENTION_MINUTES", "bad")
	t.Setenv("TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES", "")
	t.Setenv("TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES", "")

	bc := &conf.Bootstrap{}
	applyConfigEnvOverrides(bc)

	if bc.ActionCleanup != nil {
		t.Fatal("expected ActionCleanup to stay nil when env parse fails and yaml omitted block")
	}
	got := biz.ResolveActionCleanupSettings(bc.ActionCleanup)
	if got.RetentionMinutes != 60 || got.CleanupIntervalMinutes != 10 {
		t.Fatalf("expected resolve defaults (60,10), got %+v", got)
	}
}

func TestApplyActionCleanupEnvOverrides_PartialRetentionSeedsDefaults(t *testing.T) {
	t.Setenv("TAKSA_DM_ACTION_RETENTION_MINUTES", "30")
	t.Setenv("TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES", "")
	t.Setenv("TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES", "")

	bc := &conf.Bootstrap{}
	applyConfigEnvOverrides(bc)

	if bc.ActionCleanup == nil {
		t.Fatal("expected ActionCleanup after successful override")
	}
	if bc.ActionCleanup.RetentionMinutes != 30 {
		t.Fatalf("retention: want 30, got %d", bc.ActionCleanup.RetentionMinutes)
	}
	if bc.ActionCleanup.CleanupIntervalMinutes != 10 {
		t.Fatalf("interval: want default 10 when only retention env set, got %d", bc.ActionCleanup.CleanupIntervalMinutes)
	}
	got := biz.ResolveActionCleanupSettings(bc.ActionCleanup)
	if got.CleanupIntervalMinutes != 10 {
		t.Fatalf("resolved interval: want 10, got %d", got.CleanupIntervalMinutes)
	}
}

func TestApplyConfigEnvOverrides_HTTPPortPreservesLocalhost(t *testing.T) {
	t.Setenv("TAKSA_DM_HTTP_PORT", "9000")
	t.Setenv("TAKSA_DM_GRPC_PORT", "")

	bc := &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{Addr: "127.0.0.1:8000"},
		},
	}
	applyConfigEnvOverrides(bc)
	if bc.Server.Http.Addr != "127.0.0.1:9000" {
		t.Fatalf("http addr: want 127.0.0.1:9000, got %q", bc.Server.Http.Addr)
	}
}
