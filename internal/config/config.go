package config

import (
	ctx "context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metrics/internal/agent"
	log "metrics/internal/logger"
	"metrics/internal/server"

	"go.uber.org/zap"
)

type Executable interface {
	Run(ctx.Context)
}

type config struct {
	Address         string `env:"ADDRESS"`
	Key             string `env:"KEY"`
	FileStoragePath string
	DBAddress       string `env:"DATABASE_DSN"`
	StoreInterval   int    `env:"STORE_INTERVAL" envDefault:"-1"`
	PollInterval    int    `env:"POLL_INTERVAL" envDefault:"-1"`
	ReportInterval  int    `env:"REPORT_INTERVAL" envDefault:"-1"`
	Restore         bool   `env:"RESTORE" envDefault:"true"`
	RateLimit       int    `env:"RATE_LIMIT"`
}

type Option func(*config) error

type AppType uint8

const (
	Server AppType = iota
	Agent
)

func Configure(cx ctx.Context, appType AppType, opts ...Option) (Executable, error) {
	err := log.InitLog()
	if err != nil {
		return nil, fmt.Errorf("init log: %w", err)
	}
	cfg := &config{}
	for _, op := range opts {
		if err := op(cfg); err != nil {
			return nil, err
		}
	}
	switch appType {
	case Server:
		log.Info("MetricManager configuration",
			zap.String("addr", cfg.Address),
			zap.Int("store interval", cfg.StoreInterval),
			zap.Bool("restore", cfg.Restore),
			zap.String("file store", cfg.FileStoragePath),
			zap.String("database", cfg.DBAddress),
			zap.String("decrypt key", cfg.Key))
		return NewManager(cx, cfg)
	default:
		log.Info("SelfMonitor configuration",
			zap.String("addr", cfg.Address),
			zap.Int("poll interval", cfg.PollInterval),
			zap.Int("report interval", cfg.ReportInterval),
			zap.String("encrypt key", cfg.Key),
			zap.Int("rate limit", cfg.RateLimit))
		return NewMonitor(cfg)
	}
}

func NewManager(cx ctx.Context, cfg *config) (*server.MetricManager, error) {
	var err error
	manager := &server.MetricManager{Server: http.Server{}}
	manager.Addr = cfg.Address
	manager.Handler = getRoutes(cx, manager, cfg)
	manager.Storage, err = setStorage(cx, cfg)

	return manager, err
}

func NewMonitor(cfg *config) (*agent.SelfMonitor, error) {
	monitor := agent.NewSelfMonitor()
	monitor.Address = cfg.Address
	monitor.PollInterval = time.Duration(cfg.PollInterval) * time.Second
	monitor.ReportInterval = time.Duration(cfg.ReportInterval) * time.Second
	monitor.Key = cfg.Key
	if cfg.RateLimit <= 0 {
		monitor.Rate = 1
	} else {
		monitor.Rate = cfg.RateLimit
	}

	return monitor, nil
}

func CompletionCtx() (ctx.Context, ctx.CancelFunc) {
	cx, complete := ctx.WithCancel(ctx.Background())
	signChan := make(chan os.Signal, 1)
	signal.Notify(signChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signChan
		complete()
	}()

	return cx, complete
}
