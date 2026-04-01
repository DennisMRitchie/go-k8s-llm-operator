// Copyright 2026 Konstantin Lychkov. All rights reserved.
// Licensed under the Apache License, Version 2.0.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// GroupVersion is group version used to register these objects.
var GroupVersion = schema.GroupVersion{Group: "ai.dennisritchie.dev", Version: "v1"}

// SchemeBuilder is used to add functions to this group's scheme.
var SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

// AddToScheme adds the types in this group-version to the given scheme.
var AddToScheme = SchemeBuilder.AddToScheme

// ScalingMetric defines which metric drives autoscaling.
// +kubebuilder:validation:Enum=QPS;Latency;GPU;CPU
type ScalingMetric string

const (
	ScalingMetricQPS     ScalingMetric = "QPS"
	ScalingMetricLatency ScalingMetric = "Latency"
	ScalingMetricGPU     ScalingMetric = "GPU"
	ScalingMetricCPU     ScalingMetric = "CPU"
)

// DeploymentPhase represents the lifecycle phase of the LLMDeployment.
type DeploymentPhase string

const (
	PhasePending  DeploymentPhase = "Pending"
	PhaseRunning  DeploymentPhase = "Running"
	PhaseScaling  DeploymentPhase = "Scaling"
	PhaseDegraded DeploymentPhase = "Degraded"
)

// GuardrailsConfig configures the sidecar guardrails container.
type GuardrailsConfig struct {
	// Enabled toggles the guardrails sidecar injection.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Image is the container image for the guardrails sidecar.
	// +optional
	Image string `json:"image,omitempty"`

	// MaxTokensPerRequest limits input token count per request.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxTokensPerRequest int32 `json:"maxTokensPerRequest,omitempty"`

	// BlockPromptInjection enables prompt-injection detection.
	// +kubebuilder:default=true
	// +optional
	BlockPromptInjection bool `json:"blockPromptInjection,omitempty"`
}

// PrometheusConfig holds Prometheus scrape settings.
type PrometheusConfig struct {
	// Enabled toggles Prometheus metric exposure.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Port on which metrics are exposed.
	// +kubebuilder:default=9090
	// +optional
	Port int32 `json:"port,omitempty"`

	// Path is the HTTP path for metrics.
	// +kubebuilder:default="/metrics"
	// +optional
	Path string `json:"path,omitempty"`
}

// LLMDeploymentSpec is the desired state of an LLMDeployment resource.
type LLMDeploymentSpec struct {
	// ModelName is the name of the LLM model to serve (e.g. "qwen3", "llama3").
	// +kubebuilder:validation:MinLength=1
	ModelName string `json:"modelName"`

	// Image is the container image that serves the model.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Replicas is the initial (and minimum) number of pods.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// MaxReplicas is the upper bound for HPA scale-out.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetQPS is the queries-per-second threshold that triggers scaling.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1000
	TargetQPS int32 `json:"targetQPS"`

	// TargetLatencyMs is the p99 latency (ms) threshold that triggers scaling.
	// +kubebuilder:validation:Minimum=1
	// +optional
	TargetLatencyMs int32 `json:"targetLatencyMs,omitempty"`

	// ScalingMetric selects which metric drives the HPA.
	// +kubebuilder:default=QPS
	// +optional
	ScalingMetric ScalingMetric `json:"scalingMetric,omitempty"`

	// GPURequired schedules pods on GPU nodes.
	// +kubebuilder:default=false
	GPURequired bool `json:"gpuRequired,omitempty"`

	// Guardrails configures the optional sidecar guardrails container.
	// +optional
	Guardrails *GuardrailsConfig `json:"guardrails,omitempty"`

	// Prometheus configures metrics exposure.
	// +optional
	Prometheus *PrometheusConfig `json:"prometheus,omitempty"`
}

// LLMDeploymentStatus is the observed state of an LLMDeployment resource.
type LLMDeploymentStatus struct {
	// ObservedReplicas is the current number of running pods.
	ObservedReplicas int32 `json:"observedReplicas,omitempty"`

	// CurrentQPS is the most recently measured queries-per-second.
	CurrentQPS int32 `json:"currentQPS,omitempty"`

	// CurrentLatencyMs is the most recently measured p99 latency (ms).
	CurrentLatencyMs int32 `json:"currentLatencyMs,omitempty"`

	// Phase summarises the overall lifecycle state.
	Phase DeploymentPhase `json:"phase,omitempty"`

	// LastScaled is the RFC3339 timestamp of the last scaling event.
	LastScaled string `json:"lastScaled,omitempty"`

	// Conditions is a list of standard Kubernetes status conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=llmd
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelName`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.observedReplicas`
// +kubebuilder:printcolumn:name="QPS",type=integer,JSONPath=`.status.currentQPS`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LLMDeployment is the Schema for the llmdeployments API.
type LLMDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMDeploymentSpec   `json:"spec,omitempty"`
	Status LLMDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LLMDeploymentList contains a list of LLMDeployment.
type LLMDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LLMDeployment `json:"items"`
}

// DeepCopyObject implements runtime.Object for LLMDeployment.
func (in *LLMDeployment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy creates a deep copy of LLMDeployment.
func (in *LLMDeployment) DeepCopy() *LLMDeployment {
	if in == nil {
		return nil
	}
	out := new(LLMDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all fields of LLMDeployment into out.
func (in *LLMDeployment) DeepCopyInto(out *LLMDeployment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyInto copies all fields of LLMDeploymentSpec into out.
func (in *LLMDeploymentSpec) DeepCopyInto(out *LLMDeploymentSpec) {
	*out = *in
	if in.Guardrails != nil {
		in, out := &in.Guardrails, &out.Guardrails
		*out = new(GuardrailsConfig)
		**out = **in
	}
	if in.Prometheus != nil {
		in, out := &in.Prometheus, &out.Prometheus
		*out = new(PrometheusConfig)
		**out = **in
	}
}

// DeepCopyInto copies all fields of LLMDeploymentStatus into out.
func (in *LLMDeploymentStatus) DeepCopyInto(out *LLMDeploymentStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyObject implements runtime.Object for LLMDeploymentList.
func (in *LLMDeploymentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy creates a deep copy of LLMDeploymentList.
func (in *LLMDeploymentList) DeepCopy() *LLMDeploymentList {
	if in == nil {
		return nil
	}
	out := new(LLMDeploymentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all fields of LLMDeploymentList into out.
func (in *LLMDeploymentList) DeepCopyInto(out *LLMDeploymentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LLMDeployment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func init() {
	SchemeBuilder.Register(&LLMDeployment{}, &LLMDeploymentList{})
}
