package agent

import (
	"bytes"
	ctx "context"
	"net/http"
	"runtime"
	"sync"

	"metrics/internal/compress"
	"metrics/internal/logger"
	"metrics/internal/security"
	s "metrics/internal/service"

	"go.uber.org/zap"
)

const numMemMetrics = 31

var (
	numAllMetrics = runtime.NumCPU() + numMemMetrics
	randVal       float64
	pollCount     int64
	mets          = make([]*s.Metrics, numAllMetrics)
)

func NewSelfMonitor() *SelfMonitor {
	return &SelfMonitor{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (sm *SelfMonitor) sendWorker(cx ctx.Context,
	url string,
	dataCh <-chan []byte,
	wg *sync.WaitGroup,
) {
	for data := range dataCh {
		compressData, _ := compress.Compress(data)

		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(compressData))
		if sm.Key != "" {
			sign := security.Hash(&data, sm.Key)
			req.Header.Set("HashSHA256", sign)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		logger.Debug("REPORT...")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			_ = s.Retry(cx, func() error {
				r2, retErr := http.DefaultClient.Do(req)
				closeBody(r2)
				logger.Warn("retry result", zap.Error(retErr))
				return retErr
			})
		} else {
			logger.Debug("success report!")
			sm.cond.L.Lock()
			pollCount = 0
			sm.cond.L.Unlock()
		}
		closeBody(r)
	}
	wg.Done()
	logger.Debug("goodbye from sendWorker")
}

func closeBody(r *http.Response) {
	if r != nil && r.Body != nil {
		r.Body.Close()
	}
}
