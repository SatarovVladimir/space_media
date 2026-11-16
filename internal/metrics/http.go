package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTP struct {
	registry  *prometheus.Registry
	requests  *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

func NewHTTP() *HTTP {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "space_media",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "status"})
	durations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "space_media",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})
	registry.MustRegister(requests, durations)

	return &HTTP{
		registry:  registry,
		requests:  requests,
		durations: durations,
	}
}

func (m *HTTP) Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)

		method := normalizedMethod(r.Method)
		m.requests.WithLabelValues(method, strconv.Itoa(writer.status)).Inc()
		m.durations.WithLabelValues(method).Observe(time.Since(start).Seconds())
	})
}

func (m *HTTP) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func normalizedMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	default:
		return "OTHER"
	}
}
