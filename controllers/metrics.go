// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReconcilesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "carbon_aware_keda_scaler_reconciles_total",
			Help: "Total number of reconciles",
		},
		[]string{"app"},
	)

	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "carbon_aware_keda_scaler_reconcile_errors_total",
			Help: "Total number of reconcile errors",
		},
		[]string{"app"},
	)

	CarbonIntensityMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "carbon_aware_keda_scaler_carbon_intensity",
			Help: "Carbon intensity",
		},
		[]string{"app"},
	)

	DefaultMaxReplicasMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "carbon_aware_keda_scaler_default_max_replicas",
			Help: "Default max replicas",
		},
		[]string{"app"},
	)

	MaxReplicasMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "carbon_aware_keda_scaler_max_replicas",
			Help: "Max replicas",
		},
		[]string{"app"},
	)

	EcoModeOffMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "carbon_aware_keda_scaler_eco_mode_off",
			Help: "Eco mode off",
		},
		[]string{"app", "code"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(ReconcilesTotal)
	metrics.Registry.MustRegister(ReconcileErrorsTotal)
	metrics.Registry.MustRegister(CarbonIntensityMetric)
	metrics.Registry.MustRegister(DefaultMaxReplicasMetric)
	metrics.Registry.MustRegister(MaxReplicasMetric)
	metrics.Registry.MustRegister(EcoModeOffMetric)
}
