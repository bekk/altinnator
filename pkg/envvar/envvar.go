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
	// Where Altinnator reaches Altinn: the mock when run locally, or Altinn's platform in TT02 or PROD.
	AltinnPlatformBaseURL string `split_words:"true" required:"true"`
}

func (a *AdditionalConfigMapData) Decode(value string) error {
	return json.Unmarshal([]byte(value), a)
}

func Load() error {
	var loaded EnvironmentalVariables
	if err := envconfig.Process("altinnator", &loaded); err != nil {
		if parseErr, ok := errors.AsType[*envconfig.ParseError](err); ok {
			return fmt.Errorf("%s: %w", parseErr.KeyName, parseErr.Err)
		}
		return err
	}
	if strings.TrimSpace(loaded.AltinnPlatformBaseURL) == "" {
		return errors.New("ALTINNATOR_ALTINN_PLATFORM_BASE_URL must not be blank")
	}
	envVars = loaded
	return nil
}

func Get() EnvironmentalVariables {
	return envVars
}
