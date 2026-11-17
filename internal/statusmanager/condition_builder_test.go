package statusmanager_test

import (
	"github.com/kartverket/altinnator/api/v1alpha1"
	"github.com/kartverket/altinnator/internal/statusmanager"
	"github.com/kartverket/altinnator/pkg/model"
	"github.com/kartverket/altinnator/pkg/reconciliation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BuildAltinnResourceCondition", func() {
	var resource *v1alpha1.AltinnResource

	BeforeEach(func() {
		resource = newTestAltinnResource()
	})

	DescribeTable("builds the condition for the reconciliation state",
		func(state statusmanager.ReconciliationState, expectedStatus metav1.ConditionStatus, reason, message string) {
			condition := statusmanager.BuildAltinnResourceCondition(resource, state, nil)

			Expect(condition.Type).To(Equal("AltinnResource-test-resource"))
			Expect(condition.Status).To(Equal(expectedStatus))
			Expect(condition.Reason).To(Equal(reason))
			Expect(condition.Message).To(Equal(message))
			Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
		},
		Entry("pending", statusmanager.StatePending, metav1.ConditionUnknown,
			statusmanager.ReconciliationPending, "Descendants of AltinnResource are not yet reconciled."),
		Entry("failed", statusmanager.StateFailed, metav1.ConditionFalse,
			"ReconciliationFailed", "Descendants of AltinnResource failed during reconciliation."),
		Entry("ready", statusmanager.StateReady, metav1.ConditionTrue,
			"ReconciliationSuccess", "Descendants of AltinnResource reconciled successfully."),
	)

	It("preserves transition time only while condition content is unchanged", func() {
		oldTime := metav1.NewTime(metav1.Now().Add(-60))
		existing := metav1.Condition{
			Type:               "AltinnResource-test-resource",
			Status:             metav1.ConditionTrue,
			Reason:             "ReconciliationSuccess",
			Message:            "Descendants of AltinnResource reconciled successfully.",
			LastTransitionTime: oldTime,
		}

		same := statusmanager.BuildAltinnResourceCondition(resource, statusmanager.StateReady, []metav1.Condition{existing})
		Expect(same.LastTransitionTime).To(Equal(oldTime))

		changed := statusmanager.BuildAltinnResourceCondition(resource, statusmanager.StateFailed, []metav1.Condition{existing})
		Expect(changed.LastTransitionTime).NotTo(Equal(oldTime))
	})
})

var _ = Describe("BuildDescendantConditions", func() {
	It("builds success, error, and unknown conditions for descendants", func() {
		success, failure := "created", "creation failed"
		descendants := []model.Descendant[client.Object]{
			{ID: "Secret-success", SuccessMessage: &success},
			{ID: "Secret-failure", ErrorMessage: &failure},
			{ID: "ConfigMap-unknown"},
		}

		conditions := statusmanager.BuildDescendantConditions(descendants, nil)

		Expect(conditions).To(HaveLen(3))
		Expect(conditions[0].Type).To(Equal("Secret-success"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[0].Reason).To(Equal("Success"))
		Expect(conditions[0].Message).To(Equal(success))
		Expect(conditions[1].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[1].Reason).To(Equal("Error"))
		Expect(conditions[1].Message).To(Equal(failure))
		Expect(conditions[2].Status).To(Equal(metav1.ConditionUnknown))
		Expect(conditions[2].Reason).To(Equal("Unknown"))
		Expect(conditions[2].Message).To(Equal("No status message set"))
		for _, condition := range conditions {
			Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
		}
	})

	It("preserves transition time when the descendant condition is unchanged", func() {
		message := "created"
		existingTime := metav1.NewTime(metav1.Now().Add(-60))
		existing := metav1.Condition{
			Type:               "Secret-one",
			Status:             metav1.ConditionTrue,
			Reason:             "Success",
			Message:            message,
			LastTransitionTime: existingTime,
		}

		conditions := statusmanager.BuildDescendantConditions(
			[]model.Descendant[client.Object]{{ID: "Secret-one", SuccessMessage: &message}},
			[]metav1.Condition{existing},
		)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).To(Equal(existingTime))
	})
})

var _ = Describe("BuildMissingResourceConditions", func() {
	It("returns conditions only for expected resources without descendants", func() {
		descendants := []model.Descendant[client.Object]{
			{ID: "Secret-present", Object: &corev1.Secret{}},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "present", false),
			newMockResource("ConfigMap", "missing", false),
			newMockResource("Secret", "optional", true),
		}

		conditions := statusmanager.BuildMissingResourceConditions(descendants, resources, nil)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Type).To(Equal("ConfigMap-missing"))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal("NotFound"))
		Expect(conditions[0].Message).To(Equal("Expected resource missing of kind ConfigMap was not created"))
		Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
	})

	It("preserves transition time for an unchanged missing resource", func() {
		resource := newMockResource("Secret", "missing", false)
		existingTime := metav1.NewTime(metav1.Now().Add(-60))
		existing := metav1.Condition{
			Type:               "Secret-missing",
			Status:             metav1.ConditionFalse,
			Reason:             "NotFound",
			Message:            "Expected resource missing of kind Secret was not created",
			LastTransitionTime: existingTime,
		}

		conditions := statusmanager.BuildMissingResourceConditions(nil, []reconciliation.ControllerResource{resource}, []metav1.Condition{existing})

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].LastTransitionTime).To(Equal(existingTime))
	})
})

var _ = Describe("BuildConditions", func() {
	It("returns the resource condition, descendant conditions, and missing resource conditions in order", func() {
		resource := newTestAltinnResource()
		success := "created"
		descendants := []model.Descendant[client.Object]{
			{ID: "Secret-present", Object: &corev1.Secret{}, SuccessMessage: &success},
		}
		resources := []reconciliation.ControllerResource{
			newMockResource("Secret", "present", false),
			newMockResource("ConfigMap", "missing", false),
		}

		conditions := statusmanager.BuildConditions(resource, statusmanager.StateReady, descendants, resources, nil)

		Expect(conditions).To(HaveLen(3))
		Expect(conditions[0].Type).To(Equal("AltinnResource-test-resource"))
		Expect(conditions[1].Type).To(Equal("Secret-present"))
		Expect(conditions[2].Type).To(Equal("ConfigMap-missing"))
	})
})
