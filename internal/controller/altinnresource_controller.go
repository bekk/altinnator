package controller

import (
	"context"
	"fmt"
	"maps"
	"time"

	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/internal/reconciler"
	"github.com/kartverket/altinnator/internal/statusmanager"
	"github.com/kartverket/altinnator/pkg/envvar"
	"github.com/kartverket/altinnator/pkg/labels"
	"github.com/kartverket/altinnator/pkg/log"
	"github.com/kartverket/altinnator/pkg/model"
	"github.com/kartverket/altinnator/pkg/reconciliation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sErrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AltinnResourceReconciler reconciles a AltinnResource object
type AltinnResourceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *AltinnResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&altinnatorv1alpha1.AltinnResource{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=altinnresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=altinnresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=altinnator.kartverket.no,resources=altinnresources/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AltinnResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	altinnResource := new(altinnatorv1alpha1.AltinnResource)

	rLog.Info(fmt.Sprintf("Received reconcile request for AltinnResource with name %s", req.String()))

	if err := r.Get(ctx, req.NamespacedName, altinnResource); err != nil {
		if apierrors.IsNotFound(err) {
			rLog.Debug(
				fmt.Sprintf("AltinnResource with name %s not found. Probably a delete.", req.String()),
			)
			return reconcile.Result{}, nil
		}
		rLog.Error(err, fmt.Sprintf("Failed to get AltinnResource with name %s", req.String()))
		return reconcile.Result{}, err
	}

	rLog.Debug(fmt.Sprintf("AltinnResource with name %s found", req.String()))

	altinnResource.InitializeStatus()
	originalAltinnResource := altinnResource.DeepCopy()

	if !altinnResource.DeletionTimestamp.IsZero() {
		rLog.Info(fmt.Sprintf("Deleting AltinnResource with name %s", req.String()))
		return ctrl.Result{}, nil
	}

	if originalAltinnResource.Labels == nil {
		originalAltinnResource.Labels = make(map[string]string)
	}
	maps.Copy(originalAltinnResource.Labels, labels.AltinnResourceStandardLabels())
	if !maps.Equal(originalAltinnResource.Labels, altinnResource.Labels) {
		err := r.Patch(ctx, originalAltinnResource, client.MergeFrom(altinnResource))
		if err != nil {
			return r.failReconcile(ctx, altinnResource, err, "failed to patch AltinnResource with AltinnResource standard labels")
		}
	}

	mergedConfigMapData := make(
		map[string]string,
		len(envvar.Get().AdditionalConfigMapData)+len(altinnResource.Spec.RequiredConfigMapData)+len(altinnResource.Spec.OptionalConfigMapData),
	)
	maps.Copy(mergedConfigMapData, envvar.Get().AdditionalConfigMapData)
	maps.Copy(mergedConfigMapData, altinnResource.Spec.RequiredConfigMapData)
	maps.Copy(mergedConfigMapData, altinnResource.Spec.OptionalConfigMapData)

	scope := &model.Scope{
		AltinnResource: *altinnResource,
		DummyData:      mergedConfigMapData,
	}

	controllerResources := reconciler.ControllerResources(scope)

	defer func() {
		statusmanager.UpdateAltinnResourceStatus(ctx, r.Client, r.Recorder, scope, originalAltinnResource, controllerResources)
	}()

	return r.doReconcile(ctx, controllerResources, scope)
}

func (r *AltinnResourceReconciler) doReconcile(
	ctx context.Context,
	reconcileFuncs []reconciliation.ControllerResource,
	scope *model.Scope,
) (ctrl.Result, error) {
	result := ctrl.Result{}
	var errs []error
	for _, rf := range reconcileFuncs {
		reconcileResult, err := rf.Reconcile(ctx, r.Client, r.Scheme)
		if err != nil {
			r.Recorder.Eventf(
				&scope.AltinnResource,
				nil,
				"Warning",
				fmt.Sprintf("%sReconcileFailed", rf.GetResourceKind()),
				"Reconcile",
				"%s with name %s failed during reconciliation.", rf.GetResourceKind(), rf.GetResourceName(),
			)
			errs = append(errs, err)
		} else {
			r.Recorder.Eventf(
				&scope.AltinnResource,
				nil,
				"Normal",
				fmt.Sprintf("%sReconciledSuccessfully", rf.GetResourceKind()),
				"Reconcile",
				"%s with name %s reconciled successfully.", rf.GetResourceKind(), rf.GetResourceName(),
			)
		}
		if len(errs) > 0 {
			continue
		}
		result = lowestNonZeroResult(result, reconcileResult)
	}

	if len(errs) > 0 {
		r.Recorder.Eventf(
			&scope.AltinnResource,
			nil,
			"Warning",
			"ReconcileFailed",
			"Reconcile",
			"AltinnResource failed during reconciliation",
		)
		return ctrl.Result{}, k8sErrors.NewAggregate(errs)
	}
	r.Recorder.Eventf(
		&scope.AltinnResource,
		nil,
		"Normal",
		"ReconcileSuccess",
		"Reconcile",
		"AltinnResource reconciled successfully",
	)
	return result, nil
}

func lowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero() && j.IsZero():
		return ctrl.Result{}
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.RequeueAfter != 0 && j.RequeueAfter != 0:
		if i.RequeueAfter < j.RequeueAfter {
			return i
		}
		return j
	case i.RequeueAfter != 0:
		return i
	case j.RequeueAfter != 0:
		return j
	default:
		return ctrl.Result{RequeueAfter: 0 * time.Second}
	}
}

func (r *AltinnResourceReconciler) failReconcile(
	ctx context.Context,
	altinnResource *altinnatorv1alpha1.AltinnResource,
	err error,
	errMsg string,
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)
	rLog.Error(err, errMsg)
	altinnResource.Status.Phase = altinnatorv1alpha1.PhaseFailed
	altinnResource.Status.Message = errMsg
	if updateErr := statusmanager.UpdateStatus(ctx, r.Client, *altinnResource); updateErr != nil {
		return ctrl.Result{}, updateErr
	}
	return ctrl.Result{}, err
}
