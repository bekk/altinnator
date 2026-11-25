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

	// ResourceID is the resource's identifier in Altinn. It cannot be changed once applied,
	// and it cannot start with app_ or se_, which Altinn keeps for Altinn Studio apps and Altinn 2 services.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9_-]{4,}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceId cannot be changed"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('app_') && !self.startsWith('se_')",message="resourceId cannot start with app_ or se_ as these prefixes are used by Altinn Studio apps and Altinn 2 services"
	ResourceID string `json:"resourceId"`

	// Homepage is a web page about the resource.
	//
	// +kubebuilder:validation:Optional
	Homepage URL `json:"homepage,omitempty"`

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
	// En is the text in English.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	En string `json:"en"`

	// Nb is the text in Norwegian Bokmål.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	Nb string `json:"nb"`

	// Nn is the text in Norwegian Nynorsk.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`\S`
	Nn string `json:"nn"`
}

// ContactPoint is a way to get help with the resource. Altinn may show contact points in service catalogues,
// see https://docs.altinn.studio/en/authorization/guides/resource-owner/create-resource-resource-admin/.
type ContactPoint struct {
	// Email is an email address to contact.
	//
	// +kubebuilder:validation:Optional
	Email Email `json:"email,omitempty"`

	// Telephone is a phone number to call.
	//
	// +kubebuilder:validation:Optional
	Telephone PhoneNumber `json:"telephone,omitempty"`

	// ContactPage is a link to a web page with contact information.
	//
	// +kubebuilder:validation:Optional
	ContactPage URL `json:"contactPage,omitempty"`

	// Category is a heading for the contact point's other fields that says what they are for,
	// such as "User support", "Feedback" or "Chat with us".
	//
	// +kubebuilder:validation:Optional
	Category string `json:"category,omitempty"`
}

// Keyword is a word to find the resource by in Altinn.
type Keyword struct {
	// Word is the keyword itself.
	//
	// +kubebuilder:validation:Optional
	Word string `json:"word,omitempty"`

	// Language is the language the word is in, such as nb, nn or en.
	//
	// +kubebuilder:validation:Optional
	Language string `json:"language,omitempty"`
}

// URL is an absolute http(s) URL.
//
// +kubebuilder:validation:MaxLength=255
// +kubebuilder:validation:Pattern=`^https?://[^\s/$.?#][^\s]*$`
type URL string

// Email is an email address.
//
// +kubebuilder:validation:Format=email
type Email string

// PhoneNumber is a phone number of digits, optionally starting with +, which may be grouped with spaces,
// hyphens and parentheses.
//
// +kubebuilder:validation:Pattern=`^\+?[0-9 ()-]*[0-9][0-9 ()-]*$`
type PhoneNumber string

// MaskinportenSchemaResourceStatus defines the observed state of MaskinportenSchemaResource.
type MaskinportenSchemaResourceStatus struct {
	// Conditions say whether the resource in Altinn matches the spec, and why not when it doesn't.
	//
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Resource ID",type=string,JSONPath=`.spec.resourceId`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`

// MaskinportenSchemaResource is a resource of type MaskinportenSchema in Altinn's Resource Registry.
type MaskinportenSchemaResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaskinportenSchemaResourceSpec   `json:"spec"`
	Status MaskinportenSchemaResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MaskinportenSchemaResourceList contains a list of MaskinportenSchemaResource.
type MaskinportenSchemaResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaskinportenSchemaResource `json:"items"`
}
