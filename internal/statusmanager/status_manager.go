package statusmanager

import (
	"context"
	"fmt"

	"github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/pkg/log"
	"github.com/kartverket/altinnator/pkg/model"
	"github.com/kartverket/altinnator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconciliationState int

// NB: used in switch cases, ensure they are exhaustive.
const (
	StatePending ReconciliationState = iota
	StateFailed
	StateReady
)

// UpdateAltinnResourceStatus builds the new status, compares with the original, and updates if changed.
func UpdateAltinnResourceStatus(
	ctx context.Context,
	k8sClient client.Client,
	recorder events.EventRecorder,
	scope *model.Scope,
	originalAltinnResource *v1alpha1.AltinnResource,
	controllerResources []reconciliation.ControllerResource,
) {
	sc := &scope.AltinnResource
	rLog := log.GetLogger(ctx)
	rLog.Debug(fmt.Sprintf("Updating AltinnResource status for %s/%s", sc.Namespace, sc.Name))
	recorder.Eventf(
		sc,
		nil,
		"Normal",
		"StatusUpdateStarted",
		"Reconcile",
		"Status update of AltinnResource started.")

	reconciliationState, err := DetermineReconciliationState(scope, controllerResources)
	if err != nil {
		rLog.Error(err, "Failed to determine reconciliation state")
		recorder.Eventf(
			sc,
			nil,
			"Warning",
			"StatusUpdateFailed",
			"Reconcile",
			fmt.Sprintf("Failed to determine reconciliation state: %v", err),
		)
		return
	}

	sc.Status.ObservedGeneration = sc.GetGeneration()
	sc.Status.Phase = determinePhase(*reconciliationState)
	sc.Status.Ready = determineReadiness(*reconciliationState)
	sc.Status.Message = statusMessage(*reconciliationState)
	sc.Status.Conditions = BuildConditions(
		sc,
		*reconciliationState,
		scope.Descendants,
		controllerResources,
		originalAltinnResource.Status.Conditions,
	)

	if !equality.Semantic.DeepEqual(originalAltinnResource.Status, sc.Status) {
		rLog.Debug(fmt.Sprintf("Updating AltinnResource status with name %s/%s", sc.Namespace, sc.Name))
		if err = UpdateStatus(ctx, k8sClient, *sc); err != nil {
			rLog.Error(
				err,
				fmt.Sprintf("Failed to update AltinnResource status with name %s/%s", sc.Namespace, sc.Name),
			)
			recorder.Eventf(sc, nil, "Warning", "StatusUpdateFailed", "Reconcile", "Status update of AltinnResource failed.")
		} else {
			recorder.Eventf(sc, nil, "Normal", "StatusUpdateSuccess", "Reconcile", "Status update of AltinnResource updated successfully.")
		}
	}
}

func DetermineReconciliationState(
	scope *model.Scope,
	controllerResources []reconciliation.ControllerResource,
) (*ReconciliationState, error) {
	switch {
	case len(scope.GetErrors()) > 0:
		return new(StateFailed), nil
	case len(scope.Descendants) != reconciliation.CountNonNilResources(controllerResources):
		return new(StatePending), nil
	}

	return new(StateReady), nil
}

func determinePhase(reconciliationState ReconciliationState) v1alpha1.Phase {
	switch reconciliationState {
	case StatePending:
		return v1alpha1.PhasePending
	case StateFailed:
		return v1alpha1.PhaseFailed
	case StateReady:
		return v1alpha1.PhaseReady
	}
	panic("could not determine phase")
}

func determineReadiness(reconciliationState ReconciliationState) bool {
	switch reconciliationState {
	case StatePending, StateFailed:
		return false
	case StateReady:
		return true
	}
	panic("could not determine readiness")
}

func statusMessage(reconciliationState ReconciliationState) string {
	switch reconciliationState {
	case StatePending:
		return "AltinnResource pending due to missing Descendants."
	case StateFailed:
		return "AltinnResource failed."
	case StateReady:
		return "AltinnResource ready."
	}
	panic("could not determine status message")
}
