// Copyright 2026 Konstantin Lychkov. All rights reserved.
// Licensed under the Apache License, Version 2.0.

// Package reconciler provides helpers that translate an LLMDeployment CR into
// the concrete Kubernetes resources it manages (Deployment, Service, HPA).
package reconciler

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	llmv1 "github.com/DennisMRitchie/go-k8s-llm-operator/api/v1"
)

// ResourceReconciler builds and syncs child resources for an LLMDeployment.
type ResourceReconciler struct {
	Client client.Client
}

// -----------------------------------------------------------------------
// Deployment
// -----------------------------------------------------------------------

// ReconcileDeployment ensures a Deployment matching the LLMDeployment spec exists.
func (r *ResourceReconciler) ReconcileDeployment(ctx context.Context, llm *llmv1.LLMDeployment) error {
	log := log.FromContext(ctx)
	desired := r.buildDeployment(llm)

	existing := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating Deployment", "name", desired.Name)
		return r.Client.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	// Update replica count and image if they drift.
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template.Spec.Containers[0].Image = desired.Spec.Template.Spec.Containers[0].Image
	log.Info("Updating Deployment", "name", existing.Name)
	return r.Client.Update(ctx, existing)
}

func (r *ResourceReconciler) buildDeployment(llm *llmv1.LLMDeployment) *appsv1.Deployment {
	labels := map[string]string{
		"app":         llm.Name,
		"managed-by":  "llm-operator",
		"llm-model":   llm.Spec.ModelName,
	}

	containers := []corev1.Container{r.buildMainContainer(llm)}

	// Inject guardrails sidecar when requested.
	if llm.Spec.Guardrails != nil && llm.Spec.Guardrails.Enabled {
		containers = append(containers, r.buildGuardrailSidecar(llm.Spec.Guardrails))
	}

	podSpec := corev1.PodSpec{
		Containers: containers,
	}

	// Schedule on GPU nodes when required.
	if llm.Spec.GPURequired {
		podSpec.NodeSelector = map[string]string{
			"accelerator": "nvidia-tesla",
		}
		podSpec.Tolerations = []corev1.Toleration{
			{Key: "nvidia.com/gpu", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      llm.Name,
			Namespace: llm.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(llm, llmv1.GroupVersion.WithKind("LLMDeployment")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &llm.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}
}

func (r *ResourceReconciler) buildMainContainer(llm *llmv1.LLMDeployment) corev1.Container {
	c := corev1.Container{
		Name:            "llm-server",
		Image:           llm.Spec.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 11434, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{Name: "MODEL_NAME", Value: llm.Spec.ModelName},
			{Name: "TARGET_QPS", Value: fmt.Sprintf("%d", llm.Spec.TargetQPS)},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt32(11434),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       10,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt32(11434),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       20,
		},
	}

	if llm.Spec.GPURequired {
		c.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("1"),
			},
		}
	}

	return c
}

func (r *ResourceReconciler) buildGuardrailSidecar(cfg *llmv1.GuardrailsConfig) corev1.Container {
	img := cfg.Image
	if img == "" {
		img = "ghcr.io/dennisritchie/llm-guardrails:latest"
	}
	return corev1.Container{
		Name:  "guardrails",
		Image: img,
		Ports: []corev1.ContainerPort{
			{Name: "guardrails", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{Name: "BLOCK_PROMPT_INJECTION", Value: fmt.Sprintf("%v", cfg.BlockPromptInjection)},
			{Name: "MAX_TOKENS_PER_REQUEST", Value: fmt.Sprintf("%d", cfg.MaxTokensPerRequest)},
		},
	}
}

// -----------------------------------------------------------------------
// Service
// -----------------------------------------------------------------------

// ReconcileService ensures a ClusterIP Service exists for the LLMDeployment.
func (r *ResourceReconciler) ReconcileService(ctx context.Context, llm *llmv1.LLMDeployment) error {
	log := log.FromContext(ctx)
	desired := r.buildService(llm)

	existing := &corev1.Service{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating Service", "name", desired.Name)
		return r.Client.Create(ctx, desired)
	}
	return err
}

func (r *ResourceReconciler) buildService(llm *llmv1.LLMDeployment) *corev1.Service {
	labels := map[string]string{"app": llm.Name, "managed-by": "llm-operator"}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      llm.Name + "-svc",
			Namespace: llm.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(llm, llmv1.GroupVersion.WithKind("LLMDeployment")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": llm.Name},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt32(11434), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

// -----------------------------------------------------------------------
// HorizontalPodAutoscaler
// -----------------------------------------------------------------------

// ReconcileHPA ensures an HPA matching the LLMDeployment scaling policy exists.
func (r *ResourceReconciler) ReconcileHPA(ctx context.Context, llm *llmv1.LLMDeployment) error {
	log := log.FromContext(ctx)
	desired := r.buildHPA(llm)

	existing := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating HPA", "name", desired.Name)
		return r.Client.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get hpa: %w", err)
	}

	existing.Spec.MaxReplicas = desired.Spec.MaxReplicas
	existing.Spec.MinReplicas = desired.Spec.MinReplicas
	existing.Spec.Metrics = desired.Spec.Metrics
	log.Info("Updating HPA", "name", existing.Name)
	return r.Client.Update(ctx, existing)
}

func (r *ResourceReconciler) buildHPA(llm *llmv1.LLMDeployment) *autoscalingv2.HorizontalPodAutoscaler {
	minReplicas := llm.Spec.Replicas
	utilization := int32(80) // default CPU utilization target

	metrics := []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &utilization,
				},
			},
		},
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      llm.Name + "-hpa",
			Namespace: llm.Namespace,
			Labels:    map[string]string{"app": llm.Name, "managed-by": "llm-operator"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(llm, llmv1.GroupVersion.WithKind("LLMDeployment")),
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       llm.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: llm.Spec.MaxReplicas,
			Metrics:     metrics,
		},
	}
}
