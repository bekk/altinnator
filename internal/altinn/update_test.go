package altinn_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/kartverket/altinnator/internal/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.UpdateMaskinportenSchema", func() {
	var (
		resourceStatus int
		resourceBody   string
		updateStatus   int
		updateAnswer   string
		reads          int
		writes         []createRequest
		tokenProvider  *fakeTokenProvider
		server         *httptest.Server
		client         *altinn.Client
		schema         altinn.NewMaskinportenSchema
	)

	BeforeEach(func() {
		resourceStatus = http.StatusOK
		resourceBody = matrikkelBrukernavn
		updateStatus = http.StatusOK
		updateAnswer = ""
		reads, writes = 0, nil
		tokenProvider = &fakeTokenProvider{}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				reads++
				w.WriteHeader(resourceStatus)
				_, _ = w.Write([]byte(resourceBody))
				return
			}
			body, _ := io.ReadAll(r.Body)
			writes = append(writes, createRequest{
				method:        r.Method,
				path:          r.URL.Path,
				authorization: r.Header.Get("Authorization"),
				contentType:   r.Header.Get("Content-Type"),
				body:          string(body),
			})
			w.WriteHeader(updateStatus)
			_, _ = w.Write([]byte(updateAnswer))
		}))
		DeferCleanup(server.Close)
		client = altinn.NewClient(server.URL, server.Client(), tokenProvider)
		schema = altinn.NewMaskinportenSchema{
			ResourceID:            "kv-matrikkel-brukernavn",
			Scopes:                []string{"kartverk:matrikkel:brukernavn", "kartverk:matrikkel:adresse"},
			Title:                 altinn.MultilingualText{En: "Cadastre API", Nb: "Matrikkel API", Nn: "Matrikkel API"},
			Description:           altinn.MultilingualText{En: "The Cadastre", Nb: "Matrikkelen", Nn: "Matrikkelen"},
			DelegationDescription: altinn.MultilingualText{En: "Allows", Nb: "Gir", Nn: "Gir"},
		}
	})

	It("puts the whole resource, owned by Kartverket, with a token", func() {
		err := client.UpdateMaskinportenSchema(context.Background(), schema)

		Expect(err).NotTo(HaveOccurred())
		Expect(writes).To(HaveLen(1))
		Expect(writes[0].method).To(Equal(http.MethodPut))
		Expect(writes[0].path).To(Equal("/resourceregistry/api/v1/resource/kv-matrikkel-brukernavn"))
		Expect(writes[0].authorization).To(Equal("Bearer test-token"))
		Expect(writes[0].contentType).To(Equal("application/json"))
		Expect(writes[0].body).To(MatchJSON(`{
			"identifier": "kv-matrikkel-brukernavn",
			"title": {"en": "Cadastre API", "nb": "Matrikkel API", "nn": "Matrikkel API"},
			"description": {"en": "The Cadastre", "nb": "Matrikkelen", "nn": "Matrikkelen"},
			"rightDescription": {"en": "Allows", "nb": "Gir", "nn": "Gir"},
			"resourceReferences": [
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:brukernavn", "referenceType": "MaskinportenScope"},
				{"referenceSource": "Default", "reference": "kartverk:matrikkel:adresse", "referenceType": "MaskinportenScope"}
			],
			"delegable": true,
			"visible": true,
			"resourceType": "MaskinportenSchema",
			"hasCompetentAuthority": {"organization": "971040238", "orgcode": "kv"},
			"accessListMode": "Disabled"
		}`))
	})

	It("fails with ErrNotFound without writing when Altinn has no resource with the id", func() {
		resourceStatus = http.StatusNotFound

		err := client.UpdateMaskinportenSchema(context.Background(), schema)

		Expect(err).To(MatchError(altinn.ErrNotFound))
		Expect(writes).To(BeEmpty())
	})

	DescribeTable("fails with ErrTaken without writing when another resource has the id",
		func(replace, with string) {
			resourceBody = strings.Replace(matrikkelBrukernavn, replace, with, 1)

			err := client.UpdateMaskinportenSchema(context.Background(), schema)

			Expect(err).To(MatchError(altinn.ErrTaken))
			Expect(writes).To(BeEmpty())
		},
		Entry("when the resource is another organisation's", `"971040238"`, `"974760223"`),
		Entry("when the resource is of another type", `"MaskinportenSchema"`, `"GenericAccessResource"`),
	)

	It("fails with ErrRejected and Altinn's reasons when Altinn will not take the resource", func() {
		updateStatus, updateAnswer = http.StatusBadRequest, `{"errors":{"Title":["Missing title in nn"]}}`

		err := client.UpdateMaskinportenSchema(context.Background(), schema)

		Expect(err).To(MatchError(altinn.ErrRejected))
		Expect(err).To(MatchError(ContainSubstring("Missing title in nn")))
	})

	It("tells the token provider when Altinn refuses the token", func() {
		updateStatus = http.StatusUnauthorized

		err := client.UpdateMaskinportenSchema(context.Background(), schema)

		Expect(err).To(MatchError(ContainSubstring("responded 401")))
		Expect(tokenProvider.refused).To(Equal([]string{"test-token"}))
	})

	It("fails without calling Altinn when the client has no token provider", func() {
		readOnly := altinn.NewClient(server.URL, server.Client(), nil)

		err := readOnly.UpdateMaskinportenSchema(context.Background(), schema)

		Expect(err).To(MatchError(ContainSubstring("needs an AccessTokenProvider")))
		Expect(reads).To(BeZero())
		Expect(writes).To(BeEmpty())
	})
})
