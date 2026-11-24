package envvar_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/kartverket/altinnator/pkg/envvar"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestEnvvar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envvar Suite")
}

const jwk = `{"kty":"RSA","kid":"test","d":"secret-part"}`

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
		Expect(loaded.MaskinportenClientJWK.Reveal()).To(Equal(jwk))
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

var _ = Describe("Secret", func() {
	secret := envvar.Secret(jwk)

	It("does not show its value when formatted", func() {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			Expect(fmt.Sprintf(format, secret)).NotTo(ContainSubstring("secret-part"), format)
			Expect(fmt.Sprintf(format, envvar.EnvironmentalVariables{MaskinportenClientJWK: secret})).
				NotTo(ContainSubstring("secret-part"), format)
		}
	})

	It("does not show its value in the log", func() {
		var out bytes.Buffer
		logger := zap.New(zap.WriteTo(&out))

		logger.Info("loaded", "jwk", secret, "env", envvar.EnvironmentalVariables{MaskinportenClientJWK: secret})

		Expect(out.String()).To(ContainSubstring("loaded"))
		Expect(out.String()).NotTo(ContainSubstring("secret-part"))
	})
})
