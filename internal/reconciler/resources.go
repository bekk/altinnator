package reconciler

import (
	"github.com/kartverket/altinnator/internal/names"
	"github.com/kartverket/altinnator/pkg/labels"
	"github.com/kartverket/altinnator/pkg/model"
	"github.com/kartverket/altinnator/pkg/reconciliation"
	"github.com/kartverket/altinnator/pkg/resourcegenerator/configmap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerResources creates all reconcile actions for the given AltinnResource scope.
func ControllerResources(scope *model.Scope) []reconciliation.ControllerResource {
	return []reconciliation.ControllerResource{
		dummyConfigMap(scope),
	}
}

func dummyConfigMap(scope *model.Scope) ControllerResourceAdapter[*corev1.ConfigMap] {
	configMapName := names.NewDummyNamer(scope.AltinnResource).ConfigMapName()
	desiredResource := configmap.GetDesired(
		buildObjectMeta(configMapName, scope.AltinnResource.Namespace),
		scope.DummyData,
	)

	return ControllerResourceAdapter[*corev1.ConfigMap]{
		reconciliation.ReconcilerAdapter[*corev1.ConfigMap]{
			Func: reconciliation.ResourceReconciler[*corev1.ConfigMap]{
				ResourceKind:    "ConfigMap",
				ResourceName:    configMapName,
				DesiredResource: new(desiredResource),
				Scope:           scope,
				ShouldUpdate:    ConfigMapShouldUpdateFunc,
				UpdateFields:    configMapUpdateFieldsFunc,
			},
		},
	}
}

func ConfigMapShouldUpdateFunc(current, desired *corev1.ConfigMap) bool {
	return !equality.Semantic.DeepEqual(current.BinaryData, desired.BinaryData) ||
		!equality.Semantic.DeepEqual(current.Data, desired.Data) ||
		!equality.Semantic.DeepEqual(current.Immutable, desired.Immutable) ||
		labelsNeedUpdate(current, desired) ||
		annotationsNeedUpdate(current, desired)
}

func configMapUpdateFieldsFunc(current, desired *corev1.ConfigMap) {
	current.BinaryData = desired.BinaryData
	current.Data = desired.Data
	current.Immutable = desired.Immutable
	current.Labels = desired.Labels
	current.Annotations = desired.Annotations
}

type LabeledObject interface {
	GetLabels() map[string]string
}

// labelsNeedUpdate reports whether any of the desired labels are missing from current or have a
// different value. Labels on current that are not part of the desired set are ignored.
func labelsNeedUpdate(current, desired LabeledObject) bool {
	desiredLabels := desired.GetLabels()
	currentLabels := current.GetLabels()
	for key, value := range desiredLabels {
		if currentValue, ok := currentLabels[key]; !ok || currentValue != value {
			return true
		}
	}
	return false
}

type AnnotatedObject interface {
	GetAnnotations() map[string]string
}

// annotationsNeedUpdate reports whether any of the desired annotations are missing from current or have a
// different value. Annotations on current that are not part of the desired set are ignored.
func annotationsNeedUpdate(current, desired AnnotatedObject) bool {
	desiredAnnotations := desired.GetAnnotations()
	currentAnnotations := current.GetAnnotations()
	for key, value := range desiredAnnotations {
		if currentValue, ok := currentAnnotations[key]; !ok || currentValue != value {
			return true
		}
	}
	return false
}

func buildObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels.AltinnResourceStandardLabels(),
	}
}
