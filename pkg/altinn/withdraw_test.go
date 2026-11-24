package altinn_test

import (
	"context"
	"net/http"
	"strings"

	"github.com/kartverket/altinnator/pkg/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.WithdrawMaskinportenSchema", func() {
	var (
		fake          *fakeAltinn
		tokenProvider *fakeTokenProvider
		client        *altinn.Client
	)

	BeforeEach(func() {
		fake = newFakeAltinn(strings.Replace(matrikkelBrukernavn, `"status": "Completed",`, `"status": "Completed",
  "delegable": true,
  "visible": true,
  "isOneTimeConsent": false,
  "versionId": 7,
  "createdAt": "2026-11-01T10:00:00+00:00",
  "updatedAt": "2026-11-02T10:00:00+00:00",`, 1))
		tokenProvider = &fakeTokenProvider{}
		client = altinn.NewClient(fake.server.URL, fake.server.Client(), "971040238", "kv", tokenProvider)
	})

	It("puts the resource back withdrawn, with the rest as Altinn gave it", func() {
		err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.writes).To(HaveLen(1))
		Expect(fake.writes[0].method).To(Equal(http.MethodPut))
		Expect(fake.writes[0].path).To(Equal("/resourceregistry/api/v1/resource/kv-matrikkel-brukernavn"))
		Expect(fake.writes[0].authorization).To(Equal("Bearer test-token"))
		Expect(fake.writes[0].contentType).To(Equal("application/json"))
		Expect(fake.writes[0].body).To(MatchJSON(`{
			"identifier": "kv-matrikkel-brukernavn",
			"title": {"en": "Cadastre API", "nb": "Matrikkel API", "nn": "Matrikkel API"},
			"description": {"en": "The Cadastre", "nb": "Matrikkelen", "nn": "Matrikkelen"},
			"rightDescription": {"en": "Allows", "nb": "Gir", "nn": "Gir"},
			"status": "Withdrawn",
			"delegable": false,
			"visible": false,
			"isOneTimeConsent": false,
			"versionId": 7,
			"createdAt": "2026-11-01T10:00:00+00:00",
			"updatedAt": "2026-11-02T10:00:00+00:00",
			"resourceReferences": [
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:brukernavn", "referenceType": "MaskinportenScope"},
				{"referenceSource": "Default", "reference": "https://kartverket.no", "referenceType": "Uri"}
			],
			"resourceType": "MaskinportenSchema",
			"hasCompetentAuthority": {"organization": "971040238", "orgcode": "kv"}
		}`))
	})

	It("fails with ErrNotFound without writing when Altinn has no resource with the id", func() {
		fake.resourceStatus = http.StatusNotFound

		err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(altinn.ErrNotFound))
		Expect(fake.writes).To(BeEmpty())
	})

	It("does nothing when the resource is already withdrawn", func() {
		fake.resourceBody = strings.NewReplacer(
			`"status": "Completed"`, `"status": "Withdrawn"`,
			`"delegable": true`, `"delegable": false`,
			`"visible": true`, `"visible": false`,
		).Replace(fake.resourceBody)

		err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.writes).To(BeEmpty())
	})

	It("withdraws a resource that is withdrawn but still visible", func() {
		fake.resourceBody = strings.Replace(fake.resourceBody, `"status": "Completed"`, `"status": "Withdrawn"`, 1)

		err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.writes).To(HaveLen(1))
	})

	DescribeTable("fails with ErrTaken without writing when the id is not Kartverket's MaskinportenSchema",
		func(from, to string) {
			fake.resourceBody = strings.Replace(fake.resourceBody, from, to, 1)

			err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

			Expect(err).To(MatchError(altinn.ErrTaken))
			Expect(fake.writes).To(BeEmpty())
		},
		Entry("of another type", `"MaskinportenSchema"`, `"GenericAccessResource"`),
		Entry("owned by another organisation", `"971040238"`, `"991825827"`),
	)

	It("fails with what Altinn answered when the write fails", func() {
		fake.writeStatus = http.StatusInternalServerError

		err := client.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(ContainSubstring("responded 500")))
	})

	It("fails without calling Altinn when the client has no token provider", func() {
		readOnly := altinn.NewClient(fake.server.URL, fake.server.Client(), "971040238", "kv", nil)

		err := readOnly.WithdrawMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(ContainSubstring("needs an AccessTokenProvider")))
		Expect(fake.reads).To(BeZero())
		Expect(fake.writes).To(BeEmpty())
	})
})
