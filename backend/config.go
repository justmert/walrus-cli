package backend

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the Walrus backend configuration
type Config struct {
	Walrus WalrusConfig `yaml:"walrus"`
}

// WalrusConfig contains Walrus-specific settings
type WalrusConfig struct {
	AggregatorURL string       `yaml:"aggregator_url"`
	PublisherURL  string       `yaml:"publisher_url"`
	Epochs        int          `yaml:"epochs"`
	Wallet        WalletConfig `yaml:"wallet"`
}

// WalletConfig contains wallet settings
type WalletConfig struct {
	PrivateKey string `yaml:"private_key"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Walrus: WalrusConfig{
			AggregatorURL: "https://aggregator.walrus-testnet.walrus.space",
			PublisherURL:  "https://publisher.walrus-testnet.walrus.space",
			Epochs:        5,
			Wallet: WalletConfig{
				PrivateKey: "",
			},
		},
	}
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	// If no path provided, try default locations
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultPaths := []string{
				filepath.Join(home, ".config", "walrus-rclone", "config.yaml"),
				filepath.Join(home, ".walrus-rclone", "config.yaml"),
				"walrus-config.yaml",
			}

			for _, p := range defaultPaths {
				if _, err := os.Stat(p); err == nil {
					path = p
					break
				}
			}
		}
	}

	// If still no path, return default config
	if path == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Set defaults for missing values
	if config.Walrus.AggregatorURL == "" {
		config.Walrus.AggregatorURL = "https://aggregator.walrus-testnet.walrus.space"
	}
	if config.Walrus.PublisherURL == "" {
		config.Walrus.PublisherURL = "https://publisher.walrus-testnet.walrus.space"
	}
	if config.Walrus.Epochs == 0 {
		config.Walrus.Epochs = 5
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Walrus.AggregatorURL == "" {
		return fmt.Errorf("aggregator_url is required")
	}
	if c.Walrus.PublisherURL == "" {
		return fmt.Errorf("publisher_url is required")
	}
	if c.Walrus.Epochs <= 0 {
		return fmt.Errorf("epochs must be positive")
	}
	return nil
}