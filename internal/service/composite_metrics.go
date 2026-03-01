package service

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	compositeRoutingTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "composite_model_routing_total",
			Help: "Total number of routing decisions for composite models",
		},
		[]string{"model", "strategy", "target_model"},
	)

	compositeLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "composite_model_latency_seconds",
			Help:    "Latency of composite model requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model", "backend_model"},
	)

	compositeFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "composite_model_fallback_total",
			Help: "Total number of fallbacks in composite models",
		},
		[]string{"model", "from_model", "to_model"},
	)

	compositeAggregationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "composite_model_aggregation_duration_seconds",
			Help:    "Duration of response aggregation",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model", "method"},
	)
)

// RecordCompositeRouting records a routing decision for a composite model
func RecordCompositeRouting(model, strategy, targetModel string) {
	compositeRoutingTotal.WithLabelValues(model, strategy, targetModel).Inc()
}

// RecordCompositeLatency records the latency of a composite model request
func RecordCompositeLatency(model, backendModel string, duration time.Duration) {
	compositeLatency.WithLabelValues(model, backendModel).Observe(duration.Seconds())
}

// RecordCompositeFallback records a fallback event in composite models
func RecordCompositeFallback(model, fromModel, toModel string) {
	compositeFallbackTotal.WithLabelValues(model, fromModel, toModel).Inc()
}

// RecordCompositeAggregation records the duration of response aggregation
func RecordCompositeAggregation(model string, method string, duration time.Duration) {
	compositeAggregationDuration.WithLabelValues(model, method).Observe(duration.Seconds())
}

func init() {
	prometheus.MustRegister(
		compositeRoutingTotal,
		compositeLatency,
		compositeFallbackTotal,
		compositeAggregationDuration,
	)
}
