package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Helm      *HelmConfig      `yaml:"helm"`
	Kustomize *KustomizeConfig `yaml:"kustomize"`
}

type HelmConfig struct {
	Chart       string   `yaml:"chart"`
	Repo        string   `yaml:"repo"`
	Version     string   `yaml:"version"`
	ReleaseName string   `yaml:"releaseName"`
	Namespace   string   `yaml:"namespace"`
	Values      []string `yaml:"values"`
	Output      string   `yaml:"output"`
}

type KustomizeConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Kustomize == nil {
		return fmt.Errorf("kustomize section is required")
	}
	if c.Kustomize.Path == "" {
		return fmt.Errorf("kustomize.path is required")
	}

	if c.Helm != nil {
		if c.Helm.Chart == "" {
			return fmt.Errorf("helm.chart is required when helm is specified")
		}
		if c.Helm.Output == "" {
			return fmt.Errorf("helm.output is required when helm is specified")
		}
	}

	return nil
}
