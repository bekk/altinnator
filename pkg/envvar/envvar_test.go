package envvar_test

import (
	"os"
	"testing"

	"github.com/kartverket/altinnator/pkg/envvar"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnvvar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envvar Suite")
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
		setEnv("ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA", `{"extra_key": "extra_value"}`)
		setEnv("ALTINNATOR_ALTINN_PLATFORM_BASE_URL", "https://platform.tt02.altinn.no")
	})

	It("reads the additional ConfigMap data and the Altinn platform base URL", func() {
		Expect(envvar.Load()).To(Succeed())

		loaded := envvar.Get()
		Expect(loaded.AdditionalConfigMapData).To(Equal(envvar.AdditionalConfigMapData{"extra_key": "extra_value"}))
		Expect(loaded.AltinnPlatformBaseURL).To(Equal("https://platform.tt02.altinn.no"))
	})

	It("reports a value it cannot read, without quoting the value", func() {
		setEnv("ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA", `{"key": secret-part}`)

		err := envvar.Load()

		Expect(err).To(MatchError(ContainSubstring("ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA: invalid character")))
		Expect(err.Error()).NotTo(ContainSubstring("secret-part"))
	})

	DescribeTable("requires",
		func(key string) {
			unsetEnv(key)

			Expect(envvar.Load()).To(MatchError(ContainSubstring(key)))
		},
		Entry(nil, "ALTINNATOR_ADDITIONAL_CONFIG_MAP_DATA"),
		Entry(nil, "ALTINNATOR_ALTINN_PLATFORM_BASE_URL"),
	)

	It("refuses a blank Altinn platform base URL", func() {
		setEnv("ALTINNATOR_ALTINN_PLATFORM_BASE_URL", "  ")

		Expect(envvar.Load()).To(MatchError("ALTINNATOR_ALTINN_PLATFORM_BASE_URL must not be blank"))
	})
})
