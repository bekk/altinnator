package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MaskinportenSchemaResourceSpec defines the desired state of MaskinportenSchemaResource.
type MaskinportenSchemaResourceSpec struct {
	// Scopes are the Maskinporten scopes the resource gives access to. No OAuth scope can contain whitespace.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:Pattern=`^\S+$`
	// +listType=set
	Scopes []string `json:"scopes"`

	// DelegationDescription says what is given when access is delegated.
	//
	// +kubebuilder:validation:Required
	DelegationDescription MultilingualText `json:"delegationDescription"`

	// Title is the resource's name in Altinn.
	//
	// +kubebuilder:validation:Required
	Title MultilingualText `json:"title"`

	// Description says what the resource is.
	//
	// +kubebuilder:validation:Required
	Description MultilingualText `json:"description"`

	// ResourceID is the resource's identifier in Altinn. It cannot be changed.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9_-]{4,}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceId cannot be changed"
	ResourceID string `json:"resourceId"`

	// Homepage is a web page about the resource.
	//
	// +kubebuilder:validation:Optional
	Homepage string `json:"homepage,omitempty"`

	// ContactPoints are where to get help with the resource.
	//
	// +kubebuilder:validation:Optional
	ContactPoints []ContactPoint `json:"contactPoints,omitempty"`

	// Keywords are words to find the resource by in Altinn.
	//
	// +kubebuilder:validation:Optional
	Keywords []Keyword `json:"keywords,omitempty"`
}

// MultilingualText is a text in the three languages Altinn requires.
// Altinn counts a text that is only whitespace as missing.
type MultilingualText struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	En string `json:"en"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	Nb string `json:"nb"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	Nn string `json:"nn"`
}

// ContactPoint is a way to get help with the resource.
type ContactPoint struct {
	// +kubebuilder:validation:Optional
	Email string `json:"email,omitempty"`

	// +kubebuilder:validation:Optional
	Telephone string `json:"telephone,omitempty"`

	// +kubebuilder:validation:Optional
	ContactPage string `json:"contactpage,omitempty"`

	// Category is what the contact point is, such as a customer centre.
	//
	// +kubebuilder:validation:Optional
	Category string `json:"category,omitempty"`
}

// Keyword is a word to find the resource by in Altinn.
type Keyword struct {
	// +kubebuilder:validation:Optional
	Word string `json:"word,omitempty"`

	// +kubebuilder:validation:Optional
	Language string `json:"language,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Resource ID",type=string,JSONPath=`.spec.resourceId`

// MaskinportenSchemaResource is a resource of type MaskinportenSchema in Altinn's Resource Registry.
type MaskinportenSchemaResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MaskinportenSchemaResourceSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// MaskinportenSchemaResourceList contains a list of MaskinportenSchemaResource.
type MaskinportenSchemaResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaskinportenSchemaResource `json:"items"`
}
