package proxy

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AppMetrics struct {
	enabled          bool
	RequestsCounter  *prometheus.CounterVec
	RequestsInflight prometheus.Gauge
	RequestsDuration *prometheus.HistogramVec
}

func (p *ProxyInstance) setupMetrics() {
	p.metrics.RequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "udsproxy_http_requests_total",
			Help: "How many requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method"},
	)

	rqDurationHistogramOpts := prometheus.HistogramOpts{
		Name:        "udsproxy_request_duration_seconds",
		Help:        "A histogram of latencies for requests.",
		Buckets:     []float64{.1, .2, .4, .6, .8, 1, 1.2, 1.5, 1.8, 2, 2.5, 5},
		ConstLabels: prometheus.Labels{"handler": "proxyHandler"},
	}
	p.metrics.RequestsDuration = prometheus.NewHistogramVec(
		rqDurationHistogramOpts,
		[]string{"method"},
	)

	p.metrics.RequestsInflight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "udsproxy",
		Subsystem: "http",
		Name:      "inflight",
		Help:      "Number of requests being actively processed",
	})

	prometheus.MustRegister(
		p.metrics.RequestsDuration,
		p.metrics.RequestsInflight,
		p.metrics.RequestsCounter,
	)
	p.metrics.enabled = true
}

func (p *ProxyInstance) startPrometheusMetricsServer() {
	log.Printf("Prometheus : http://localhost%s/metrics", p.Options.PrometheusPort)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(p.Options.PrometheusPort, nil))
}
