package main

import (
	"metrics/internal/config"
	"metrics/internal/logger"

	"go.uber.org/zap"
)

func main() {
	ctx, complete := config.CompletionCtx()
	defer complete()

	serv, err := config.Configure(ctx,
		config.Server,
		config.WithEnv,
		config.WithServerFlags,
	)
	if err != nil {
		logger.Fatal("server config error", zap.Error(err))
	}

	serv.Run(ctx)
}
