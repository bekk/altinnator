package envvar

import (
	"github.com/kelseyhightower/envconfig"
)

var envVars EnvironmentalVariables

type EnvironmentalVariables struct{}

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
