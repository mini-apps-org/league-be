package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registered on the default registry, which promhttp.Handler serves on the
// metrics port.
var (
	httpRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests served, by route template, method and status code.",
	}, []string{"route", "method", "code"})

	httpRequestDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time to serve an HTTP request, by route template and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})
)

// RecordRequestMetrics counts and times every matched request. The route label
// is the template (/api/v1/roles/{id}), never the raw path: one series per id
// would grow without bound.
func (m *middleware) RecordRequestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeLabel := prometheus.Labels{"route": routeTemplate(r)}

		counted := promhttp.InstrumentHandlerCounter(httpRequestCounter.MustCurryWith(routeLabel), next)
		promhttp.InstrumentHandlerDuration(httpRequestDurationHistogram.MustCurryWith(routeLabel), counted).ServeHTTP(w, r)
	})
}

func routeTemplate(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return "unmatched"
	}
	template, err := route.GetPathTemplate()
	if err != nil {
		return "unmatched"
	}
	return template
}
