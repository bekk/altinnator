package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	// The lifetimes the mock in altinn-management gives its tokens
	maskinportenLifetime = 120 * time.Second
	altinnLifetime       = 30 * time.Minute
)

// fakeClock is the time the Provider and fakeAltinn share. A test moves it forward to make tokens expire.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

type cannedResponse struct {
	status int
	body   string
}

// fakeAltinn plays Maskinporten's token endpoint and Altinn's token exchange, and records the requests to them. Like
// the mock in altinn-management, its Altinn tokens are JWTs without a signature. A canned response replaces a token.
type fakeAltinn struct {
	server *httptest.Server
	clock  *fakeClock

	mu                     sync.Mutex
	grantForms             []url.Values
	exchangeAuthorizations []string
	altinnTokens           []string
	altinnLifetime         time.Duration
	maskinportenAnswer     *cannedResponse
	exchangeAnswer         *cannedResponse
	exchangeHangs          atomic.Bool
}

func newFakeAltinn(clock *fakeClock) *fakeAltinn {
	fake := &fakeAltinn{clock: clock, altinnLifetime: altinnLifetime}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", fake.serveMaskinporten)
	mux.HandleFunc("GET "+exchangePath, fake.serveExchange)
	fake.server = httptest.NewServer(mux)
	return fake
}

func (f *fakeAltinn) serveMaskinporten(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = r.ParseForm()
	f.grantForms = append(f.grantForms, r.PostForm)
	if f.maskinportenAnswer != nil {
		w.WriteHeader(f.maskinportenAnswer.status)
		_, _ = w.Write([]byte(f.maskinportenAnswer.body))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": fmt.Sprintf("maskinporten-token-%d", len(f.grantForms)),
		"token_type":   "Bearer",
		"expires_in":   int64(maskinportenLifetime / time.Second),
	})
}

func (f *fakeAltinn) serveExchange(w http.ResponseWriter, r *http.Request) {
	if f.exchangeHangs.Load() {
		<-r.Context().Done()
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exchangeAuthorizations = append(f.exchangeAuthorizations, r.Header.Get("Authorization"))
	if f.exchangeAnswer != nil {
		w.WriteHeader(f.exchangeAnswer.status)
		_, _ = w.Write([]byte(f.exchangeAnswer.body))
		return
	}
	token := unsignedJWT(fmt.Sprintf(`{"jti":"altinn-token-%d","exp":%d}`,
		len(f.exchangeAuthorizations), f.clock.Now().Add(f.altinnLifetime).Unix()))
	f.altinnTokens = append(f.altinnTokens, token)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(token))
}

func (f *fakeAltinn) grants() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.grantForms)
}

func (f *fakeAltinn) exchanges() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.exchangeAuthorizations)
}

func (f *fakeAltinn) issuedAltinnTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.altinnTokens)
}

func (f *fakeAltinn) setAltinnLifetime(lifetime time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.altinnLifetime = lifetime
}

// answer makes Maskinporten and the exchange answer with the canned responses. nil gives a token again.
func (f *fakeAltinn) answer(maskinporten, exchange *cannedResponse) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maskinportenAnswer = maskinporten
	f.exchangeAnswer = exchange
}

// unsignedJWT makes a JWT without a signature, like the mock's tokens.
func unsignedJWT(payload string) string {
	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + encode([]byte(payload)) + "."
}

// verifiedGrant checks that the test key signed the grant, and gives its kid, its claims, and its raw claims.
func verifiedGrant(assertion string) (string, grantClaims, map[string]any) {
	parsed, err := jwt.ParseSigned(assertion, []jose.SignatureAlgorithm{jose.RS256})
	Expect(err).NotTo(HaveOccurred())
	var claims grantClaims
	Expect(parsed.Claims(&privateKey.PublicKey, &claims)).To(Succeed())
	var raw map[string]any
	Expect(parsed.UnsafeClaimsWithoutVerification(&raw)).To(Succeed())
	return parsed.Headers[0].KeyID, claims, raw
}

