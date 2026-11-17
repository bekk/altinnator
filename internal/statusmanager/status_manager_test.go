package statusmanager_test

import (
	"context"

	"github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/internal/statusmanager"
	"github.com/kartverket/altinnator/pkg/model"
	"github.com/kartverket/altinnator/pkg/reconciliation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestAltinnResource() *v1alpha1.AltinnResource {
	return &v1alpha1.AltinnResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AltinnResource",
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-resource",
			Namespace:  "default",
			Generation: 1,
		},
	}
}

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

var _ = Describe("DetermineReconciliationState", func() {
	It("returns failed when any descendant has an error", func() {
		message := "creation failed"
		scope := &model.Scope{
			Descendants: []model.Descendant[client.Object]{
				{ID: "Secret-one", ErrorMessage: &message},
			},
		}

		state, err := statusmanager.DetermineReconciliationState(scope, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(*state).To(Equal(statusmanager.StateFailed))
	})

	It("returns pending when expected resources are missing", func() {
		scope := &model.Scope{}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "one", false),
		}

		state, err := statusmanager.DetermineReconciliationState(scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*state).To(Equal(statusmanager.StatePending))
	})

	It("ignores nil resources when determining whether reconciliation is complete", func() {
		scope := &model.Scope{}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "optional", true),
		}

		state, err := statusmanager.DetermineReconciliationState(scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*state).To(Equal(statusmanager.StateReady))
	})

	It("returns ready when every non-nil resource has a descendant", func() {
		scope := &model.Scope{
			Descendants: []model.Descendant[client.Object]{
				{ID: "Secret-one", Object: &corev1.Secret{}},
			},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "one", false),
		}

		state, err := statusmanager.DetermineReconciliationState(scope, resources)

		Expect(err).NotTo(HaveOccurred())
		Expect(*state).To(Equal(statusmanager.StateReady))
	})
})

var _ = Describe("UpdateAltinnResourceStatus", func() {
	var (
		ctx          context.Context
		resource     *v1alpha1.AltinnResource
		original     *v1alpha1.AltinnResource
		k8sClient    client.Client
		fakeRecorder *events.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		resource = newTestAltinnResource()
		original = resource.DeepCopy()
		fakeRecorder = events.NewFakeRecorder(10)
		k8sClient = fake.NewClientBuilder().
			WithScheme(newTestScheme()).
			WithObjects(resource).
			WithStatusSubresource(resource).
			Build()
	})

	It("sets ready status and emits success when reconciliation is complete", func() {
		success := "created successfully"
		scope := &model.Scope{
			AltinnResource: *resource,
			Descendants: []model.Descendant[client.Object]{
				{ID: "Secret-one", Object: &corev1.Secret{}, SuccessMessage: &success},
			},
		}

		statusmanager.UpdateAltinnResourceStatus(ctx, k8sClient, fakeRecorder, scope, original, []reconciliation.ControllerResource{
			newMockResource("Secret", "one", false),
		})

		updated := &v1alpha1.AltinnResource{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha1.PhaseReady))
		Expect(updated.Status.Ready).To(BeTrue())
		Expect(updated.Status.ObservedGeneration).To(Equal(resource.Generation))
		Expect(updated.Status.Message).To(Equal("AltinnResource ready."))
		Expect(updated.Status.Conditions).To(HaveLen(2))

		close(fakeRecorder.Events)
		var eventMessages []string
		for event := range fakeRecorder.Events {
			eventMessages = append(eventMessages, event)
		}
		Expect(eventMessages).To(HaveLen(2))
		Expect(eventMessages[0]).To(ContainSubstring("StatusUpdateStarted"))
		Expect(eventMessages[1]).To(ContainSubstring("StatusUpdateSuccess"))
	})

	It("sets failed status when a descendant has an error", func() {
		failure := "creation failed"
		scope := &model.Scope{
			AltinnResource: *resource,
			Descendants: []model.Descendant[client.Object]{
				{ID: "Secret-one", ErrorMessage: &failure},
			},
		}

		statusmanager.UpdateAltinnResourceStatus(ctx, k8sClient, fakeRecorder, scope, original, []reconciliation.ControllerResource{
			newMockResource("Secret", "one", false),
		})

		updated := &v1alpha1.AltinnResource{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha1.PhaseFailed))
		Expect(updated.Status.Ready).To(BeFalse())
		Expect(updated.Status.Message).To(Equal("AltinnResource failed."))
	})

	It("does not write or emit success when status is unchanged", func() {
		resource.Status.Phase = v1alpha1.PhaseReady
		resource.Status.Ready = true
		resource.Status.Message = "AltinnResource ready."
		resource.Status.ObservedGeneration = resource.Generation
		existingCondition := metav1.Condition{
			Type:               "AltinnResource-test-resource",
			Status:             metav1.ConditionTrue,
			Reason:             "ReconciliationSuccess",
			Message:            "Descendants of AltinnResource reconciled successfully.",
			LastTransitionTime: metav1.Now(),
		}
		resource.Status.Conditions = []metav1.Condition{existingCondition}
		original = resource.DeepCopy()
		k8sClient = fake.NewClientBuilder().
			WithScheme(newTestScheme()).
			WithObjects(resource).
			WithStatusSubresource(resource).
			Build()
		scope := &model.Scope{AltinnResource: *resource}

		statusmanager.UpdateAltinnResourceStatus(ctx, k8sClient, fakeRecorder, scope, original, nil)

		close(fakeRecorder.Events)
		var eventMessages []string
		for event := range fakeRecorder.Events {
			eventMessages = append(eventMessages, event)
		}
		Expect(eventMessages).To(HaveLen(1))
		Expect(eventMessages[0]).To(ContainSubstring("StatusUpdateStarted"))
	})
})
