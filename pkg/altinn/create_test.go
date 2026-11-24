package altinn_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/kartverket/altinnator/pkg/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.CreateMaskinportenSchema", func() {
	var (
		status        int
		answer        string
		requests      []writeRequest
		tokenProvider *fakeTokenProvider
		server        *httptest.Server
		client        *altinn.Client
		content       altinn.MaskinportenSchemaContent
	)

	BeforeEach(func() {
		status = http.StatusCreated
		answer = ""
		requests = nil
		tokenProvider = &fakeTokenProvider{}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			requests = append(requests, writeRequest{
				method:        r.Method,
				path:          r.URL.Path,
				authorization: r.Header.Get("Authorization"),
				contentType:   r.Header.Get("Content-Type"),
				body:          string(body),
			})
			w.WriteHeader(status)
			_, _ = w.Write([]byte(answer))
		}))
		DeferCleanup(server.Close)
		client = altinn.NewClient(server.URL, server.Client(), "971040238", "kv", tokenProvider)
		content = altinn.MaskinportenSchemaContent{
			Scopes:                []string{"kartverk:matrikkel:brukernavn"},
			Title:                 altinn.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
			Description:           altinn.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
			DelegationDescription: altinn.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
			Homepage:              "https://kartverket.no/matrikkel",
			ContactPoints:         []altinn.ContactPoint{{Category: "Kundesenter", Email: "post@kartverket.no"}},
			Keywords:              []altinn.Keyword{{Word: "matrikkel", Language: "nb"}},
		}
	})

	It("posts the resource, owned by Kartverket, with a token", func() {
		err := client.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).NotTo(HaveOccurred())
		Expect(requests).To(HaveLen(1))
		Expect(requests[0].method).To(Equal(http.MethodPost))
		Expect(requests[0].path).To(Equal("/resourceregistry/api/v1/resource"))
		Expect(requests[0].authorization).To(Equal("Bearer test-token"))
		Expect(requests[0].contentType).To(Equal("application/json"))
		Expect(requests[0].body).To(MatchJSON(`{
			"identifier": "kv-matrikkel-brukernavn",
			"title": {"en": "Cadastre API", "nb": "Matrikkel API", "nn": "Matrikkel API"},
			"description": {"en": "The Cadastre", "nb": "Matrikkelen", "nn": "Matrikkelen"},
			"rightDescription": {"en": "Allows", "nb": "Gir", "nn": "Gir"},
			"homepage": "https://kartverket.no/matrikkel",
			"contactPoints": [{"category": "Kundesenter", "email": "post@kartverket.no", "telephone": "", "contactPage": ""}],
			"keywords": [{"word": "matrikkel", "language": "nb"}],
			"resourceReferences": [
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:brukernavn", "referenceType": "MaskinportenScope"}
			],
			"delegable": true,
			"visible": true,
			"resourceType": "MaskinportenSchema",
			"hasCompetentAuthority": {"organization": "971040238", "orgcode": "kv"},
			"accessListMode": "Disabled"
		}`))
	})

	It("leaves out the homepage, contact points and keywords when they are empty", func() {
		content.Homepage, content.ContactPoints, content.Keywords = "", nil, nil

		err := client.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).NotTo(HaveOccurred())
		Expect(requests[0].body).NotTo(ContainSubstring("homepage"))
		Expect(requests[0].body).NotTo(ContainSubstring("contactPoints"))
		Expect(requests[0].body).NotTo(ContainSubstring("keywords"))
	})

	It("fails with ErrTaken when Altinn has a resource with the id", func() {
		status = http.StatusConflict

		err := client.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(altinn.ErrTaken))
		Expect(tokenProvider.refused).To(BeEmpty())
	})

	DescribeTable("fails with ErrRejected and Altinn's reasons when Altinn will not take the resource",
		func(rejectedStatus int, reasons string) {
			status, answer = rejectedStatus, reasons

			err := client.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

			Expect(err).To(MatchError(altinn.ErrRejected))
			Expect(err).To(MatchError(ContainSubstring(reasons)))
			Expect(tokenProvider.refused).To(BeEmpty())
		},
		Entry("with 400", http.StatusBadRequest, `{"errors":{"Title":["Missing title in nn"]}}`),
		Entry("with 401 and InvalidPrefix, which is about the scopes and not the token",
			http.StatusUnauthorized, `{"errors":{"InvalidPrefix":["digdir:example"]}}`),
	)

	It("tells the token provider when Altinn refuses the token", func() {
		status = http.StatusUnauthorized

		err := client.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(ContainSubstring("responded 401")))
		Expect(err).NotTo(MatchError(altinn.ErrRejected))
		Expect(tokenProvider.refused).To(Equal([]string{"test-token"}))
	})

	DescribeTable("fails without calling Altinn",
		func(id, message string) {
			err := client.CreateMaskinportenSchema(context.Background(), id, content)

			Expect(err).To(MatchError(ContainSubstring(message)))
			Expect(requests).To(BeEmpty())
		},
		Entry("when the id is one of Altinn's routes", "search", `"search" is one of Altinn's routes`),
		Entry("when the id breaks Altinn's rule", "KV.API", `but was "KV.API"`),
	)

	It("fails without calling Altinn when the client has no token provider", func() {
		readOnly := altinn.NewClient(server.URL, server.Client(), "971040238", "kv", nil)

		err := readOnly.CreateMaskinportenSchema(context.Background(), "kv-matrikkel-brukernavn", content)

		Expect(err).To(MatchError(ContainSubstring("needs an AccessTokenProvider")))
		Expect(requests).To(BeEmpty())
	})
})
