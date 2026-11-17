package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AltinnResourceSpec defines the desired state of AltinnResource.
//
// +kubebuilder:validation:XValidation:rule="!has(self.optionalConfigMapData) || !self.requiredConfigMapData.exists(k, k in self.optionalConfigMapData)",message="keys in requiredConfigMapData and optionalConfigMapData must be unique"
type AltinnResourceSpec struct {
	// RequiredConfigMapData is a map of key-value pairs that must be present in the created ConfigMap.
	//
	// +kubebuilder:validation:Required
	RequiredConfigMapData map[string]string `json:"requiredConfigMapData"`

	// OptionalConfigMapData is a map of key-value pairs that may be present in the created ConfigMap.
	//
	// +kubebuilder:validation:Optional
	OptionalConfigMapData map[string]string `json:"optionalConfigMapData,omitempty"`
}

// AltinnResourceStatus defines the observed state of AltinnResource.
type AltinnResourceStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	Phase              Phase              `json:"phase,omitempty"`
	Ready              bool               `json:"ready"`
	Message            string             `json:"message,omitempty"`
}

type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseReady   Phase = "Ready"
	PhaseFailed  Phase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

// AltinnResource is the Schema for the altinnresources API.
type AltinnResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AltinnResourceSpec   `json:"spec,omitempty"`
	Status AltinnResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AltinnResourceList contains a list of AltinnResource.
type AltinnResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AltinnResource `json:"items"`
}

func (ar *AltinnResource) InitializeStatus() {
	if ar.Status.Conditions == nil {
		ar.Status.Conditions = []metav1.Condition{}
	}
	ar.Status.ObservedGeneration = ar.GetGeneration()
	ar.Status.Ready = false
	ar.Status.Phase = PhasePending
}
