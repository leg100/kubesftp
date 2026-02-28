package internal

import (
	"os"

	"github.com/goccy/go-yaml"
	"github.com/kelseyhightower/envconfig"
)

type config struct {
	HostKeysSecret     string      `split_words:"true"`
	HostKeysAlgorithms []algorithm `split_words:"true"`
	Namespace          string      `envconfig:"pod_namespace"`
	Users              []user
	ConfigFilePath     string `split_words:"true" required:"true"`
	ChrootsDir         string `split_words:"true" default:"/chroots"`
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

func LoadConfig() (config, error) {
	var cfg config
	if err := envconfig.Process("", &cfg); err != nil {
		return config{}, err
	}
	configFile, err := os.ReadFile(cfg.ConfigFilePath)
	if err != nil {
		return config{}, err
	}
	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		return config{}, err
	}

	return cfg, nil
}
