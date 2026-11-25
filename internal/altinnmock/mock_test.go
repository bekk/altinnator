package altinnmock_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Changing a resource directly in the mock", func() {
	It("adds a version that Altinn's API then returns, like a change in Altinn Studio", func() {
		Expect(mock.SetResource(resource("kv-test"))).To(Succeed())
		first, _ := mock.Resource("kv-test")
		changed := resource("kv-test")
		changed["status"] = "Active"

		Expect(mock.SetResource(changed)).To(Succeed())

		Expect(fetch("kv-test")).To(HaveKeyWithValue("status", "Active"))
		second, _ := mock.Resource("kv-test")
		Expect(second["versionId"]).To(BeNumerically(">", first["versionId"]))
		Expect(second["createdAt"]).To(Equal(first["createdAt"]))
	})

	It("refuses a resource without an identifier", func() {
		Expect(mock.SetResource(map[string]any{"title": texts("kv-test")})).To(HaveOccurred())
	})

	It("finds no resource that was never made", func() {
		_, found := mock.Resource("kv-nothing")

		Expect(found).To(BeFalse())
	})
})
