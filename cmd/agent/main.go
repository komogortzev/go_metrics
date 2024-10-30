package main

import (
	"metrics/internal/config"
	"metrics/internal/logger"

	"go.uber.org/zap"
)

func main() {
	ctx, complete := config.CompletionCtx()
	defer complete()

	ag, err := config.Configure(ctx,
		config.Agent,
		config.WithEnv,
		config.WithAgentFlags,
	)
	if err != nil {
		logger.Fatal("agent config error", zap.Error(err))
	}

	ag.Run(ctx)
}
