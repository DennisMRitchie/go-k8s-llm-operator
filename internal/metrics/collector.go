// Copyright 2026 Konstantin Lychkov. All rights reserved.
// Licensed under the Apache License, Version 2.0.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ReconcileTotal counts total reconcile calls, labelled by result.
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llmoperator",
			Subsystem: "controller",
			Name:      "reconcile_total",
			Help:      "Total number of reconcile calls partitioned by result (success/error).",
		},
		[]string{"namespace", "name", "result"},
	)

	// ReconcileDuration tracks reconcile loop duration.
	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "llmoperator",
			Subsystem: "controller",
			Name:      "reconcile_duration_seconds",
			Help:      "Duration in seconds of the reconcile loop.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)

	// LLMDeploymentReplicas is the current observed replica count per LLMDeployment.
	LLMDeploymentReplicas = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "llmoperator",
			Subsystem: "deployment",
			Name:      "replicas",
			Help:      "Current observed replica count for each LLMDeployment.",
		},
		[]string{"namespace", "name", "model"},
	)

	// LLMDeploymentQPS is the observed QPS per LLMDeployment.
	LLMDeploymentQPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "llmoperator",
			Subsystem: "deployment",
			Name:      "current_qps",
			Help:      "Current observed QPS for each LLMDeployment.",
		},
		[]string{"namespace", "name", "model"},
	)

	// LLMDeploymentLatency is the observed p99 latency per LLMDeployment.
	LLMDeploymentLatency = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "llmoperator",
			Subsystem: "deployment",
			Name:      "latency_p99_ms",
			Help:      "Current p99 latency (ms) for each LLMDeployment.",
		},
		[]string{"namespace", "name", "model"},
	)

	// ScaleEvents counts autoscaling events, labelled by direction (up/down).
	ScaleEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llmoperator",
			Subsystem: "autoscaler",
			Name:      "scale_events_total",
			Help:      "Total autoscaling events partitioned by direction.",
		},
		[]string{"namespace", "name", "direction"},
	)
)

func init() {
	// Register all metrics with the controller-runtime global registry so they
	// are exposed automatically on the /metrics endpoint.
	metrics.Registry.MustRegister(
		ReconcileTotal,
		ReconcileDuration,
		LLMDeploymentReplicas,
		LLMDeploymentQPS,
		LLMDeploymentLatency,
		ScaleEvents,
	)
}
