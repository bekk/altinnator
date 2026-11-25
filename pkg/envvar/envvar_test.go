package envvar_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/kartverket/altinnator/pkg/envvar"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestEnvvar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envvar Suite")
}

var (
	privateKey *rsa.PrivateKey
	// jwk is privateKey as a JWK, and privatePart is its private exponent, which must never be shown
	jwk         string
	privatePart string
)

var _ = BeforeSuite(func() {
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	jwk = jwkOf("test-kid")

	var fields map[string]string
	Expect(json.Unmarshal([]byte(jwk), &fields)).To(Succeed())
	privatePart = fields["d"]
	Expect(privatePart).NotTo(BeEmpty())
})

func jwkOf(keyID string) string {
	encoded, err := jose.JSONWebKey{Key: privateKey, KeyID: keyID}.MarshalJSON()
	Expect(err).NotTo(HaveOccurred())
	return string(encoded)
}

// setEnv sets an environment variable for one spec, and restores it afterwards.
func setEnv(key, value string) {
	old, had := os.LookupEnv(key)
	Expect(os.Setenv(key, value)).To(Succeed())
	DeferCleanup(func() {
		if had {
			Expect(os.Setenv(key, old)).To(Succeed())
		} else {
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	})
}

func unsetEnv(key string) {
	setEnv(key, "")
	Expect(os.Unsetenv(key)).To(Succeed())
}

var _ = Describe("Load", func() {
	BeforeEach(func() {
		setEnv("ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA", `{}`)
		setEnv("ALTINNATOR_ALTINN_ENVIRONMENT", "TT02")
		setEnv("ALTINNATOR_MASKINPORTEN_CLIENT_ID", "client")
		setEnv("ALTINNATOR_MASKINPORTEN_CLIENT_JWK", jwk)
		unsetEnv("ALTINNATOR_LOCAL_MOCK_URL")
	})

	It("reads the Altinn environment and the Maskinporten client", func() {
		Expect(envvar.Load()).To(Succeed())

		loaded := envvar.Get()
		Expect(loaded.AltinnEnvironment).To(Equal(envvar.AltinnEnvironmentTT02))
		Expect(loaded.MaskinportenClientID).To(Equal("client"))
		Expect(loaded.MaskinportenClientJWK.String()).To(Equal("Key(kid=test-kid)"))
	})

	It("refuses a bad Maskinporten key, without showing the key", func() {
		setEnv("ALTINNATOR_MASKINPORTEN_CLIENT_JWK", jwkOf(""))

		Expect(envvar.Load()).To(MatchError(
			"ALTINNATOR_MASKINPORTEN_CLIENT_JWK: the Maskinporten key must have a kid"))
	})

	DescribeTable("picks Altinn and Maskinporten from the environment",
		func(environment, localMockURL, platformBaseURL, maskinportenTokenURL string) {
			setEnv("ALTINNATOR_ALTINN_ENVIRONMENT", environment)
			if localMockURL != "" {
				setEnv("ALTINNATOR_LOCAL_MOCK_URL", localMockURL)
			}
			Expect(envvar.Load()).To(Succeed())

			loaded := envvar.Get()
			Expect(loaded.PlatformBaseURL()).To(Equal(platformBaseURL))
			Expect(loaded.MaskinportenTokenURL()).To(Equal(maskinportenTokenURL))
		},
		Entry("LOCAL", "LOCAL", "", "http://localhost:8090", "http://localhost:8090/token"),
		Entry("LOCAL, with the mock somewhere else", "LOCAL", "http://host.docker.internal:8090/",
			"http://host.docker.internal:8090/", "http://host.docker.internal:8090/token"),
		Entry("TT02", "TT02", "", "https://platform.tt02.altinn.no", "https://test.maskinporten.no/token"),
		Entry("TT02, even with a mock URL", "TT02", "http://host.docker.internal:8090",
			"https://platform.tt02.altinn.no", "https://test.maskinporten.no/token"),
		Entry("PROD", "PROD", "", "https://platform.altinn.no", "https://maskinporten.no/token"),
	)

	It("refuses an unknown Altinn environment", func() {
		setEnv("ALTINNATOR_ALTINN_ENVIRONMENT", "tt02")

		Expect(envvar.Load()).To(MatchError(ContainSubstring("must be one of LOCAL, TT02 or PROD")))
	})

	DescribeTable("requires",
		func(key string) {
			unsetEnv(key)

			Expect(envvar.Load()).To(MatchError(ContainSubstring(key)))
		},
		Entry(nil, "ALTINNATOR_ALTINN_ENVIRONMENT"),
		Entry(nil, "ALTINNATOR_MASKINPORTEN_CLIENT_ID"),
		Entry(nil, "ALTINNATOR_MASKINPORTEN_CLIENT_JWK"),
	)

	DescribeTable("refuses blank",
		func(key string) {
			setEnv(key, "  ")

			Expect(envvar.Load()).To(MatchError(ContainSubstring(key + " must not be blank")))
		},
		Entry(nil, "ALTINNATOR_MASKINPORTEN_CLIENT_ID"),
	)
})

var _ = Describe("EnvironmentalVariables", func() {
	var loaded envvar.EnvironmentalVariables

	BeforeEach(func() {
		setEnv("ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA", `{}`)
		setEnv("ALTINNATOR_ALTINN_ENVIRONMENT", "TT02")
		setEnv("ALTINNATOR_MASKINPORTEN_CLIENT_ID", "client")
		setEnv("ALTINNATOR_MASKINPORTEN_CLIENT_JWK", jwk)
		Expect(envvar.Load()).To(Succeed())
		loaded = envvar.Get()
	})

	It("do not show the Maskinporten key when formatted", func() {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			for _, value := range []any{loaded, loaded.MaskinportenClientJWK} {
				Expect(fmt.Sprintf(format, value)).NotTo(ContainSubstring(privatePart), format)
			}
		}
	})

	It("do not show the Maskinporten key in the log", func() {
		var out bytes.Buffer
		logger := zap.New(zap.WriteTo(&out))

		logger.Info("loaded", "jwk", loaded.MaskinportenClientJWK, "env", loaded)

		Expect(out.String()).To(ContainSubstring("Key(kid=test-kid)"))
		Expect(out.String()).NotTo(ContainSubstring(privatePart))
	})
})
