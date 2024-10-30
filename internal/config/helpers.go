package config

import (
	ctx "context"
	"fmt"
	"net/http"

	c "metrics/internal/compress"
	log "metrics/internal/logger"
	sec "metrics/internal/security"
	"metrics/internal/server"

	"github.com/go-chi/chi/v5"
)

func setStorage(cx ctx.Context, cfg *config) (server.Storage, error) {
	switch {
	case cfg.DBAddress != "":
		db, err := server.NewDB(cx, cfg.DBAddress)
		if err != nil {
			return nil, fmt.Errorf("db configure error: %w", err)
		}
		return db, nil
	case cfg.FileStoragePath != "":
		fs := server.NewFileStore(cfg.FileStoragePath, cfg.StoreInterval)
		if cfg.Restore {
			fs.RestoreFromFile(cx)
		}
		return fs, nil
	}
	return server.NewMemStore(), nil
}

func getRoutes(cx ctx.Context, m *server.MetricManager, cfg *config) *chi.Mux {
	ctxMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			customCtx := wrapCtx{
				ctxChi:  r.Context(),
				Context: cx,
			}
			next.ServeHTTP(w, r.WithContext(customCtx))
		})
	}
	router := chi.NewRouter()
	router.Use(log.WithHandlerLog)
	router.Use(c.GzipMiddleware)
	router.Use(ctxMiddleware)
	router.Get("/", m.GetAllHandler)
	router.Get("/ping", m.PingHandler)
	router.Post("/value/", sec.HashMiddleware(cfg.Key, m.GetJSON))
	router.Get("/value/{type}/{id}", m.GetHandler)
	router.Post("/update/", sec.HashMiddleware(cfg.Key, m.UpdateJSON))
	router.Post("/update/{type}/{id}/{value}", m.UpdateHandler)
	router.Post("/updates/", sec.HashMiddleware(cfg.Key, m.BatchHandler))

	return router
}

type wrapCtx struct { // обертка для контекста, что бы не терять контекст chi.Router
	ctx.Context
	ctxChi ctx.Context
}

func (wr wrapCtx) Value(key any) any {
	return wr.ctxChi.Value(key)
}