var _ = Describe("Provider", func() {
	var (
		clock    *fakeClock
		fake     *fakeAltinn
		provider *Provider
	)

	BeforeEach(func() {
		clock = &fakeClock{now: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}
		fake = newFakeAltinn(clock)
		DeferCleanup(fake.server.Close)

		key, err := ParseKey(jwkOf(privateKey, testKeyID))
		Expect(err).NotTo(HaveOccurred())
		provider, err = newProvider(Config{
			MaskinportenTokenURL: fake.server.URL + "/token",
			PlatformBaseURL:      fake.server.URL,
			ClientID:             "test-client",
			Key:                  key,
		}, fake.server.Client(), clock.Now)
		Expect(err).NotTo(HaveOccurred())
	})

	It("refuses a Maskinporten token URL that is not absolute", func() {
		key, err := ParseKey(jwkOf(privateKey, testKeyID))
		Expect(err).NotTo(HaveOccurred())

		_, err = NewProvider(Config{MaskinportenTokenURL: "/token", Key: key}, http.DefaultClient)

		Expect(err).To(MatchError(`the Maskinporten token URL "/token" is not an absolute URL`))
	})

	Describe("getting a token", func() {
		It("trades a Maskinporten token for an Altinn token", func(ctx SpecContext) {
			token, err := provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(fake.exchanges()).To(Equal([]string{"Bearer maskinporten-token-1"}))
			Expect(token.Reveal()).To(Equal(fake.issuedAltinnTokens()[0]))
			Expect(token.expiresAt).To(BeTemporally("==", clock.Now().Add(altinnLifetime)))
		})

		It("asks Maskinporten for the resource write scope with a grant the key signs", func(ctx SpecContext) {
			_, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(fake.grants()).To(HaveLen(1))
			form := fake.grants()[0]
			Expect(form.Get("grant_type")).To(Equal("urn:ietf:params:oauth:grant-type:jwt-bearer"))
			kid, claims, raw := verifiedGrant(form.Get("assertion"))
			Expect(kid).To(Equal(testKeyID))
			Expect(claims.Issuer).To(Equal("test-client"))
			Expect(claims.Scope).To(Equal(ResourceWriteScope))
			Expect(claims.ID).NotTo(BeEmpty())
			Expect(claims.IssuedAt.Time()).To(BeTemporally("==", clock.Now()))
			Expect(claims.Expiry.Time()).To(BeTemporally("==", clock.Now().Add(60*time.Second)))
			// A string and not a list, since the mock and Maskinporten want one audience: Maskinporten's issuer
			Expect(raw).To(HaveKeyWithValue("aud", fake.server.URL+"/"))
		})
	})

	Describe("caching", func() {
		It("reuses the token until 30 seconds before it expires", func(ctx SpecContext) {
			first, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			clock.Advance(altinnLifetime - refreshLeeway - time.Second)
			again, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(again).To(Equal(first))
			Expect(fake.grants()).To(HaveLen(1))
			Expect(fake.exchanges()).To(HaveLen(1))

			clock.Advance(time.Second)
			renewed, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(renewed.Reveal()).NotTo(Equal(first.Reveal()))
			Expect(renewed.expiresAt).To(BeTemporally("==", clock.Now().Add(altinnLifetime)))
			// The Maskinporten token expired long before, so it was renewed too
			Expect(fake.exchanges()).To(Equal([]string{"Bearer maskinporten-token-1", "Bearer maskinporten-token-2"}))
		})

		It("signs a new grant for each Maskinporten token, since Maskinporten refuses a grant it has seen", func(ctx SpecContext) {
			_, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())
			clock.Advance(altinnLifetime)
			_, err = provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(fake.grants()).To(HaveLen(2))
			_, first, _ := verifiedGrant(fake.grants()[0].Get("assertion"))
			_, second, _ := verifiedGrant(fake.grants()[1].Get("assertion"))
			Expect(second.ID).NotTo(Equal(first.ID))
		})

		It("trades the cached Maskinporten token for a new Altinn token while it lasts", func(ctx SpecContext) {
			fake.setAltinnLifetime(time.Minute)
			first, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			clock.Advance(time.Minute - refreshLeeway)
			renewed, err := provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(renewed.Reveal()).NotTo(Equal(first.Reveal()))
			Expect(fake.grants()).To(HaveLen(1))
			Expect(fake.exchanges()).To(Equal([]string{"Bearer maskinporten-token-1", "Bearer maskinporten-token-1"}))
		})

		It("gets one token for callers at the same time", func(ctx SpecContext) {
			tokens := make([]Token, 10)
			var callers sync.WaitGroup
			for i := range tokens {
				callers.Go(func() {
					defer GinkgoRecover()
					var err error
					tokens[i], err = provider.Token(ctx)
					Expect(err).NotTo(HaveOccurred())
				})
			}
			callers.Wait()

			Expect(tokens).To(HaveEach(Equal(tokens[0])))
			Expect(fake.grants()).To(HaveLen(1))
			Expect(fake.exchanges()).To(HaveLen(1))
		})

		It("gives up on a fetch that hangs, so callers waiting for it get through", SpecTimeout(5*time.Second),
			func(ctx SpecContext) {
				provider.altinnTokens.timeout = 100 * time.Millisecond
				fake.exchangeHangs.Store(true)

				errs := make([]error, 2)
				var callers sync.WaitGroup
				for i := range errs {
					callers.Go(func() { _, errs[i] = provider.Token(ctx) })
				}
				callers.Wait()

				Expect(errs).To(HaveEach(MatchError(ContainSubstring("context deadline exceeded"))))
			})
	})

	Describe("when Altinn answers 401", func() {
		It("gets a new Altinn token with the cached Maskinporten token", func(ctx SpecContext) {
			refused, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			provider.Refused(refused)
			renewed, err := provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(renewed.Reveal()).NotTo(Equal(refused.Reveal()))
			Expect(fake.grants()).To(HaveLen(1))
			Expect(fake.exchanges()).To(HaveLen(2))
		})

		It("keeps a newer token when an older one is refused late", func(ctx SpecContext) {
			refused, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())
			provider.Refused(refused)
			renewed, err := provider.Token(ctx)
			Expect(err).NotTo(HaveOccurred())

			provider.Refused(refused)
			again, err := provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(again).To(Equal(renewed))
			Expect(fake.exchanges()).To(HaveLen(2))
		})
	})

	Describe("when the Altinn token exchange fails", func() {
		It("gets a new Maskinporten token for the next try", func(ctx SpecContext) {
			fake.answer(nil, &cannedResponse{http.StatusUnauthorized, `{"title":"Unauthorized","status":401}`})
			_, err := provider.Token(ctx)
			Expect(err).To(HaveOccurred())

			fake.answer(nil, nil)
			_, err = provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(fake.grants()).To(HaveLen(2))
			Expect(fake.exchanges()).To(Equal([]string{"Bearer maskinporten-token-1", "Bearer maskinporten-token-2"}))
		})
	})

	Describe("errors", func() {
		// The messages are matched in full, so they cannot quote a token
		DescribeTable("reports a failed token request to Maskinporten",
			func(ctx SpecContext, answer cannedResponse, message string) {
				fake.answer(&answer, nil)

				_, err := provider.Token(ctx)

				Expect(err).To(MatchError(message))
				Expect(fake.exchanges()).To(BeEmpty())
			},
			Entry("with Maskinporten's OAuth error",
				cannedResponse{http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid assertion"}`},
				"the token request to Maskinporten failed with HTTP 400: invalid_grant: Invalid assertion"),
			Entry("with only the status when the body is not an OAuth error",
				cannedResponse{http.StatusServiceUnavailable, `<html>secret-token</html>`},
				"the token request to Maskinporten failed with HTTP 503"),
			Entry("when the body is not JSON",
				cannedResponse{http.StatusOK, `secret-token`},
				"the response from Maskinporten is not JSON"),
			Entry("when there is no access_token",
				cannedResponse{http.StatusOK, `{"expires_in":120}`},
				"the response from Maskinporten has no access_token"),
			Entry("when the access_token cannot go in an Authorization header",
				cannedResponse{http.StatusOK, `{"access_token":"secret token","expires_in":120}`},
				"the access_token from Maskinporten is not a valid bearer token"),
			Entry("when there is no expires_in",
				cannedResponse{http.StatusOK, `{"access_token":"secret-token"}`},
				"the response from Maskinporten has no expires_in"),
		)

		DescribeTable("reports a failed Altinn token exchange",
			func(ctx SpecContext, answer cannedResponse, message string) {
				fake.answer(nil, &answer)

				_, err := provider.Token(ctx)

				Expect(err).To(MatchError(message))
			},
			Entry("with the traceId from Altinn's problem details",
				cannedResponse{http.StatusUnauthorized, `{"title":"Unauthorized","status":401,"traceId":"00-abc-01"}`},
				"the Altinn token exchange failed with HTTP 401 (traceId 00-abc-01)"),
			Entry("with the detail and traceId from Altinn's problem details",
				cannedResponse{http.StatusForbidden, `{"title":"Forbidden","status":403,"detail":"Missing scope","traceId":"00-def-01"}`},
				"the Altinn token exchange failed with HTTP 403: Missing scope (traceId 00-def-01)"),
			Entry("with only the status when the problem details have no detail or traceId",
				cannedResponse{http.StatusUnauthorized, `{"title":"Unauthorized","status":401}`},
				"the Altinn token exchange failed with HTTP 401"),
			Entry("without the body, which can be a token",
				cannedResponse{http.StatusInternalServerError, `secret-token`},
				"the Altinn token exchange failed with HTTP 500"),
			Entry("when the token is empty",
				cannedResponse{http.StatusOK, " \n"},
				"the Altinn token exchange answered with an empty token"),
			Entry("when the token cannot go in an Authorization header",
				cannedResponse{http.StatusOK, `secret token`},
				"the token from the Altinn token exchange is not a valid bearer token"),
			Entry("when the token is not a JWT",
				cannedResponse{http.StatusOK, `secret-token`},
				"the Altinn token is not a JWT"),
			Entry("when the payload is not base64url",
				cannedResponse{http.StatusOK, `a.b.c`},
				"the Altinn token's payload is not base64url"),
			Entry("when the payload is not JSON",
				cannedResponse{http.StatusOK, unsignedJWT(`secret`)},
				"the Altinn token's payload is not JSON with a numeric exp"),
			Entry("when exp is not a number",
				cannedResponse{http.StatusOK, unsignedJWT(`{"exp":"soon"}`)},
				"the Altinn token's payload is not JSON with a numeric exp"),
			Entry("when there is no exp",
				cannedResponse{http.StatusOK, unsignedJWT(`{"jti":"secret"}`)},
				"the Altinn token has no exp claim"),
		)

		It("reports that Maskinporten cannot be reached", func(ctx SpecContext) {
			fake.server.Close()

			_, err := provider.Token(ctx)

			Expect(err).To(MatchError(ContainSubstring("calling Maskinporten")))
		})

		It("does not keep a failure, so the next call tries again", func(ctx SpecContext) {
			fake.answer(&cannedResponse{http.StatusBadGateway, ""}, nil)
			_, err := provider.Token(ctx)
			Expect(err).To(HaveOccurred())

			fake.answer(nil, nil)
			token, err := provider.Token(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.Reveal()).To(Equal(fake.issuedAltinnTokens()[0]))
			Expect(fake.grants()).To(HaveLen(2))
			Expect(fake.exchanges()).To(HaveLen(1))
		})
	})
})
