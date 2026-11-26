package maskinportenschemaresource

import (
	"context"
	"fmt"

	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-altinnator-kartverket-no-v1alpha1-maskinportenschemaresource,mutating=false,failurePolicy=fail,sideEffects=None,groups=altinnator.kartverket.no,resources=maskinportenschemaresources,verbs=create,versions=v1alpha1,name=vmaskinportenschemaresource-v1alpha1.kb.io,admissionReviewVersions=v1
// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=maskinportenschemaresources,verbs=list

type Validator struct {
	reader client.Reader
}

var _ admission.Validator[*altinnatorv1alpha1.MaskinportenSchemaResource] = &Validator{}

// The API reader skips the cache, which could miss a resource created moments ago
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &altinnatorv1alpha1.MaskinportenSchemaResource{}).
		WithValidator(&Validator{reader: mgr.GetAPIReader()}).
		Complete()
}

func (v *Validator) ValidateCreate(
	ctx context.Context,
	resource *altinnatorv1alpha1.MaskinportenSchemaResource,
) (admission.Warnings, error) {
	var resources altinnatorv1alpha1.MaskinportenSchemaResourceList
	if err := v.reader.List(ctx, &resources); err != nil {
		return nil, fmt.Errorf("listing MaskinportenSchemaResources: %w", err)
	}
	for _, other := range resources.Items {
		if other.Spec.ResourceID == resource.Spec.ResourceID {
			return nil, fmt.Errorf("resourceId %s is already used by MaskinportenSchemaResource %s/%s",
				resource.Spec.ResourceID, other.Namespace, other.Name)
		}
	}
	return nil, nil
}

// The CRD keeps resourceId from changing, so an update can't take another resource's id
func (v *Validator) ValidateUpdate(
	_ context.Context,
	_, _ *altinnatorv1alpha1.MaskinportenSchemaResource,
) (admission.Warnings, error) {
	return nil, nil
}

func (v *Validator) ValidateDelete(
	_ context.Context,
	_ *altinnatorv1alpha1.MaskinportenSchemaResource,
) (admission.Warnings, error) {
	return nil, nil
}
