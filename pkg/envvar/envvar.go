package envvar

import (
	"encoding/json"

	"github.com/kelseyhightower/envconfig"
)

var envVars EnvironmentalVariables

type AdditionalConfigMapData map[string]string

type EnvironmentalVariables struct {
	AdditionalConfigMapData AdditionalConfigMapData `split_words:"true" required:"true"`
}

func (a *AdditionalConfigMapData) Decode(value string) error {
	return json.Unmarshal([]byte(value), a)
}

func Load() error {
	var loaded EnvironmentalVariables
	if err := envconfig.Process("altinnator", &loaded); err != nil {
		return err
	}
	envVars = loaded
	return nil
}

func Get() EnvironmentalVariables {
	return envVars
}
