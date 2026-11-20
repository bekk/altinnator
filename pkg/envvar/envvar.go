package envvar

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

var envVars EnvironmentalVariables

type AdditionalConfigMapData map[string]string

type EnvironmentalVariables struct {
	AdditionalConfigMapData AdditionalConfigMapData `split_words:"true" required:"true"`
	AltinnEnvironment       AltinnEnvironment       `split_words:"true" required:"true"`
	MaskinportenClientID    string                  `split_words:"true" required:"true"`
	MaskinportenClientJWK   Secret                  `split_words:"true" required:"true"`
}

func (a *AdditionalConfigMapData) Decode(value string) error {
	return json.Unmarshal([]byte(value), a)
}

type AltinnEnvironment string

const (
	AltinnEnvironmentLocal AltinnEnvironment = "LOCAL"
	AltinnEnvironmentTT02  AltinnEnvironment = "TT02"
	AltinnEnvironmentProd  AltinnEnvironment = "PROD"
)

type altinnEndpoints struct {
	platformBaseURL      string
	maskinportenTokenURL string
}

var endpoints = map[AltinnEnvironment]altinnEndpoints{
	AltinnEnvironmentLocal: {"http://localhost:8090", "http://localhost:8090/token"},
	AltinnEnvironmentTT02:  {"https://platform.tt02.altinn.no", "https://test.maskinporten.no/token"},
	AltinnEnvironmentProd:  {"https://platform.altinn.no", "https://maskinporten.no/token"},
}

func (e *AltinnEnvironment) Decode(value string) error {
	if _, ok := endpoints[AltinnEnvironment(value)]; !ok {
		return fmt.Errorf("must be one of %s, %s or %s",
			AltinnEnvironmentLocal, AltinnEnvironmentTT02, AltinnEnvironmentProd)
	}
	*e = AltinnEnvironment(value)
	return nil
}

func (e AltinnEnvironment) PlatformBaseURL() string {
	return endpoints[e].platformBaseURL
}

func (e AltinnEnvironment) MaskinportenTokenURL() string {
	return endpoints[e].maskinportenTokenURL
}

type Secret string

func (Secret) String() string   { return "***" }
func (Secret) GoString() string { return "***" }
func (Secret) MarshalLog() any  { return "***" }

func (s Secret) Reveal() string { return string(s) }

func Load() error {
	var loaded EnvironmentalVariables
	if err := envconfig.Process("altinnator", &loaded); err != nil {
		return err
	}
	if strings.TrimSpace(loaded.MaskinportenClientID) == "" {
		return errors.New("ALTINNATOR_MASKINPORTEN_CLIENT_ID must not be blank")
	}
	if strings.TrimSpace(loaded.MaskinportenClientJWK.Reveal()) == "" {
		return errors.New("ALTINNATOR_MASKINPORTEN_CLIENT_JWK must not be blank")
	}
	envVars = loaded
	return nil
}

func Get() EnvironmentalVariables {
	return envVars
}
