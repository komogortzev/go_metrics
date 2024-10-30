package server

import (
	s "metrics/internal/service"
)

func getQuery(oper dbOperation, met *s.Metrics) string {
	switch oper {
	case insertMetric:
		if met.IsGauge() {
			return insertGauge
		}
		return insertCounter
	default:
		if met.IsGauge() {
			return selectGauge
		}
		return selectCounter
	}
}

func setVal(met *s.Metrics, val any) {
	if v, ok := val.(int64); ok {
		met.Delta = &v
	} else {
		v, _ := val.(float64)
		met.Value = &v
	}
}
