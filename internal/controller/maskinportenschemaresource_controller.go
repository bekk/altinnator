package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/pkg/altinn"
	"github.com/kartverket/altinnator/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const readyCondition = "Ready"

// Changes made directly in Altinn cause no event in the cluster, so Altinn is read again this often
const driftCheckInterval = time.Hour

// registry is what the controller needs from Altinn's Resource Registry
type registry interface {
	GetMaskinportenSchema(ctx context.Context, id string) (*altinn.MaskinportenSchema, error)
	CreateMaskinportenSchema(ctx context.Context, id string, content altinn.MaskinportenSchemaContent) error
	UpdateMaskinportenSchema(ctx context.Context, id string, content altinn.MaskinportenSchemaContent) error
	SetMaskinportenSchemaPolicy(ctx context.Context, id string) error
}

// MaskinportenSchemaResourceReconciler reconciles a MaskinportenSchemaResource object
type MaskinportenSchemaResourceReconciler struct {
	client.Client
	Registry registry
}

// SetupWithManager sets up the controller with the Manager.
func (r *MaskinportenSchemaResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&altinnatorv1alpha1.MaskinportenSchemaResource{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=maskinportenschemaresources,verbs=get;list;watch
// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=maskinportenschemaresources/status,verbs=get;update;patch

// Reconcile makes the resource in Altinn match the MaskinportenSchemaResource, and shows how that went in its Ready
// condition.
func (r *MaskinportenSchemaResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	resource := new(altinnatorv1alpha1.MaskinportenSchemaResource)
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug("MaskinportenSchemaResource not found. Probably a delete.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get MaskinportenSchemaResource: %w", err)
	}

	syncErr := r.sync(ctx, resource)
	if err := r.report(ctx, resource, syncErr); err != nil {
		return ctrl.Result{}, err
	}
	// Altinn will never take the resource as it is, and the status says why
	if syncErr != nil {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: driftCheckInterval}, nil
}

func (r *MaskinportenSchemaResourceReconciler) sync(
	ctx context.Context,
	resource *altinnatorv1alpha1.MaskinportenSchemaResource,
) error {
	id := resource.Spec.ResourceID
	want := contentFrom(resource.Spec)
	have, err := r.Registry.GetMaskinportenSchema(ctx, id)
	switch {
	case errors.Is(err, altinn.ErrNotFound):
		err = r.Registry.CreateMaskinportenSchema(ctx, id, want)
	case err == nil && differs(want, have):
		err = r.Registry.UpdateMaskinportenSchema(ctx, id, want)
	}
	if err != nil {
		return err
	}
	// Setting the policy can fail after the resource is created, and the resource then matches the spec on the next
	// try. Setting it until the resource is ready makes sure it is set.
	if !meta.IsStatusConditionTrue(resource.Status.Conditions, readyCondition) {
		return r.Registry.SetMaskinportenSchemaPolicy(ctx, id)
	}
	return nil
}

// report returns err only when trying again may help
func (r *MaskinportenSchemaResourceReconciler) report(
	ctx context.Context,
	resource *altinnatorv1alpha1.MaskinportenSchemaResource,
	err error,
) error {
	ready := metav1.Condition{
		Type:               readyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            "The resource in Altinn matches the spec",
		ObservedGeneration: resource.Generation,
	}
	switch {
	case errors.Is(err, altinn.ErrRejected):
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, "Rejected", err.Error()
		err = nil
	case errors.Is(err, altinn.ErrTaken):
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, "Taken", err.Error()
		err = nil
	case err != nil:
		ready.Status, ready.Reason, ready.Message = metav1.ConditionFalse, "Failed", err.Error()
	}
	if !meta.SetStatusCondition(&resource.Status.Conditions, ready) {
		return err
	}
	return errors.Join(err, r.Status().Update(ctx, resource))
}

func contentFrom(spec altinnatorv1alpha1.MaskinportenSchemaResourceSpec) altinn.MaskinportenSchemaContent {
	content := altinn.MaskinportenSchemaContent{
		Scopes:                spec.Scopes,
		Title:                 altinn.MultilingualText(spec.Title),
		Description:           altinn.MultilingualText(spec.Description),
		DelegationDescription: altinn.MultilingualText(spec.DelegationDescription),
		Homepage:              string(spec.Homepage),
	}
	for _, point := range spec.ContactPoints {
		content.ContactPoints = append(content.ContactPoints, altinn.ContactPoint{
			Category:    point.Category,
			Email:       string(point.Email),
			Telephone:   string(point.Telephone),
			ContactPage: string(point.ContactPage),
		})
	}
	for _, keyword := range spec.Keywords {
		content.Keywords = append(content.Keywords, altinn.Keyword(keyword))
	}
	return content
}

// slices.Equal counts a missing list and an empty one as the same, so an empty list from Altinn is no change
func differs(want altinn.MaskinportenSchemaContent, have *altinn.MaskinportenSchema) bool {
	return !slices.Equal(want.Scopes, have.Scopes) ||
		want.Title != have.Title ||
		want.Description != have.Description ||
		want.DelegationDescription != have.DelegationDescription ||
		want.Homepage != have.Homepage ||
		!slices.Equal(want.ContactPoints, have.ContactPoints) ||
		!slices.Equal(want.Keywords, have.Keywords)
}
