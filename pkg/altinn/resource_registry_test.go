package altinn_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/kartverket/altinnator/pkg/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const matrikkelBrukernavn = `{
  "identifier": "kv-matrikkel-brukernavn",
  "title": {"en": "Cadastre API", "nb": "Matrikkel API", "nn": "Matrikkel API"},
  "description": {"en": "The Cadastre", "nb": "Matrikkelen", "nn": "Matrikkelen"},
  "rightDescription": {"en": "Allows", "nb": "Gir", "nn": "Gir"},
  "status": "Completed",
  "resourceReferences": [
    {"referenceSource": "Default", "reference": "kartverk:matrikkel:brukernavn", "referenceType": "MaskinportenScope"},
    {"referenceSource": "Default", "reference": "https://kartverket.no", "referenceType": "Uri"}
  ],
  "resourceType": "MaskinportenSchema",
  "hasCompetentAuthority": {"organization": "971040238", "orgcode": "kv"}
}`

type response struct {
	status int
	body   string
}

var _ = Describe("Client.GetMaskinportenSchema", func() {
	var (
		responses map[string]response
		requests  []*http.Request
		client    *altinn.Client
	)

	BeforeEach(func() {
		responses = map[string]response{}
		requests = nil
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r)
			res, ok := responses[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(res.status)
			_, _ = w.Write([]byte(res.body))
		}))
		DeferCleanup(server.Close)
		client = altinn.NewClient(server.URL, server.Client(), "971040238", nil)
	})

	It("returns Kartverket's MaskinportenSchema", func() {
		responses["/resourceregistry/api/v1/resource/kv-matrikkel-brukernavn"] = response{http.StatusOK, matrikkelBrukernavn}

		schema, err := client.GetMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(schema).To(Equal(&altinn.MaskinportenSchema{
			ResourceID:            "kv-matrikkel-brukernavn",
			Scopes:                []string{"kartverk:matrikkel:brukernavn"},
			Title:                 altinn.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
			Description:           altinn.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
			DelegationDescription: altinn.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
			Status:                "Completed",
		}))
		Expect(requests).To(HaveLen(1))
		Expect(requests[0].Method).To(Equal(http.MethodGet))
		Expect(requests[0].Header.Get("Accept")).To(Equal("application/json"))
	})

	It("fails with ErrNotFound when Altinn has no resource with the id", func() {
		_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

		Expect(err).To(MatchError(altinn.ErrNotFound))
	})

	DescribeTable("fails with ErrTaken when another resource has the id",
		func(body, message string) {
			responses["/resourceregistry/api/v1/resource/some-resource"] = response{http.StatusOK, body}

			_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

			Expect(err).To(MatchError(altinn.ErrTaken))
			Expect(err).NotTo(MatchError(altinn.ErrNotFound))
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("when the resource is another organisation's",
			`{"resourceType": "MaskinportenSchema", "hasCompetentAuthority": {"organization": "974760223"}}`,
			"resource some-resource is a MaskinportenSchema owned by 974760223"),
		Entry("when the resource is of another type",
			`{"resourceType": "GenericAccessResource", "hasCompetentAuthority": {"organization": "971040238"}}`,
			"resource some-resource is a GenericAccessResource owned by 971040238"),
	)

	It("fails without calling Altinn when the id is one of Altinn's routes", func() {
		_, err := client.GetMaskinportenSchema(context.Background(), "search")

		Expect(err).To(MatchError(ContainSubstring(`"search" is one of Altinn's routes`)))
		Expect(requests).To(BeEmpty())
	})

	It("fails without calling Altinn when the id breaks Altinn's rule", func() {
		_, err := client.GetMaskinportenSchema(context.Background(), "../export")

		Expect(err).To(MatchError(ContainSubstring(`but was "../export"`)))
		Expect(requests).To(BeEmpty())
	})

	It("fails when Altinn responds with an error", func() {
		responses["/resourceregistry/api/v1/resource/some-resource"] = response{http.StatusServiceUnavailable, ""}

		_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

		Expect(err).To(MatchError(HaveSuffix("responded 503")))
	})

	It("fails with what Altinn answered when it responds with an error", func() {
		responses["/resourceregistry/api/v1/resource/some-resource"] = response{
			http.StatusForbidden, `{"title": "Forbidden"}`,
		}

		_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

		Expect(err).To(MatchError(HaveSuffix(`responded 403: {"title": "Forbidden"}`)))
	})

	It("fails with only the start of a long answer from Altinn", func() {
		responses["/resourceregistry/api/v1/resource/some-resource"] = response{
			http.StatusBadGateway, strings.Repeat("x", 2000),
		}

		_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

		Expect(err).To(MatchError(HaveSuffix("responded 502: " + strings.Repeat("x", 1024))))
	})

	It("fails when Altinn responds with something other than a resource", func() {
		responses["/resourceregistry/api/v1/resource/some-resource"] = response{http.StatusOK, "<html>"}

		_, err := client.GetMaskinportenSchema(context.Background(), "some-resource")

		Expect(err).To(MatchError(ContainSubstring("parsing resource some-resource")))
	})
})
