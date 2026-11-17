package reconciliation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kartverket/altinnator/pkg/log"
	"github.com/kartverket/altinnator/pkg/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RequiresCreateAction ReconcileAction = iota
	RequiresUpdateAction
	RequiresDeleteAction
	RequiresNoAction
)

type ReconcileAction int

func (reconcileAction ReconcileAction) Action() string {
	switch reconcileAction {
	case RequiresCreateAction:
		return "creation"
	case RequiresUpdateAction:
		return "update"
	case RequiresDeleteAction:
		return "deletion"
	case RequiresNoAction:
		return "no action"
	}
	panic("Unknown reconcile action")
}

type ControllerResource interface {
	Reconcile(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme) (ctrl.Result, error)
	GetResourceKind() string
	GetResourceName() string
	IsResourceNil() bool
}

type ReconcilerAdapter[T client.Object] struct {
	Func ResourceReconciler[T]
}

type ResourceReconciler[T client.Object] struct {
	ResourceKind    string
	ResourceName    string
	DesiredResource *T
	Scope           *model.Scope
	ShouldUpdate    func(current T, desired T) bool
	UpdateFields    func(current T, desired T)
}

func CountNonNilResources(rfs []ControllerResource) int {
	count := 0
	for _, rf := range rfs {
		if !rf.IsResourceNil() {
			count++
		}
	}
	return count
}

// ReconcileControllerResource reconciles a single Kubernetes resource towards its desired state while respecting
// ownership: Altinnator only mutates or deletes resources that are controlled by the AltinnResource in scope. Resources
// with a matching name that are not owned by the AltinnResource are left untouched (when desired is nil) or cause an
// error (when they would otherwise be created or updated).
func ReconcileControllerResource[T client.Object](
	ctx context.Context,
	k8sClient client.Client,
	scheme *runtime.Scheme,
	scope *model.Scope,
	resourceKind, resourceName string,
	desired *T,
	shouldUpdate func(current, desired T) bool,
	updateFields func(current, desired T),
) (ctrl.Result, error) {
	rLog := log.GetLogger(ctx)

	resourceType := reflect.TypeOf((*T)(nil)).Elem()
	current, _ := reflect.New(resourceType.Elem()).Interface().(T)
	current.SetNamespace(scope.AltinnResource.Namespace)
	current.SetName(resourceName)

	currentExists := true
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(current), current); err != nil {
		if apierrors.IsNotFound(err) {
			currentExists = false
		} else {
			errorReason := fmt.Sprintf(
				"Unable to get %s %s/%s.",
				resourceKind,
				current.GetNamespace(),
				current.GetName(),
			)
			scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
			return ctrl.Result{}, err
		}
	}

	currentIsOwned := metav1.IsControlledBy(current, &scope.AltinnResource)

	desiredIsNil := desired == nil || reflect.ValueOf(*desired).IsNil()

	rLog.Info(
		fmt.Sprintf("Determining reconcile action for %s %s/%s", resourceKind, current.GetNamespace(), current.GetName()),
	)
	reconcileAction, err := DetermineReconcileAction[T](
		current,
		desired,
		desiredIsNil,
		shouldUpdate,
		currentExists,
		currentIsOwned,
	)
	if err != nil {
		errorReason := fmt.Sprintf(
			"Failed to reconcile %s %s/%s: %s",
			resourceKind,
			current.GetNamespace(),
			current.GetName(),
			err,
		)
		scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, err
	}

	rLog.Info(
		fmt.Sprintf("%s %s/%s needs %s", resourceKind, current.GetNamespace(), current.GetName(), reconcileAction.Action()),
	)

	switch *reconcileAction {
	case RequiresDeleteAction:
		return reconcileOnDelete[T](rLog, ctx, k8sClient, scope, current, resourceKind, resourceName)
	case RequiresCreateAction:
		return reconcileOnCreate[T](rLog, ctx, scheme, scope, k8sClient, *desired, resourceKind, resourceName)
	case RequiresUpdateAction:
		return reconcileOnUpdate[T](rLog, ctx, k8sClient, scope, *desired, current, updateFields, resourceKind, resourceName)
	case RequiresNoAction:
		rLog.Debug(
			fmt.Sprintf("No action needed for %s %s/%s.", resourceKind, current.GetNamespace(), current.GetName()),
		)
		if !desiredIsNil {
			successMessage := fmt.Sprintf(
				"Successfully reconciled %s %s/%s.",
				resourceKind,
				current.GetNamespace(),
				resourceName,
			)
			rLog.Info(successMessage)
			scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, fmt.Errorf(
		"encountered unknown reconcile action when reconciling %s %s/%s",
		resourceKind,
		current.GetNamespace(),
		current.GetName(),
	)
}

// DetermineReconcileAction decides which action is required to bring the current resource towards the desired state,
// taking ownership into account:
//   - When the desired resource is nil, it is deleted only if it exists and is owned by the AltinnResource; otherwise
//     no action is taken (an unowned resource with a matching name is ignored).
//   - When the desired resource is not nil, a missing resource is created, an existing owned resource is updated when
//     shouldUpdate reports a difference, and an existing resource that is not owned by the AltinnResource results in an
//     error so Altinnator never overwrites resources it does not control.
func DetermineReconcileAction[T client.Object](
	current T,
	desired *T,
	isDesiredNil bool,
	shouldUpdate func(current, desired T) bool,
	currentExists bool,
	currentIsOwnedByAltinnResource bool,
) (*ReconcileAction, error) {
	if isDesiredNil {
		if currentExists && currentIsOwnedByAltinnResource {
			return new(RequiresDeleteAction), nil
		}
		return new(RequiresNoAction), nil
	}

	if !currentExists {
		return new(RequiresCreateAction), nil
	}

	if !currentIsOwnedByAltinnResource {
		return nil, fmt.Errorf(
			"cannot update %s/%s as it is not owned by AltinnResource",
			current.GetNamespace(),
			current.GetName(),
		)
	}

	if shouldUpdate(current, *desired) {
		return new(RequiresUpdateAction), nil
	}

	return new(RequiresNoAction), nil
}

func reconcileOnCreate[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	scheme *runtime.Scheme,
	scope *model.Scope,
	k8sClient client.Client,
	desired T,
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	rLog.Debug(
		fmt.Sprintf("%s %s/%s does not exist", resourceKind, desired.GetNamespace(), desired.GetName()),
	)

	if controllerRefErr := ctrl.SetControllerReference(&scope.AltinnResource, desired, scheme); controllerRefErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to set AltinnResource ownerReference on %s %s/%s.",
			resourceKind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, controllerRefErr
	}

	rLog.Info(
		fmt.Sprintf("Creating %s %s/%s", resourceKind, desired.GetNamespace(), desired.GetName()),
	)
	if createErr := k8sClient.Create(ctx, desired); createErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to create %s %s/%s",
			resourceKind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(desired, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, createErr
	}

	successMessage := fmt.Sprintf(
		"Successfully created %s %s/%s.",
		resourceKind,
		desired.GetNamespace(),
		desired.GetName(),
	)
	scope.ReplaceDescendant(desired, nil, &successMessage, resourceKind, resourceName)
	return ctrl.Result{}, nil
}

