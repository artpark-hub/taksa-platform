package main

import (
	"flag"
	"os"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"

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

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()

	// Load config first to get log level
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

	// Override with environment variables (Docker containerized service pattern)
	// Pattern: TAKSA_DM_* (aligned with taksa-deployments)
	if logLevel := os.Getenv("TAKSA_DM_LOG_LEVEL"); logLevel != "" {
		bc.LogLevel = logLevel
	}
	if logFile := os.Getenv("TAKSA_DM_LOG_FILE"); logFile != "" {
		bc.LogFile = logFile
	}
	if httpPort := os.Getenv("TAKSA_DM_HTTP_PORT"); httpPort != "" {
		bc.Server.Http.Addr = "0.0.0.0:" + httpPort
	}
	if grpcPort := os.Getenv("TAKSA_DM_GRPC_PORT"); grpcPort != "" {
		bc.Server.Grpc.Addr = "0.0.0.0:" + grpcPort
	}
	if dbDriver := os.Getenv("TAKSA_DM_DATABASE_DRIVER"); dbDriver != "" {
		bc.Data.Database.Driver = dbDriver
	}
	if dbSource := os.Getenv("TAKSA_DM_DATABASE_SOURCE"); dbSource != "" {
		bc.Data.Database.Source = dbSource
	}
	if baseURL := os.Getenv("TAKSA_DM_BASE_URL"); baseURL != "" {
		bc.Deployment.BaseUrl = baseURL
	}
	if dockerImage := os.Getenv("TAKSA_DM_UMH_CORE_DOCKER_IMAGE"); dockerImage != "" {
		bc.Deployment.UmhCoreDockerImage = dockerImage
	}
	if jwtSecret := os.Getenv("TAKSA_DM_JWT_SECRET"); jwtSecret != "" {
		bc.Server.JwtSecret = jwtSecret
	}
	if bc.Server.JwtSecret == "" {
		panic("TAKSA_DM_JWT_SECRET is required (set in .env, config.yaml server.jwt_secret, or environment)")
	}

	// Initialize Zap logger based on config log_level and log_file
	zapLogger, err := newZapLogger(bc.LogLevel, bc.LogFile)
	if err != nil {
		panic(err)
	}
	defer zapLogger.Sync()

	// Wrap Zap logger to be compatible with Kratos logger interface
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

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Deployment, logger, zapLogger)
	if err != nil {
		zapLogger.Fatal("Failed to wire app", zap.Error(err))
	}
	defer cleanup()

	zapLogger.Info("Application initialized, starting server...")

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		zapLogger.Fatal("Application error", zap.Error(err))
	}
}
