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
	RequestsSize     *prometheus.HistogramVec
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
		Buckets:     []float64{.1, .3, .7, 1, 1.5, 2.5},
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

	p.metrics.RequestsSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{500, 1000, 2500, 5000},
		},
		[]string{},
	)

	prometheus.MustRegister(
		p.metrics.RequestsDuration,
		p.metrics.RequestsInflight,
		p.metrics.RequestsCounter,
		p.metrics.RequestsSize,
	)
	p.metrics.enabled = true
}

func getTracingTransport(transport *http.Transport) http.RoundTripper {
	// copy-pasta from
	// https://github.com/prometheus/client_golang/blob/master/prometheus/promhttp/instrument_client_test.go
	dnsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "udsproxy_dns_duration_seconds",
			Help:    "Trace dns latency histogram.",
			Buckets: []float64{.005, .01, .025, .05, .1, .5, 1},
		},
		[]string{"event"},
	)
	tlsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "udsproxy_tls_duration_seconds",
			Help:    "Trace tls latency histogram.",
			Buckets: []float64{.02, .05, .07, .1, .2, .4},
		},
		[]string{"event"},
	)
	trace := &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}
	prometheus.MustRegister(tlsLatencyVec, dnsLatencyVec)
	return promhttp.InstrumentRoundTripperTrace(trace, transport)
}

func (p *ProxyInstance) startPrometheusMetricsServer() {
	log.Printf("Prometheus : http://localhost%s/metrics", p.Options.PrometheusPort)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(p.Options.PrometheusPort, nil))
}
