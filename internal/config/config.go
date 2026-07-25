package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Profile Profile `mapstructure:"profile"`
	SMTP    SMTP    `mapstructure:"smtp"`
	Letter  Letter  `mapstructure:"letter"`
}

type Profile struct {
	Name    string `mapstructure:"name"`
	Email   string `mapstructure:"email"`
	Phone   string `mapstructure:"phone"`
	Address string `mapstructure:"address"`
}

type SMTP struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	UseTLS   bool   `mapstructure:"use_tls"`
}

type Letter struct {
	Template string `mapstructure:"template"` // dpdpa | dlg | simple
}

func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".finwipe")
}

func DefaultPath() string {
	return filepath.Join(DefaultDir(), "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	// Create dir if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Defaults
	viper.SetDefault("profile.name", "")
	viper.SetDefault("profile.email", "")
	viper.SetDefault("profile.phone", "")
	viper.SetDefault("profile.address", "")
	viper.SetDefault("smtp.host", "smtp.gmail.com")
	viper.SetDefault("smtp.port", 465)
	viper.SetDefault("smtp.use_tls", true)
	viper.SetDefault("letter.template", "dpdpa")

	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	viper.SetConfigFile(path)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
