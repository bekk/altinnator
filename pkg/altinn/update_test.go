package altinn_test

import (
	"context"
	"net/http"
	"strings"

	"github.com/kartverket/altinnator/pkg/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.UpdateMaskinportenSchema", func() {
	var (
		fake          *fakeAltinn
		tokenProvider *fakeTokenProvider
		client        *altinn.Client
		content       altinn.MaskinportenSchemaContent
	)

	BeforeEach(func() {
		fake = newFakeAltinn(strings.Replace(matrikkelBrukernavn, `"status": "Completed",`, `"status": "Completed",
  "delegable": true,
  "visible": true,
  "accessListMode": "Enabled",
  "isOneTimeConsent": false,
  "versionId": 7,`, 1))
		tokenProvider = &fakeTokenProvider{}
		client = altinn.NewClient(fake.server.URL, fake.server.Client(), "971040238", "kv", tokenProvider)
		content = altinn.MaskinportenSchemaContent{
			Scopes:                []string{"kartverk:matrikkel:brukernavn", "kartverk:matrikkel:les"},
			Title:                 altinn.MultilingualText{En: "Cadastre", Nb: "Matrikkel", Nn: "Matrikkel"},
			Description:           altinn.MultilingualText{En: "New", Nb: "Ny", Nn: "Ny"},
			DelegationDescription: altinn.MultilingualText{En: "Lets", Nb: "Lar", Nn: "Lar"},
			Homepage:              "https://kartverket.no",
			ContactPoints:         []altinn.ContactPoint{{Category: "Support", Email: "support@kartverket.no"}},
			Keywords:              []altinn.Keyword{{Word: "eiendom", Language: "nb"}},
		}
	})

	It("puts the new content back, with the rest as Altinn gave it", func() {
		err := client.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.writes).To(HaveLen(1))
		Expect(fake.writes[0].method).To(Equal(http.MethodPut))
		Expect(fake.writes[0].path).To(Equal("/resourceregistry/api/v1/resource/kv-matrikkel-brukernavn"))
		Expect(fake.writes[0].authorization).To(Equal("Bearer test-token"))
		Expect(fake.writes[0].contentType).To(Equal("application/json"))
		Expect(fake.writes[0].body).To(MatchJSON(`{
			"identifier": "kv-matrikkel-brukernavn",
			"title": {"en": "Cadastre", "nb": "Matrikkel", "nn": "Matrikkel"},
			"description": {"en": "New", "nb": "Ny", "nn": "Ny"},
			"rightDescription": {"en": "Lets", "nb": "Lar", "nn": "Lar"},
			"homepage": "https://kartverket.no",
			"contactPoints": [{"category": "Support", "email": "support@kartverket.no", "telephone": "", "contactPage": ""}],
			"keywords": [{"word": "eiendom", "language": "nb"}],
			"status": "Completed",
			"delegable": true,
			"visible": true,
			"accessListMode": "Enabled",
			"isOneTimeConsent": false,
			"versionId": 7,
			"resourceReferences": [
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:brukernavn", "referenceType": "MaskinportenScope"},
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:les", "referenceType": "MaskinportenScope"},
				{"referenceSource": "Default", "reference": "https://kartverket.no", "referenceType": "Uri"}
			],
			"resourceType": "MaskinportenSchema",
			"hasCompetentAuthority": {"organization": "971040238", "orgcode": "kv"}
		}`))
	})

	It("removes the homepage, contact points and keywords when they are left out", func() {
		content.Homepage, content.ContactPoints, content.Keywords = "", nil, nil

		err := client.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.writes[0].body).NotTo(ContainSubstring("homepage"))
		Expect(fake.writes[0].body).NotTo(ContainSubstring("contactPoints"))
		Expect(fake.writes[0].body).NotTo(ContainSubstring("keywords"))
	})

	It("fails with ErrNotFound without writing when Altinn has no resource with the id", func() {
		fake.resourceStatus = http.StatusNotFound

		err := client.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(altinn.ErrNotFound))
		Expect(fake.writes).To(BeEmpty())
	})

	DescribeTable("fails with ErrTaken without writing when the id is not Kartverket's MaskinportenSchema",
		func(from, to string) {
			fake.resourceBody = strings.Replace(fake.resourceBody, from, to, 1)

			err := client.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

			Expect(err).To(MatchError(altinn.ErrTaken))
			Expect(fake.writes).To(BeEmpty())
		},
		Entry("of another type", `"MaskinportenSchema"`, `"GenericAccessResource"`),
		Entry("owned by another organisation", `"971040238"`, `"991825827"`),
	)

	It("fails with ErrRejected when Altinn will not take the change", func() {
		fake.writeStatus = http.StatusBadRequest

		err := client.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(altinn.ErrRejected))
	})

	It("fails without calling Altinn when the client has no token provider", func() {
		readOnly := altinn.NewClient(fake.server.URL, fake.server.Client(), "971040238", "kv", nil)

		err := readOnly.UpdateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(ContainSubstring("needs an AccessTokenProvider")))
		Expect(fake.reads).To(BeZero())
		Expect(fake.writes).To(BeEmpty())
	})
})
