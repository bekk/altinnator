package statusmanager_test

import (
	"context"
	"sync/atomic"

	"github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/internal/statusmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("UpdateStatus", func() {
	It("writes the new status through the status subresource", func() {
		ctx := context.Background()
		resource := newTestAltinnResource()
		resource.Status.Phase = v1alpha1.PhasePending
		k8sClient := fake.NewClientBuilder().
			WithScheme(newTestScheme()).
			WithObjects(resource).
			WithStatusSubresource(resource).
			Build()

		resource.Status.Phase = v1alpha1.PhaseReady
		resource.Status.Ready = true
		resource.Status.Message = "Ready"

		Expect(statusmanager.UpdateStatus(ctx, k8sClient, *resource)).To(Succeed())

		updated := &v1alpha1.AltinnResource{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha1.PhaseReady))
		Expect(updated.Status.Ready).To(BeTrue())
		Expect(updated.Status.Message).To(Equal("Ready"))
	})

	It("returns NotFound when the resource does not exist", func() {
		resource := newTestAltinnResource()
		k8sClient := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

		err := statusmanager.UpdateStatus(context.Background(), k8sClient, *resource)

		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("retries a conflict and then succeeds", func() {
		resource := newTestAltinnResource()
		var updateCalls atomic.Int32
		k8sClient := fake.NewClientBuilder().
			WithScheme(newTestScheme()).
			WithObjects(resource).
			WithStatusSubresource(resource).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(
					ctx context.Context,
					c client.Client,
					subResourceName string,
					obj client.Object,
					opts ...client.SubResourceUpdateOption,
				) error {
					if updateCalls.Add(1) == 1 {
						return apierrors.NewConflict(
							schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "altinnresources"},
							obj.GetName(),
							nil,
						)
					}
					return c.SubResource(subResourceName).Update(ctx, obj, opts...)
				},
			}).
			Build()
		resource.Status.Phase = v1alpha1.PhaseReady

		Expect(statusmanager.UpdateStatus(context.Background(), k8sClient, *resource)).To(Succeed())
		Expect(updateCalls.Load()).To(Equal(int32(2)))

		updated := &v1alpha1.AltinnResource{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(resource), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha1.PhaseReady))
	})
})
