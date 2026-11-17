package statusmanager_test

import (
	"context"

	"github.com/kartverket/altinnator/pkg/reconciliation"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockControllerResource struct {
	resourceKind string
	resourceName string
	isNil        bool
}

func (m *mockControllerResource) Reconcile(
	context.Context,
	client.Client,
	*runtime.Scheme,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (m *mockControllerResource) GetResourceKind() string { return m.resourceKind }
func (m *mockControllerResource) GetResourceName() string { return m.resourceName }
func (m *mockControllerResource) IsResourceNil() bool     { return m.isNil }

var _ reconciliation.ControllerResource = (*mockControllerResource)(nil)

func newMockResource(kind, name string, isNil bool) reconciliation.ControllerResource {
	return &mockControllerResource{
		resourceKind: kind,
		resourceName: name,
		isNil:        isNil,
	}
}
