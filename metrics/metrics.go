package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	DocumentCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "shallowseek_documents_total",
		Help: "Total number of indexed documents",
	})

	SearchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "shallowseek_search_duration_seconds",
		Help:    "Duration of search operations in seconds",
		Buckets: prometheus.DefBuckets,
	})

	UploadDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "shallowseek_upload_duration_seconds",
		Help:    "Duration of file upload processing in seconds",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(DocumentCount)
	prometheus.MustRegister(SearchDuration)
	prometheus.MustRegister(UploadDuration)
} 