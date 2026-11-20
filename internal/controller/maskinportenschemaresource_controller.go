package controller

import (
	"context"

	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// MaskinportenSchemaResourceReconciler reconciles a MaskinportenSchemaResource object
type MaskinportenSchemaResourceReconciler struct {
	client.Client
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

// Reconcile only logs the MaskinportenSchemaResource for now.
func (r *MaskinportenSchemaResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	resource := new(altinnatorv1alpha1.MaskinportenSchemaResource)
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug("MaskinportenSchemaResource not found. Probably a delete.")
			return ctrl.Result{}, nil
		}
		rLog.Error(err, "Failed to get MaskinportenSchemaResource")
		return ctrl.Result{}, err
	}

	rLog.Info("Received MaskinportenSchemaResource",
		"resourceId", resource.Spec.ResourceID,
		"scopes", resource.Spec.Scopes,
		"title", resource.Spec.Title.Nb,
	)
	return ctrl.Result{}, nil
}
