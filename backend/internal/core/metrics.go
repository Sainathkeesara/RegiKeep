package core

import "github.com/prometheus/client_golang/prometheus"

var (
	KeepaliveSuccessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regikeep_keepalive_success_total",
			Help: "Total successful keepalive attempts",
		},
		[]string{"registry", "strategy"},
	)

	KeepaliveFailureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "regikeep_keepalive_failure_total",
			Help: "Total failed keepalive attempts",
		},
		[]string{"registry", "strategy"},
	)

	KeepaliveLastSuccessTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "regikeep_keepalive_last_success_timestamp",
			Help: "Unix timestamp of the last successful keepalive per image",
		},
		[]string{"image_id"},
	)

	ArchiveBytesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "regikeep_archive_bytes_total",
			Help: "Total bytes sent to cold archive storage",
		},
	)
)

func init() {
	prometheus.MustRegister(
		KeepaliveSuccessTotal,
		KeepaliveFailureTotal,
		KeepaliveLastSuccessTimestamp,
		ArchiveBytesTotal,
	)
}
