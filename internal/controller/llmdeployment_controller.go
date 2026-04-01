// Copyright 2026 Konstantin Lychkov. All rights reserved.
// Licensed under the Apache License, Version 2.0.

// Package controller implements the LLMDeployment reconcile loop.
package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	llmv1 "github.com/DennisMRitchie/go-k8s-llm-operator/api/v1"
	"github.com/DennisMRitchie/go-k8s-llm-operator/internal/metrics"
	"github.com/DennisMRitchie/go-k8s-llm-operator/internal/reconciler"
)

const (
	finalizerName   = "ai.dennisritchie.dev/finalizer"
	requeueInterval = 30 * time.Second
)

// LLMDeploymentReconciler reconciles LLMDeployment resources.
//
// +kubebuilder:rbac:groups=ai.dennisritchie.dev,resources=llmdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ai.dennisritchie.dev,resources=llmdeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ai.dennisritchie.dev,resources=llmdeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
type LLMDeploymentReconciler struct {
	client.Client
	ResourceReconciler *reconciler.ResourceReconciler
}

// Reconcile is the main reconciliation loop called by controller-runtime.
func (r *LLMDeploymentReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	start := time.Now()

	// -------------------------------------------------------------------
	// 1. Fetch the LLMDeployment CR
	// -------------------------------------------------------------------
	llm := &llmv1.LLMDeployment{}
	if err := r.Get(ctx, req.NamespacedName, llm); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// -------------------------------------------------------------------
	// 2. Finalizer handling (cleanup on deletion)
	// -------------------------------------------------------------------
	if !llm.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, llm)
	}

	if !controllerutil.ContainsFinalizer(llm, finalizerName) {
		controllerutil.AddFinalizer(llm, finalizerName)
		if err := r.Update(ctx, llm); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// -------------------------------------------------------------------
	// 3. Reconcile child resources
	// -------------------------------------------------------------------
	if err := r.ResourceReconciler.ReconcileDeployment(ctx, llm); err != nil {
		return r.setDegradedStatus(ctx, llm, "DeploymentFailed", err)
	}

	if err := r.ResourceReconciler.ReconcileService(ctx, llm); err != nil {
		return r.setDegradedStatus(ctx, llm, "ServiceFailed", err)
	}

	if err := r.ResourceReconciler.ReconcileHPA(ctx, llm); err != nil {
		return r.setDegradedStatus(ctx, llm, "HPAFailed", err)
	}

	// -------------------------------------------------------------------
	// 4. Autoscaling decision (operator-side QPS/latency gate)
	// -------------------------------------------------------------------
	scaled := r.autoscale(ctx, llm)

	// -------------------------------------------------------------------
	// 5. Sync observed replica count from the child Deployment
	// -------------------------------------------------------------------
	if err := r.syncObservedReplicas(ctx, llm); err != nil {
		log.Error(err, "Failed to sync observed replicas")
	}

	// -------------------------------------------------------------------
	// 6. Update status
	// -------------------------------------------------------------------
	phase := llmv1.PhaseRunning
	if scaled {
		phase = llmv1.PhaseScaling
	}
	llm.Status.Phase = phase

	meta.SetStatusCondition(&llm.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "LLMDeployment successfully reconciled",
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, llm); err != nil {
		log.Error(err, "Failed to update status")
	}

	// -------------------------------------------------------------------
	// 7. Expose Prometheus metrics
	// -------------------------------------------------------------------
	metrics.ReconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	metrics.ReconcileDuration.WithLabelValues(req.Namespace, req.Name).Observe(time.Since(start).Seconds())
	metrics.LLMDeploymentReplicas.WithLabelValues(req.Namespace, req.Name, llm.Spec.ModelName).Set(float64(llm.Status.ObservedReplicas))
	metrics.LLMDeploymentQPS.WithLabelValues(req.Namespace, req.Name, llm.Spec.ModelName).Set(float64(llm.Status.CurrentQPS))
	metrics.LLMDeploymentLatency.WithLabelValues(req.Namespace, req.Name, llm.Spec.ModelName).Set(float64(llm.Status.CurrentLatencyMs))

	log.Info("Reconcile complete",
		"phase", phase,
		"replicas", llm.Spec.Replicas,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// autoscale applies operator-side scaling logic on top of HPA.
// Returns true when a scaling event was triggered.
func (r *LLMDeploymentReconciler) autoscale(ctx context.Context, llm *llmv1.LLMDeployment) bool {
	log := log.FromContext(ctx)
	scaled := false

	qpsBreached := llm.Status.CurrentQPS > llm.Spec.TargetQPS
	latencyBreached := llm.Spec.TargetLatencyMs > 0 && llm.Status.CurrentLatencyMs > llm.Spec.TargetLatencyMs

	if (qpsBreached || latencyBreached) && llm.Spec.Replicas < llm.Spec.MaxReplicas {
		llm.Spec.Replicas++
		llm.Status.LastScaled = time.Now().Format(time.RFC3339)
		metrics.ScaleEvents.WithLabelValues(llm.Namespace, llm.Name, "up").Inc()
		log.Info("Scale-up triggered",
			"newReplicas", llm.Spec.Replicas,
			"currentQPS", llm.Status.CurrentQPS,
			"targetQPS", llm.Spec.TargetQPS,
		)
		scaled = true
	}

	// Scale down when QPS is well below threshold (50% headroom) and >1 replica.
	if !qpsBreached && !latencyBreached &&
		llm.Status.CurrentQPS < llm.Spec.TargetQPS/2 &&
		llm.Spec.Replicas > 1 {
		llm.Spec.Replicas--
		llm.Status.LastScaled = time.Now().Format(time.RFC3339)
		metrics.ScaleEvents.WithLabelValues(llm.Namespace, llm.Name, "down").Inc()
		log.Info("Scale-down triggered", "newReplicas", llm.Spec.Replicas)
		scaled = true
	}

	return scaled
}

// syncObservedReplicas reads the child Deployment and writes the available
// replica count back into the CR status.
func (r *LLMDeploymentReconciler) syncObservedReplicas(ctx context.Context, llm *llmv1.LLMDeployment) error {
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: llm.Name, Namespace: llm.Namespace}, dep); err != nil {
		return err
	}
	llm.Status.ObservedReplicas = dep.Status.AvailableReplicas
	return nil
}

// setDegradedStatus writes a Degraded phase + condition and returns the error.
func (r *LLMDeploymentReconciler) setDegradedStatus(
	ctx context.Context,
	llm *llmv1.LLMDeployment,
	reason string,
	err error,
) (reconcile.Result, error) {
	llm.Status.Phase = llmv1.PhaseDegraded
	meta.SetStatusCondition(&llm.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Update(ctx, llm)
	metrics.ReconcileTotal.WithLabelValues(llm.Namespace, llm.Name, "error").Inc()
	return reconcile.Result{}, fmt.Errorf("%s: %w", reason, err)
}

// handleDeletion runs cleanup logic before the CR is removed.
func (r *LLMDeploymentReconciler) handleDeletion(ctx context.Context, llm *llmv1.LLMDeployment) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling deletion of LLMDeployment", "name", llm.Name)

	// Child resources are garbage-collected automatically via owner references.
	// Remove the finalizer so the CR can be deleted.
	controllerutil.RemoveFinalizer(llm, finalizerName)
	if err := r.Update(ctx, llm); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// SetupWithManager registers the controller with the manager and sets up watches.
func (r *LLMDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return builder.ControllerManagedBy(mgr).
		For(&llmv1.LLMDeployment{}).
		Owns(&appsv1.Deployment{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
