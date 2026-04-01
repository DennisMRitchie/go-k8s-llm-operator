// Copyright 2026 Konstantin Lychkov. All rights reserved.
// Licensed under the Apache License, Version 2.0.

package main

import (
	"flag"
	"log"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	llmv1 "github.com/DennisMRitchie/go-k8s-llm-operator/api/v1"
	"github.com/DennisMRitchie/go-k8s-llm-operator/internal/controller"
	"github.com/DennisMRitchie/go-k8s-llm-operator/internal/reconciler"

	// Ensure metrics are registered on startup.
	_ "github.com/DennisMRitchie/go-k8s-llm-operator/internal/metrics"
)

func main() {
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8084", "Address for the Prometheus /metrics endpoint.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for liveness and readiness probes.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for HA deployments.")
	flag.Parse()

	// Structured logging via zap.
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "llm-operator-leader-election",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Register CRD types with the manager's scheme.
	if err := llmv1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add LLMDeployment types to scheme")
		os.Exit(1)
	}

	// Wire up the reconciler.
	resReconciler := &reconciler.ResourceReconciler{Client: mgr.GetClient()}

	if err := (&controller.LLMDeploymentReconciler{
		Client:             mgr.GetClient(),
		ResourceReconciler: resReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LLMDeployment")
		os.Exit(1)
	}

	// Health probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatal("unable to set up health check:", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatal("unable to set up ready check:", err)
	}

	setupLog.Info("🚀 Go K8s LLM Operator started",
		"metricsAddr", metricsAddr,
		"probeAddr", probeAddr,
		"leaderElection", enableLeaderElection,
	)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
