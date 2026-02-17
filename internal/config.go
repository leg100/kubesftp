package internal

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	HostKeysSecret     string      `split_words:"true" required:"true"`
	HostKeysAlgorithms []algorithm `split_words:"true" required:"true"`
	Namespace          string      `envconfig:"pod_namespace" required:"true"`
	Users              map[string]userInfo
}

type userInfo struct {
	AuthorizedKeys []string
	AllowedHosts   []string
}

func newDefaultConfig() config {
	return config{
		HostKeysAlgorithms: []algorithm{
			ed25519Algorithm,
			ecdsaAlgorithm,
			rsaAlgorithm,
		},
	}
}

func loadConfig() (config, error) {
	var cfg config
	if err := envconfig.Process("", &cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func loadRequiredEnv(envVars map[string]string, name string) (string, error) {
	v, ok := envVars[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}
