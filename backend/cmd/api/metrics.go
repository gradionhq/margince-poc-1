package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// keyStatus is the Prometheus label name for the response status. Duplicated
// from httpserver's own keyStatus (a structured-log field key there): the two
// packages split apart in the 1c restructure (task-3-brief.md) and this
// 6-character literal isn't worth an import-boundary coupling in either
// direction (package main cannot be imported).
const keyStatus = "status"

var (
	metricsReg = prometheus.NewRegistry()

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", keyStatus})

	jobQueueDepth      = prometheus.NewGauge(prometheus.GaugeOpts{Name: "job_queue_depth", Help: "River job queue depth."})
	approvalQueueAge   = prometheus.NewGauge(prometheus.GaugeOpts{Name: "approval_queue_age_seconds", Help: "Oldest approval age."})
	eventOutboxDepth   = prometheus.NewGauge(prometheus.GaugeOpts{Name: "event_outbox_depth", Help: "Unpublished outbox rows."})
	consumerLagSeconds = prometheus.NewGauge(prometheus.GaugeOpts{Name: "consumer_lag_seconds", Help: "Consumer lag."})
)

func init() {
	metricsReg.MustRegister(httpDuration, jobQueueDepth, approvalQueueAge, eventOutboxDepth, consumerLagSeconds)
}

func metricsHandler() http.Handler {
	return promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

// instrument wraps h, observing request duration with bounded labels
// (method/route/status — never an entity or workspace id).
func instrument(route string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(sr, r)
		httpDuration.WithLabelValues(r.Method, route, strconv.Itoa(sr.status)).Observe(time.Since(start).Seconds())
	})
}

// sampleGauges periodically refreshes DB-derived gauges.
func sampleGauges(ctx context.Context, db *sql.DB) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			var depth float64
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM event_outbox WHERE published_at IS NULL`).Scan(&depth); err == nil {
				eventOutboxDepth.Set(depth)
			}
		}
	}
}
