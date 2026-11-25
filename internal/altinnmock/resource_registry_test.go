package altinnmock_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reading a resource", func() {
	It("finds a resource by its id, with its version and timestamps", func() {
		Expect(mock.SetResource(resource("kv-test"))).To(Succeed())

		stored := fetch("kv-test")

		Expect(stored["identifier"]).To(Equal("kv-test"))
		Expect(stored["versionId"]).To(BeNumerically(">", 0))
		Expect(stored["createdAt"]).To(MatchRegexp(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{6}\+00:00$`))
		Expect(stored["createdAt"]).To(Equal(stored["updatedAt"]))
	})

	It("answers an unknown id with a 404 ProblemDetails, sent as application/json", func() {
		answer := get(resourcePath + "/kv-nothing")

		Expect(answer.status).To(Equal(http.StatusNotFound))
		Expect(answer.mediaType()).To(Equal("application/json"))
		Expect(answer.json()).To(HaveKeyWithValue("title", "Not Found"))
	})

	It("matches the id exactly, case included", func() {
		Expect(mock.SetResource(resource("kv-case"))).To(Succeed())

		Expect(get(resourcePath + "/KV-CASE").status).To(Equal(http.StatusNotFound))
	})
})