func reconcileOnUpdate[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	scope *model.Scope,
	desired T,
	current T,
	updateFields func(current, desired T),
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	rLog.Debug(
		fmt.Sprintf("Updating %s %s/%s with patch operation", resourceKind, desired.GetNamespace(), desired.GetName()),
	)
	before := current.DeepCopyObject().(client.Object)
	updateFields(current, desired)

	if patchErr := k8sClient.Patch(ctx, current, client.MergeFrom(before)); patchErr != nil {
		errorReason := fmt.Sprintf(
			"Unable to patch %s %s/%s",
			resourceKind,
			desired.GetNamespace(),
			desired.GetName(),
		)
		scope.ReplaceDescendant(current, &errorReason, nil, resourceKind, resourceName)
		return ctrl.Result{}, patchErr
	}

	successMessage := fmt.Sprintf(
		"Successfully updated %s %s/%s.",
		resourceKind,
		desired.GetNamespace(),
		desired.GetName(),
	)
	scope.ReplaceDescendant(current, nil, &successMessage, resourceKind, resourceName)
	return ctrl.Result{}, nil
}

func reconcileOnDelete[T client.Object](
	rLog log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	scope *model.Scope,
	current T,
	resourceKind, resourceName string,
) (ctrl.Result, error) {
	rLog.Info(
		fmt.Sprintf(
			"Desired %s %s/%s is nil and it is owned by AltinnResource %s/%s. Will try to delete it.",
			resourceKind,
			current.GetNamespace(),
			current.GetName(),
			scope.AltinnResource.Namespace,
			scope.AltinnResource.Name,
		),
	)

	if deleteErr := k8sClient.Delete(ctx, current); deleteErr != nil {
		deleteErrorMessage := fmt.Sprintf(
			"Failed to delete %s %s/%s",
			resourceKind,
			current.GetNamespace(),
			current.GetName(),
		)
		rLog.Error(deleteErr, deleteErrorMessage)
		scope.ReplaceDescendant(current, &deleteErrorMessage, nil, resourceKind, resourceName)
		return ctrl.Result{}, deleteErr
	}

	rLog.Debug(
		fmt.Sprintf("Successfully deleted %s %s/%s", resourceKind, current.GetNamespace(), current.GetName()),
	)
	return ctrl.Result{}, nil
}
