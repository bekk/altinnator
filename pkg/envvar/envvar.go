package envvar

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kartverket/altinnator/pkg/altinn/auth"
	"github.com/kelseyhightower/envconfig"
)

var envVars EnvironmentalVariables

type AdditionalConfigMapData map[string]string

type EnvironmentalVariables struct {
	AdditionalConfigMapData AdditionalConfigMapData `split_words:"true" required:"true"`
	AltinnEnvironment       AltinnEnvironment       `split_words:"true" required:"true"`
	MaskinportenClientID    string                  `split_words:"true" required:"true"`
	MaskinportenClientJWK   MaskinportenKey         `split_words:"true" required:"true"`
	// Where LOCAL finds the mock. A pod in kind cannot reach it on localhost.
	LocalMockURL string `split_words:"true" default:"http://localhost:8090"`
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
	AltinnEnvironmentTT02: {"https://platform.tt02.altinn.no", "https://test.maskinporten.no/token"},
	AltinnEnvironmentProd: {"https://platform.altinn.no", "https://maskinporten.no/token"},
}

func (e *AltinnEnvironment) Decode(value string) error {
	switch environment := AltinnEnvironment(value); environment {
	case AltinnEnvironmentLocal, AltinnEnvironmentTT02, AltinnEnvironmentProd:
		*e = environment
		return nil
	}
	return fmt.Errorf("must be one of %s, %s or %s",
		AltinnEnvironmentLocal, AltinnEnvironmentTT02, AltinnEnvironmentProd)
}

func (e EnvironmentalVariables) PlatformBaseURL() string {
	if e.AltinnEnvironment == AltinnEnvironmentLocal {
		return e.LocalMockURL
	}
	return endpoints[e.AltinnEnvironment].platformBaseURL
}

func (e EnvironmentalVariables) MaskinportenTokenURL() string {
	if e.AltinnEnvironment == AltinnEnvironmentLocal {
		return strings.TrimRight(e.LocalMockURL, "/") + "/token"
	}
	return endpoints[e.AltinnEnvironment].maskinportenTokenURL
}

type MaskinportenKey struct {
	auth.Key
}

func (k *MaskinportenKey) Decode(value string) error {
	key, err := auth.ParseKey(value)
	if err != nil {
		return err
	}
	k.Key = key
	return nil
}

func Load() error {
	var loaded EnvironmentalVariables
	if err := envconfig.Process("altinnator", &loaded); err != nil {
		if parseErr, ok := errors.AsType[*envconfig.ParseError](err); ok {
			return fmt.Errorf("%s: %w", parseErr.KeyName, parseErr.Err)
		}
		return err
	}
	if strings.TrimSpace(loaded.MaskinportenClientID) == "" {
		return errors.New("ALTINNATOR_MASKINPORTEN_CLIENT_ID must not be blank")
	}
	envVars = loaded
	return nil
}

func Get() EnvironmentalVariables {
	return envVars
}
