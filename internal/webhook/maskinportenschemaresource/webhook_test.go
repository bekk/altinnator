package maskinportenschemaresource_test

import (
	altinnatorv1alpha1 "github.com/kartverket/altinnator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newNamespace() string {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "webhook-test-"}}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	DeferCleanup(func() { Expect(k8sClient.Delete(ctx, namespace)).To(Succeed()) })
	return namespace.Name
}

func newResource(namespace, name, resourceID string) *altinnatorv1alpha1.MaskinportenSchemaResource {
	text := altinnatorv1alpha1.MultilingualText{En: "Test", Nb: "Test", Nn: "Test"}
	return &altinnatorv1alpha1.MaskinportenSchemaResource{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: altinnatorv1alpha1.MaskinportenSchemaResourceSpec{
			Scopes:                []string{"kartverk:test"},
			DelegationDescription: text,
			Title:                 text,
			Description:           text,
			ResourceID:            resourceID,
		},
	}
}

func create(resource client.Object) error {
	err := k8sClient.Create(ctx, resource)
	if err == nil {
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, resource)).To(Succeed()) })
	}
	return err
}

var _ = Describe("MaskinportenSchemaResource webhook", func() {
	It("refuses a resource whose resourceId another resource uses, also in another namespace", func() {
		first, second := newNamespace(), newNamespace()
		Expect(create(newResource(first, "kv-api", "kv-taken-api"))).To(Succeed())

		err := create(newResource(second, "kv-api", "kv-taken-api"))

		Expect(err).To(MatchError(ContainSubstring(
			"resourceId kv-taken-api is already used by MaskinportenSchemaResource " + first + "/kv-api")))
	})

	It("accepts a resource whose resourceId no other resource uses", func() {
		namespace := newNamespace()
		Expect(create(newResource(namespace, "kv-api", "kv-free-api"))).To(Succeed())

		Expect(create(newResource(namespace, "kv-other-api", "kv-other-api"))).To(Succeed())
	})

	It("lets a resource be changed", func() {
		resource := newResource(newNamespace(), "kv-api", "kv-changed-api")
		Expect(create(resource)).To(Succeed())

		resource.Spec.Title.En = "Changed"
		Expect(k8sClient.Update(ctx, resource)).To(Succeed())
	})
})
