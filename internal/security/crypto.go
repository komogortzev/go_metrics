package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	log "metrics/internal/logger"

	"go.uber.org/zap"
)

type hashWriter struct {
	http.ResponseWriter
	sign string
}

func newHashWriter(s string, w http.ResponseWriter) *hashWriter {
	return &hashWriter{
		ResponseWriter: w,
		sign:           s,
	}
}

func (c *hashWriter) WriteHeader(statusCode int) {
	if statusCode < 300 {
		c.ResponseWriter.Header().Set("HashSHA256", c.sign)
	}
	c.ResponseWriter.WriteHeader(statusCode)
}

func Hash(data *[]byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(*data)

	return hex.EncodeToString(h.Sum(nil))
}

func HashMiddleware(key string, next http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		log.Debug("hash middleware...")
		ow := rw
		sign := req.Header.Get("HashSHA256")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Warn("HashMiddleware: body err:", zap.Error(err))
			ow.WriteHeader(http.StatusBadRequest)
			return
		}
		if key == "" || sign == "" || len(body) == 0 {
			log.Info("without hash...")
			req.Body = io.NopCloser(bytes.NewBuffer(body))
			next.ServeHTTP(ow, req)
			return
		}
		srcSign := Hash(&body, key)
		if srcSign != sign {
			log.Warn("HashMiddleware: sing error",
				zap.String("src", srcSign),
				zap.String("sign", sign),
			)
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		ow = newHashWriter(srcSign, rw)
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(ow, req)
	}
}
