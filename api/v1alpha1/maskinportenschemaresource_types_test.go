package v1alpha1_test

import (
	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const namespace = "default"

// validSpec returns the spec of a MaskinportenSchemaResource that Altinn accepts.
// Every call returns a new map, so a test can change it.
func validSpec() map[string]any {
	return map[string]any{
		"scopes": []any{"kartverk:validation:test"},
		"delegationDescription": map[string]any{
			"en": "Gives access to nothing.",
			"nb": "Gir tilgang til ingenting.",
			"nn": "Gjev tilgang til ingenting.",
		},
		"title": map[string]any{
			"en": "Validation test",
			"nb": "Valideringstest",
			"nn": "Valideringstest",
		},
		"description": map[string]any{
			"en": "A resource for testing validation.",
			"nb": "En ressurs for å teste validering.",
			"nn": "Ein ressurs for å teste validering.",
		},
		"resourceId": "kv-validation-test",
		"homepage":   "https://example.no/validation-test",
		"contactPoints": []any{
			map[string]any{"email": "test@example.no", "category": "Test e-post"},
		},
		"keywords": []any{
			map[string]any{"word": "test", "language": "en"},
		},
	}
}

// newResource is unstructured, so a test can send a spec the Go types cannot express, such as one without resourceId.
func newResource(spec map[string]any) *unstructured.Unstructured {
	resource := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	resource.SetGroupVersionKind(altinnatorv1alpha1.GroupVersion.WithKind("MaskinportenSchemaResource"))
	resource.SetName("validation-test")
	resource.SetNamespace(namespace)
	return resource
}

func expectInvalid(err error, message string) {
	GinkgoHelper()
	Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an Invalid error, got: %v", err)
	Expect(err).To(MatchError(ContainSubstring(message)))
}

var _ = Describe("MaskinportenSchemaResource CRD", func() {
	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &altinnatorv1alpha1.MaskinportenSchemaResource{}, client.InNamespace(namespace))).
			To(Succeed())
	})

	It("accepts a valid resource and keeps every field", func() {
		resource := newResource(validSpec())
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(resource.GroupVersionKind())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(resource), stored)).To(Succeed())
		Expect(stored.Object["spec"]).To(Equal(validSpec()))
	})

	DescribeTable("refuses a resource Altinn would refuse",
		func(change func(spec map[string]any), message string) {
			spec := validSpec()
			change(spec)
			expectInvalid(k8sClient.Create(ctx, newResource(spec)), message)
		},
		Entry("a title that is only whitespace, which Altinn counts as missing",
			func(spec map[string]any) { spec["title"].(map[string]any)["nn"] = " " },
			"spec.title.nn in body should match"),
		Entry("a blank description",
			func(spec map[string]any) { spec["description"].(map[string]any)["en"] = " " },
			"spec.description.en in body should match"),
		Entry("a blank delegationDescription",
			func(spec map[string]any) { spec["delegationDescription"].(map[string]any)["nb"] = " " },
			"spec.delegationDescription.nb in body should match"),
		Entry("no scopes",
			func(spec map[string]any) { spec["scopes"] = []any{} },
			"spec.scopes in body should have at least 1 items"),
		Entry("duplicate scopes",
			func(spec map[string]any) {
				spec["scopes"] = []any{"kartverk:validation:test", "kartverk:validation:test"}
			},
			"spec.scopes[1]: Duplicate value"),
		Entry("a scope with whitespace",
			func(spec map[string]any) { spec["scopes"] = []any{"kartverk:validation test"} },
			"spec.scopes[0] in body should match"),
		Entry("a resourceId with characters Altinn does not allow",
			func(spec map[string]any) { spec["resourceId"] = "KV.API" },
			"spec.resourceId in body should match"),
		Entry("a resourceId with the app_ prefix Altinn keeps for Altinn Studio apps",
			func(spec map[string]any) { spec["resourceId"] = "app_kv-api" },
			"resourceId cannot start with app_ or se_"),
		Entry("a resourceId with the se_ prefix Altinn keeps for Altinn 2 services",
			func(spec map[string]any) { spec["resourceId"] = "se_kv-api" },
			"resourceId cannot start with app_ or se_"),
	)

	Context("when the resource exists", func() {
		var resource *unstructured.Unstructured

		BeforeEach(func() {
			resource = newResource(validSpec())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		It("refuses a change of resourceId", func() {
			Expect(unstructured.SetNestedField(resource.Object, "kv-another-id", "spec", "resourceId")).To(Succeed())
			expectInvalid(k8sClient.Update(ctx, resource), "resourceId cannot be changed")
		})

		It("refuses removing resourceId", func() {
			unstructured.RemoveNestedField(resource.Object, "spec", "resourceId")
			expectInvalid(k8sClient.Update(ctx, resource), "spec.resourceId: Required value")
		})
	})
})
