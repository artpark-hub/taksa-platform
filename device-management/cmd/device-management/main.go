package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/utils"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newZapLogger(logLevel, logFile string) (*zap.Logger, error) {
	// Default log file path
	if logFile == "" {
		logFile = "/tmp/device-management.log"
	}
	
	cfg := zap.NewDevelopmentConfig()
	
	// Set output to both stdout and file
	cfg.OutputPaths = []string{
		"stdout",
		logFile,
	}
	cfg.ErrorOutputPaths = []string{
		"stderr",
		logFile,
	}

	switch logLevel {
	case "production", "prod":
		prodCfg := zap.NewProductionConfig()
		prodCfg.OutputPaths = []string{
			"stdout",
			logFile,
		}
		prodCfg.ErrorOutputPaths = []string{
			"stderr",
			logFile,
		}
		return prodCfg.Build()
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default: // debug or empty string defaults to debug
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	return cfg.Build()
}

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
		bc.Server.Http.Addr = "0.0.0.0:" + v
	}
	if v := os.Getenv("TAKSA_DM_GRPC_PORT"); v != "" {
		bc.Server.Grpc.Addr = "0.0.0.0:" + v
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
	if v := os.Getenv("TAKSA_DM_ACTION_RETENTION_MINUTES"); v != "" {
		if bc.ActionCleanup == nil {
			bc.ActionCleanup = &conf.ActionCleanup{}
		}
		if n, err := strconv.Atoi(v); err == nil {
			bc.ActionCleanup.RetentionMinutes = int32(n)
		}
	}
	if v := os.Getenv("TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES"); v != "" {
		if bc.ActionCleanup == nil {
			bc.ActionCleanup = &conf.ActionCleanup{}
		}
		if n, err := strconv.Atoi(v); err == nil {
			bc.ActionCleanup.CleanupIntervalMinutes = int32(n)
		}
	}
	if v := os.Getenv("TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES"); v != "" {
		if bc.ActionCleanup == nil {
			bc.ActionCleanup = &conf.ActionCleanup{}
		}
		if n, err := strconv.Atoi(v); err == nil {
			bc.ActionCleanup.AutoExpireMinutes = int32(n)
		}
	}
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, instanceUc *biz.InstanceUsecase) *kratos.App {
	opts := []kratos.Option{
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
	}
	if instanceUc != nil {
		opts = append(opts,
			kratos.BeforeStart(func(ctx context.Context) error {
				instanceUc.StartNATSMirrorFleetReconcile(ctx)
				instanceUc.StartActionCleanupLoop(ctx)
				return nil
			}),
			kratos.AfterStop(func(ctx context.Context) error {
				instanceUc.StopNATSMirrorFleetReconcile()
				instanceUc.StopActionCleanupLoop()
				return nil
			}),
		)
	}
	return kratos.New(opts...)
}

func main() {
	flag.Parse()

	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	applyConfigEnvOverrides(&bc)

	if strings.TrimSpace(bc.Server.JwtSecret) == "" || bc.Server.JwtSecret == `""` {
		bc.Server.JwtSecret = ""
	}
	jwtSecret, err := utils.GetOrGenerateJWTSecret(bc.Server.JwtSecret)
	if err != nil {
		panic(fmt.Errorf("Failed to get or generate JWT secret: %w", err))
	}
	bc.Server.JwtSecret = jwtSecret

	zapLogger, err := newZapLogger(bc.LogLevel, bc.LogFile)
	if err != nil {
		panic(err)
	}
	defer zapLogger.Sync()

	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	zapLogger.Info("Starting device-management",
		zap.String("service.id", id),
		zap.String("service.name", Name),
		zap.String("service.version", Version),
		zap.String("log_level", bc.LogLevel),
	)

	zapLogger.Info("Config loaded successfully",
		zap.String("http.addr", bc.Server.Http.Addr),
		zap.String("grpc.addr", bc.Server.Grpc.Addr),
		zap.String("database.driver", bc.Data.Database.Driver),
		zap.String("database.source", bc.Data.Database.Source),
	)

	statusSub := biz.ResolveStatusSubscriptionSettings(bc.DeviceStatusSubscription)
	actionCleanup := biz.ResolveActionCleanupSettings(bc.ActionCleanup)
	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Deployment, statusSub, actionCleanup, logger, zapLogger)
	if err != nil {
		zapLogger.Fatal("Failed to wire app", zap.Error(err))
	}
	defer cleanup()

	zapLogger.Info("Application initialized, starting server...")

	if err := app.Run(); err != nil {
		zapLogger.Fatal("Application error", zap.Error(err))
	}
}
