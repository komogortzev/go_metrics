package logger

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Debug(msg string, fields ...zapcore.Field) { // обертки:
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zapcore.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zapcore.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zapcore.Field) {
	logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zapcore.Field) {
	logger.Fatal(msg, fields...)
}

var logger *zap.Logger = zap.NewNop()

func InitLog() error {
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zapcore.DebugLevel)

	config := zap.Config{
		Level:       atomicLevel,
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		return fmt.Errorf("build config for logger error: %w", err)
	}

	logger.Debug("Logger configured and running with Debug level")
	return nil
}

type loggingResponse struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *loggingResponse) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, fmt.Errorf("response writing error: %w", err)
}

func (r *loggingResponse) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.status = statusCode
}

func WithHandlerLog(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		logResp := loggingResponse{
			ResponseWriter: w,
			status:         0,
			size:           0,
		}

		h.ServeHTTP(&logResp, r)

		duration := time.Since(start)

		logger.Info("Request/Response logging:",
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.Int("status", logResp.status),
			zap.Duration("duration", duration),
			zap.Int("size", logResp.size),
		)
	}

	return http.HandlerFunc(fn)
}
