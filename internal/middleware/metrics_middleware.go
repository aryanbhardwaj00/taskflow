package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts every HTTP request partitioned by HTTP method, matched route, and response status code.
	// Real-life analogy: The electronic toll gate counter registering every vehicle by type, lane, and payment status.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed by TaskFlow API.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks latency across defined duration buckets.
	// Real-life analogy: The stopwatch timing how long it takes a customer to be served at a bank teller.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of response latency in seconds for HTTP requests.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight tracks the instantaneous number of active concurrent requests.
	// Real-life analogy: The digital display outside a parking lot showing how many cars are inside right now.
	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being served concurrently.",
		},
	)
)

// PrometheusMetrics intercepts incoming HTTP requests to record latency, counts, and in-flight numbers.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Do not instrument the /metrics endpoint itself to avoid infinite recursive measurement loops
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// 1. Increment concurrent in-flight gauge
		HTTPRequestsInFlight.Inc()
		defer HTTPRequestsInFlight.Dec()

		// 2. Start latency stopwatch
		start := time.Now()

		// 3. Process the downstream handlers and route logic
		c.Next()

		// 4. Calculate elapsed duration
		duration := time.Since(start).Seconds()

		// 5. Extract route template (e.g. /api/v1/tasks/:id) rather than raw URL (/api/v1/tasks/123e4567...)
		// High Cardinality Protection: Using raw UUIDs would generate a new time series for every unique URL,
		// which exhausts Prometheus memory and slows down TSDB queries.
		path := c.FullPath()
		if path == "" {
			path = "unmatched_route"
		}

		status := strconv.Itoa(c.Writer.Status())

		// 6. Record measurements in Prometheus metrics
		HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
