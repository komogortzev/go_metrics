package service

import (
	ctx "context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	log "metrics/internal/logger"

	"github.com/cenkalti/backoff/v4"
)

const (
	gauge   = "gauge"
	counter = "counter"
)

var (
	ErrInvalidVal  = errors.New("invalid metric value")
	ErrInvalidType = errors.New("invalid metric type")

	metricsPool = sync.Pool{ // Pool для переиспользования структур Metrics
		New: func() any {
			return &Metrics{}
		},
	}
)

//go:generate ffjson $GOFILE
type Metrics struct {
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	ID    string   `json:"id"`
	MType string   `json:"type"`
}

func NewMetric(mtype, id string, val string) (*Metrics, error) {
	met, _ := metricsPool.Get().(*Metrics)
	met.ID = id
	met.MType = mtype

	if !met.IsCounter() && !met.IsGauge() {
		metricsPool.Put(met)
		return nil, ErrInvalidType
	}
	if val == "" {
		return met, nil
	}

	var err error
	if met.IsCounter() {
		err = met.setCounterValue(val)
	} else {
		err = met.setGaugeValue(val)
	}
	if err != nil {
		metricsPool.Put(met)
		return nil, err
	}

	return met, nil
}

func (met *Metrics) setCounterValue(val string) error {
	num, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return ErrInvalidVal
	}
	met.Delta = &num
	return nil
}

func (met *Metrics) setGaugeValue(val string) error {
	num, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return ErrInvalidVal
	}
	met.Value = &num
	return nil
}

func BuildMetric(name string, val any) *Metrics {
	met, _ := metricsPool.Get().(*Metrics)
	met.ID = name

	switch v := val.(type) {
	case float64:
		met.MType = gauge
		met.Value = &v
	case int64:
		met.MType = counter
		met.Delta = &v
	default:
		metricsPool.Put(met)
		return nil
	}

	return met
}

func (met *Metrics) String() string {
	if met.Delta == nil && met.Value == nil {
		return fmt.Sprintf(" (%s: <empty>)", met.ID)
	}
	if met.IsCounter() {
		return fmt.Sprintf(" (%s: %d)", met.ID, *met.Delta)
	}
	return fmt.Sprintf(" (%s: %g)", met.ID, *met.Value)
}

func (met *Metrics) MergeMetrics(met2 *Metrics) {
	if met2 == nil {
		return
	}
	if met2.IsCounter() && met2.Delta != nil {
		if met.Delta == nil {
			met.Delta = new(int64)
		}
		*met.Delta += *met2.Delta
	}
}

func (met Metrics) ToSlice() []any {
	if met.IsCounter() {
		return []any{met.ID, *met.Delta}
	}
	return []any{met.ID, *met.Value}
}

func (met *Metrics) IsGauge() bool {
	return met.MType == gauge
}

func (met *Metrics) IsCounter() bool {
	return met.MType == counter
}

func Retry(cx ctx.Context, fn func() error) error {
	log.Debug("Retry...")
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 1 * time.Second
	expBackoff.Multiplier = 3
	expBackoff.MaxInterval = 5 * time.Second
	expBackoff.MaxElapsedTime = 11 * time.Second

	return backoff.Retry(fn, backoff.WithContext(expBackoff, cx))
}
