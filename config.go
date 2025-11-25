package easywallet

import (
	"log"
	"os"
)
import "gopkg.in/yaml.v3"

type ProviderInfo struct {
	Currency       string `yaml:"currency"`
	ProviderType   string `yaml:"provider_type"`
	ServiceUrl     string `yaml:"service_url"`
	TokenAddress   string `yaml:"token_address"`
	DerivationPath string `yaml:"derivation_path"`
}
type Config struct {
	Network   string         `yaml:"network"`
	MasterKey string         `yaml:"master_key"`
	Providers []ProviderInfo `yaml:"providers"`
}

func ReadConfig(yamlFilePath string) (*Config, error) {

	yamlFile, err := os.ReadFile(yamlFilePath)
	if err != nil {
		log.Printf("Error reading YAML file: %v\n", err)
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Printf("Error unmarshaling YAML: %v\n", err)
		return nil, err
	}

	return &config, nil
}
