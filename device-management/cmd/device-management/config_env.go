package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
)

const (
	defaultActionCleanupRetentionMinutes       = 60
	defaultActionCleanupIntervalMinutes        = 10
	defaultActionCleanupAutoExpireMinutes      = 0
)

// applyConfigEnvOverrides applies TAKSA_DM_* env vars over config.yaml (env wins when set).
func applyConfigEnvOverrides(bc *conf.Bootstrap) {
	if bc == nil {
		return
	}
	if v := os.Getenv("TAKSA_DM_LOG_LEVEL"); v != "" {
		bc.LogLevel = v
	}
	if v := os.Getenv("TAKSA_DM_LOG_FILE"); v != "" {
		bc.LogFile = v
	}
	if v := os.Getenv("TAKSA_DM_HTTP_PORT"); v != "" {
		bc.Server.Http.Addr = overrideServerPort(bc.Server.Http.Addr, v)
	}
	if v := os.Getenv("TAKSA_DM_GRPC_PORT"); v != "" {
		bc.Server.Grpc.Addr = overrideServerPort(bc.Server.Grpc.Addr, v)
	}
	if v := os.Getenv("TAKSA_DM_DATABASE_DRIVER"); v != "" {
		bc.Data.Database.Driver = v
	}
	if v := os.Getenv("TAKSA_DM_DATABASE_SOURCE"); v != "" {
		bc.Data.Database.Source = v
	}
	if v := os.Getenv("TAKSA_DM_BASE_URL"); v != "" {
		bc.Deployment.BaseUrl = v
	}
	if v := os.Getenv("TAKSA_DM_UMH_CORE_DOCKER_IMAGE"); v != "" {
		bc.Deployment.UmhCoreDockerImage = v
	}
	if v := os.Getenv("TAKSA_DM_NATS_MIRROR_URLS"); v != "" {
		bc.Deployment.NatsMirrorUrls = v
	}
	if v := os.Getenv("TAKSA_DM_JWT_SECRET"); v != "" {
		bc.Server.JwtSecret = v
	}
	if v := os.Getenv("TAKSA_DM_AUTO_RESUBSCRIBE_STATUS_MESSAGES"); v != "" {
		if bc.DeviceStatusSubscription == nil {
			bc.DeviceStatusSubscription = &conf.DeviceStatusSubscription{}
		}
		enabled := v == "true" || v == "1"
		bc.DeviceStatusSubscription.AutoResubscribeStatusMessages = &enabled
	}
	applyActionCleanupEnvOverrides(bc)
}

// ensureActionCleanupProto seeds action_cleanup when env overrides need a target and yaml omitted the block.
func ensureActionCleanupProto(bc *conf.Bootstrap) {
	if bc == nil || bc.ActionCleanup != nil {
		return
	}
	bc.ActionCleanup = &conf.ActionCleanup{
		RetentionMinutes:       defaultActionCleanupRetentionMinutes,
		CleanupIntervalMinutes: defaultActionCleanupIntervalMinutes,
		AutoExpireMinutes:      defaultActionCleanupAutoExpireMinutes,
	}
}

func applyActionCleanupEnvOverrides(bc *conf.Bootstrap) {
	if bc == nil {
		return
	}
	applyActionCleanupIntEnv(bc, "TAKSA_DM_ACTION_RETENTION_MINUTES", func(ac *conf.ActionCleanup, n int32) {
		ac.RetentionMinutes = n
	})
	applyActionCleanupIntEnv(bc, "TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES", func(ac *conf.ActionCleanup, n int32) {
		ac.CleanupIntervalMinutes = n
	})
	applyActionCleanupIntEnv(bc, "TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES", func(ac *conf.ActionCleanup, n int32) {
		ac.AutoExpireMinutes = n
	})
}

func applyActionCleanupIntEnv(bc *conf.Bootstrap, envName string, set func(*conf.ActionCleanup, int32)) {
	v := os.Getenv(envName)
	if v == "" {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Printf("WARNING: %s=%q is not an integer; ignoring env override\n", envName, v)
		return
	}
	ensureActionCleanupProto(bc)
	set(bc.ActionCleanup, int32(n))
}

// overrideServerPort replaces the port in addr, preserving the host from config.yaml when present.
func overrideServerPort(addr, port string) string {
	if port == "" {
		return addr
	}
	host := "0.0.0.0"
	if addr != "" {
		if h, _, err := net.SplitHostPort(addr); err == nil && h != "" {
			host = h
		}
	}
	return net.JoinHostPort(host, port)
}
