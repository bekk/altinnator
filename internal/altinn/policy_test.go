package altinn_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/kartverket/altinnator/internal/altinn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeTokens struct {
	refused []string
}

func (f *fakeTokens) AccessToken(context.Context) (string, error) {
	return "test-token", nil
}

func (f *fakeTokens) Refused(token string) {
	f.refused = append(f.refused, token)
}

type policyRequest struct {
	method        string
	path          string
	authorization string
	contentType   string
	formName      string
	fileName      string
	fileType      string
	policy        string
}

var _ = Describe("Client.SetMaskinportenSchemaPolicy", func() {
	var (
		resourceStatus int
		policyStatus   int
		writes         []policyRequest
		tokens         *fakeTokens
		server         *httptest.Server
		client         *altinn.Client
	)

	BeforeEach(func() {
		resourceStatus = http.StatusOK
		policyStatus = http.StatusCreated
		writes = nil
		tokens = &fakeTokens{}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.WriteHeader(resourceStatus)
				_, _ = w.Write([]byte(matrikkelBrukernavn))
				return
			}
			write := policyRequest{
				method:        r.Method,
				path:          r.URL.Path,
				authorization: r.Header.Get("Authorization"),
				contentType:   r.Header.Get("Content-Type"),
			}
			if form, err := r.MultipartReader(); err == nil {
				if part, err := form.NextPart(); err == nil {
					policy, _ := io.ReadAll(part)
					write.formName, write.fileName = part.FormName(), part.FileName()
					write.fileType, write.policy = part.Header.Get("Content-Type"), string(policy)
				}
			}
			writes = append(writes, write)
			w.WriteHeader(policyStatus)
		}))
		DeferCleanup(server.Close)
		client = altinn.NewClient(server.URL, server.Client(), tokens)
	})

	It("puts the policy as the form file policyFile, with a token", func() {
		err := client.SetMaskinportenSchemaPolicy(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(writes).To(HaveLen(1))
		Expect(writes[0].method).To(Equal(http.MethodPut))
		Expect(writes[0].path).To(Equal("/resourceregistry/api/v1/resource/kv-matrikkel-brukernavn/policy"))
		Expect(writes[0].authorization).To(Equal("Bearer test-token"))
		Expect(writes[0].contentType).To(HavePrefix("multipart/form-data; boundary="))
		Expect(writes[0].formName).To(Equal("policyFile"))
		Expect(writes[0].fileName).To(Equal("policy.xml"))
		Expect(writes[0].fileType).To(Equal("application/xml"))
	})

	It("sends the same policy as altinn-management", func() {
		err := client.SetMaskinportenSchemaPolicy(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).NotTo(HaveOccurred())
		Expect(writes).To(HaveLen(1))
		Expect(writes[0].policy).To(Equal(matrikkelBrukernavnPolicy))
	})

	It("fails with ErrNotFound without writing when Kartverket has no MaskinportenSchema with the id", func() {
		resourceStatus = http.StatusNotFound

		err := client.SetMaskinportenSchemaPolicy(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(altinn.ErrNotFound))
		Expect(writes).To(BeEmpty())
	})

	It("tells the token provider when Altinn refuses the token", func() {
		policyStatus = http.StatusUnauthorized

		err := client.SetMaskinportenSchemaPolicy(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(ContainSubstring("responded 401")))
		Expect(tokens.refused).To(Equal([]string{"test-token"}))
	})

	It("fails without calling Altinn when the client has no token provider", func() {
		readOnly := altinn.NewClient(server.URL, server.Client(), nil)

		err := readOnly.SetMaskinportenSchemaPolicy(context.Background(), "kv-matrikkel-brukernavn")

		Expect(err).To(MatchError(ContainSubstring("needs an AccessTokenProvider")))
		Expect(writes).To(BeEmpty())
	})
})

// What maskinportenSchemaPolicy in altinn-management's MaskinportenSchemaPolicy.kt gives for kv-matrikkel-brukernavn
const matrikkelBrukernavnPolicy = `<?xml version="1.0" encoding="utf-8"?>
<xacml:Policy xmlns:xacml="urn:oasis:names:tc:xacml:3.0:core:schema:wd-17" PolicyId="urn:maskinportenschema:kv-matrikkel-brukernavn:1" Version="1.0" RuleCombiningAlgId="urn:oasis:names:tc:xacml:3.0:rule-combining-algorithm:deny-overrides">
  <xacml:Target />
  <xacml:Rule RuleId="urn:maskinportenschema:kv-matrikkel-brukernavn:1:1" Effect="Permit">
    <xacml:Description>MaskinportenSchema resource policy for; kv-matrikkel-brukernavn for access package; maskinporten-scopes to have access to actions; ScopeAccess</xacml:Description>
    <xacml:Target>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:3.0:function:string-equal-ignore-case">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">maskinporten-scopes</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:altinn:accesspackage" Category="urn:oasis:names:tc:xacml:1.0:subject-category:access-subject" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:1.0:function:string-equal">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">kv-matrikkel-brukernavn</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:altinn:resource" Category="urn:oasis:names:tc:xacml:3.0:attribute-category:resource" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:3.0:function:string-equal-ignore-case">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">ScopeAccess</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:oasis:names:tc:xacml:1.0:action:action-id" Category="urn:oasis:names:tc:xacml:3.0:attribute-category:action" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
    </xacml:Target>
  </xacml:Rule>
  <xacml:ObligationExpressions>
    <xacml:ObligationExpression FulfillOn="Permit" ObligationId="urn:maskinportenschema:kv-matrikkel-brukernavn:obligation:1">
      <xacml:AttributeAssignmentExpression AttributeId="urn:maskinportenschema:kv-matrikkel-brukernavn:obligation-assignment:1" Category="urn:altinn:minimum-authenticationlevel">
        <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#integer">3</xacml:AttributeValue>
      </xacml:AttributeAssignmentExpression>
    </xacml:ObligationExpression>
  </xacml:ObligationExpressions>
</xacml:Policy>`
