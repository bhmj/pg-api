package metrics

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type tPrometheusStat struct {
	errors  *prometheus.CounterVec
	latency *prometheus.HistogramVec
	sync.RWMutex
}

var stat tPrometheusStat

// Metrics implements Score function to store metrics
type Metrics interface {
	Score(method string, path string, scope string, begin time.Time, err *error)
}

// Score registers latency and error count
func (t *tPrometheusStat) Score(method string, path string, scope string, begin time.Time, err *error) {
	labels := prometheus.Labels{
		"method": method,
		"path":   path,
		"scope":  scope,
	}
	if err != nil && *err != nil {
		t.errors.With(labels).Add(1)
	}
	t.latency.With(labels).Observe(time.Since(begin).Seconds())
}

// NewMetrics returns a Metrics instance
func NewMetrics(service string, buckets []float64) Metrics {
	labelNames := []string{"method", "path", "scope"}
	defaultBuckets := []float64{0.001, 0.002, 0.003, 0.005, 0.010, 0.018, 0.030, 0.055, 0.100, 0.180, 0.300, 0.550, 1, 1.8, 3, 5} // log scale
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}
	return &tPrometheusStat{
		errors: newCounterFrom(prometheus.CounterOpts{
			Namespace: strings.Replace(service, "-", "_", -1),
			Name:      "error_count",
			Help:      "Error count per service/scope",
		}, labelNames),
		latency: newHistogramFrom(prometheus.HistogramOpts{
			Namespace: strings.Replace(service, "-", "_", -1),
			Name:      "request_latency",
			Help:      "Total duration of request in seconds",
			Buckets:   buckets,
		}, labelNames),
	}
}

func newHistogramFrom(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	hv := prometheus.NewHistogramVec(opts, labelNames)
	prometheus.MustRegister(hv)
	return hv
}
func newCounterFrom(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	co := prometheus.NewCounterVec(opts, labelNames)
	prometheus.MustRegister(co)
	return co
}
